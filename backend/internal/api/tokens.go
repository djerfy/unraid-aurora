package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	TokenRoleViewer   = "viewer"
	TokenRoleOperator = "operator"
	TokenRoleAdmin    = "admin"
)

type APIToken struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Role      string     `json:"role"`
	ReadOnly  bool       `json:"readOnly"`
	Created   time.Time  `json:"created"`
	LastUsed  time.Time  `json:"lastUsed"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	Scopes    []string   `json:"scopes,omitempty"`
	Hash      string     `json:"hash,omitempty"`
}

func normalizeTokenRole(role string, readOnly bool) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case TokenRoleViewer, TokenRoleOperator, TokenRoleAdmin:
		return strings.ToLower(strings.TrimSpace(role))
	}
	if readOnly {
		return TokenRoleViewer
	}
	return TokenRoleAdmin
}

type tokenStore struct {
	mu          sync.Mutex
	path        string
	tokens      []APIToken
	lastPersist time.Time
}

func newTokenStore(dir string) *tokenStore {
	store := &tokenStore{}
	if dir == "" {
		return store
	}
	store.path = filepath.Join(dir, "api-tokens.json")
	raw, err := os.ReadFile(store.path)
	if err == nil {
		_ = json.Unmarshal(raw, &store.tokens)
	}
	for i := range store.tokens {
		store.tokens[i].Role = normalizeTokenRole(store.tokens[i].Role, store.tokens[i].ReadOnly)
		store.tokens[i].ReadOnly = store.tokens[i].Role == TokenRoleViewer
	}
	return store
}

func (s *tokenStore) list() []APIToken {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]APIToken, len(s.tokens))
	copy(out, s.tokens)
	for i := range out {
		out[i].Hash = ""
	}
	return out
}

var tokenScopes = map[string]bool{
	"docker": true, "vms": true, "storage": true, "network": true,
	"system": true, "ups": true, "apps": true, "automation": true, "access": true,
}

func normalizeTokenScopes(scopes []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, scope := range scopes {
		scope = strings.ToLower(strings.TrimSpace(scope))
		if scope == "" || seen[scope] {
			continue
		}
		if !tokenScopes[scope] {
			return nil, fmt.Errorf("unknown token scope %q", scope)
		}
		seen[scope] = true
		out = append(out, scope)
	}
	return out, nil
}

func tokenScopeForPath(path string) string {
	switch {
	case strings.HasPrefix(path, "/api/v1/docker"):
		return "docker"
	case strings.HasPrefix(path, "/api/v1/vms"):
		return "vms"
	case strings.HasPrefix(path, "/api/v1/array/"), strings.HasPrefix(path, "/api/v1/disks/"),
		strings.HasPrefix(path, "/api/v1/shares"), strings.HasPrefix(path, "/api/v1/pools"),
		strings.HasPrefix(path, "/api/v1/parity/"), strings.HasPrefix(path, "/api/v1/mover"),
		strings.HasPrefix(path, "/api/v1/balance/"), strings.HasPrefix(path, "/api/v1/zfs/"):
		return "storage"
	case strings.HasPrefix(path, "/api/v1/network/"):
		return "network"
	case strings.HasPrefix(path, "/api/v1/ups/"):
		return "ups"
	case strings.HasPrefix(path, "/api/v1/apps/"), strings.HasPrefix(path, "/api/v1/plugins"):
		return "apps"
	case strings.HasPrefix(path, "/api/v1/user-scripts/"):
		return "automation"
	case strings.HasPrefix(path, "/api/v1/users"), strings.HasPrefix(path, "/api/v1/tokens"),
		strings.HasPrefix(path, "/api/v1/audit"), strings.HasPrefix(path, "/api/v1/notifications"):
		return "access"
	case strings.HasPrefix(path, "/api/v1/system/"), path == "/api/v1/settings",
		path == "/api/v1/logs", path == "/api/v1/terminal", path == "/api/v1/actions",
		strings.HasPrefix(path, "/api/v1/config"), strings.HasPrefix(path, "/api/v1/tools/"),
		strings.HasPrefix(path, "/api/v1/storage-layout"), strings.HasPrefix(path, "/api/v1/unraid-api"),
		path == "/api/v1/bootstrap":
		return "system"
	}
	return ""
}

func tokenScopeAllows(scopes []string, path string) bool {
	if len(scopes) == 0 {
		return true
	}
	required := tokenScopeForPath(path)
	if required == "" {
		return true
	}
	for _, scope := range scopes {
		if scope == required {
			return true
		}
	}
	return false
}

func (s *tokenStore) create(name, role string, expiresAt *time.Time, scopes []string) (APIToken, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 {
		return APIToken{}, "", errors.New("token name is required")
	}
	role = normalizeTokenRole(role, false)
	scopes, err := normalizeTokenScopes(scopes)
	if err != nil {
		return APIToken{}, "", err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return APIToken{}, "", err
	}
	raw := base64.RawURLEncoding.EncodeToString(secret)
	hash := sha256.Sum256([]byte(raw))
	idBytes := make([]byte, 8)
	_, _ = rand.Read(idBytes)
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.tokens) >= 50 {
		return APIToken{}, "", errors.New("too many API tokens")
	}
	for _, token := range s.tokens {
		if strings.EqualFold(token.Name, name) {
			return APIToken{}, "", errors.New("a token with this name already exists")
		}
	}
	token := APIToken{
		ID: hex.EncodeToString(idBytes), Name: name, Role: role, ReadOnly: role == TokenRoleViewer,
		Created: time.Now(), ExpiresAt: expiresAt, Scopes: scopes, Hash: hex.EncodeToString(hash[:]),
	}
	s.tokens = append(s.tokens, token)
	if err := s.save(); err != nil {
		return APIToken{}, "", err
	}
	token.Hash = ""
	return token, raw, nil
}

func (s *tokenStore) revoke(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, token := range s.tokens {
		if token.ID == id {
			s.tokens = append(s.tokens[:i], s.tokens[i+1:]...)
			return s.save()
		}
	}
	return errors.New("token not found")
}

func (s *tokenStore) validate(raw string) (APIToken, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return APIToken{}, false
	}
	hash := sha256.Sum256([]byte(raw))
	encoded := hex.EncodeToString(hash[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tokens {
		if subtle.ConstantTimeCompare([]byte(encoded), []byte(s.tokens[i].Hash)) == 1 {
			if s.tokens[i].ExpiresAt != nil && time.Now().After(*s.tokens[i].ExpiresAt) {
				return APIToken{}, false
			}
			s.tokens[i].LastUsed = time.Now()
			if time.Since(s.lastPersist) > 30*time.Second {
				s.lastPersist = time.Now()
				_ = s.save()
			}
			return s.tokens[i], true
		}
	}
	return APIToken{}, false
}

func (s *tokenStore) save() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.tokens, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
