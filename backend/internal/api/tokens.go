package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Role     string    `json:"role"`
	ReadOnly bool      `json:"readOnly"`
	Created  time.Time `json:"created"`
	LastUsed time.Time `json:"lastUsed"`
	Hash     string    `json:"hash,omitempty"`
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

func (s *tokenStore) create(name, role string) (APIToken, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 {
		return APIToken{}, "", errors.New("token name is required")
	}
	role = normalizeTokenRole(role, false)
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
		Created: time.Now(), Hash: hex.EncodeToString(hash[:]),
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
