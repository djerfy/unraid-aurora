// Package auth authenticates the local root account against the host shadow
// database, matching how the Unraid webGUI validates credentials. Sessions are
// stateless HMAC-signed cookies; passwords are never stored or logged.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GehirnInc/crypt"
	_ "github.com/GehirnInc/crypt/md5_crypt"
	_ "github.com/GehirnInc/crypt/sha256_crypt"
	_ "github.com/GehirnInc/crypt/sha512_crypt"
)

const (
	CookieName = "aurora_session"
	// Unraid permits three failed attempts per address before a 15 minute
	// cooldown. Aurora mirrors that policy.
	maxFailures = 3
	failureTTL  = 15 * time.Minute
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrRateLimited        = errors.New("too many attempts, try again later")
	ErrUnsupportedHash    = errors.New("unsupported shadow hash format")
)

type Session struct {
	User  string `json:"u"`
	Exp   int64  `json:"exp"`
	Nonce string `json:"n"`
	Epoch int64  `json:"e"`
}

// SessionInfo is the safe, listable view of an active session.
type SessionInfo struct {
	Nonce    string    `json:"nonce"`
	User     string    `json:"user"`
	IP       string    `json:"ip"`
	Created  time.Time `json:"created"`
	LastSeen time.Time `json:"lastSeen"`
	Expires  time.Time `json:"expires"`
	Current  bool      `json:"current"`
}

type sessionRecord struct {
	User     string
	IP       string
	Created  time.Time
	LastSeen time.Time
	Expires  time.Time
}

type failureBucket struct {
	count int
	until time.Time
}

type shadowLookupFunc func(username string) (string, error)
type passwordVerifyFunc func(password, hash string) error

type Manager struct {
	secret []byte
	ttl    time.Duration

	lookup shadowLookupFunc
	verify passwordVerifyFunc

	mu          sync.Mutex
	failures    map[string]*failureBucket
	epoch       int64
	sessions    map[string]sessionRecord
	persistDir  string
	lastPersist time.Time
}

// Option customises the shadow backend. The defaults read the host through
// getent and verify standard crypt(3) hashes.
type Option func(*Manager)

// WithShadowLookup replaces the host shadow reader (used by tests and
// embedded deployments with a different credential store).
func WithShadowLookup(fn func(username string) (string, error)) Option {
	return func(m *Manager) {
		if fn != nil {
			m.lookup = fn
		}
	}
}

// WithPasswordVerifier replaces the crypt(3) verifier (used by tests).
func WithPasswordVerifier(fn func(password, hash string) error) Option {
	return func(m *Manager) {
		if fn != nil {
			m.verify = fn
		}
	}
}

// WithPersistence loads the HMAC secret and session registry from dir so
// sessions survive an Aurora restart. The directory must be private (0750).
func WithPersistence(dir string) Option {
	return func(m *Manager) {
		if strings.TrimSpace(dir) == "" {
			return
		}
		m.persistDir = dir
		m.loadSecret()
		m.loadSessions()
	}
}

func New(ttl time.Duration, opts ...Option) *Manager {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		panic("auth: cannot read random secret: " + err.Error())
	}
	m := &Manager{
		secret:   secret,
		ttl:      ttl,
		lookup:   lookupShadow,
		verify:   verifyShadowHash,
		failures: map[string]*failureBucket{},
		epoch:    time.Now().UnixNano(),
		sessions: map[string]sessionRecord{},
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Login validates the root account against the host. Usernames other than root
// are rejected exactly like the original webGUI.
func (m *Manager) Login(username, password, ip string) (string, Session, error) {
	if !m.allow(ip) {
		return "", Session{}, ErrRateLimited
	}
	if strings.TrimSpace(username) != username || username != "root" || password == "" {
		m.recordFailure(ip)
		return "", Session{}, ErrInvalidCredentials
	}
	hash, err := m.lookup(username)
	if err != nil || !isUsableHash(hash) {
		m.recordFailure(ip)
		return "", Session{}, ErrInvalidCredentials
	}
	if err := m.verify(password, hash); err != nil {
		m.recordFailure(ip)
		return "", Session{}, ErrInvalidCredentials
	}
	m.clearFailures(ip)

	session := Session{
		User:  username,
		Exp:   time.Now().Add(m.ttl).Unix(),
		Nonce: randomToken(12),
		Epoch: m.currentEpoch(),
	}
	token, err := m.sign(session)
	if err != nil {
		return "", Session{}, err
	}
	m.mu.Lock()
	m.sessions[session.Nonce] = sessionRecord{
		User: session.User, IP: ip, Created: time.Now(),
		LastSeen: time.Now(), Expires: time.Unix(session.Exp, 0),
	}
	m.saveSessionsLocked()
	m.mu.Unlock()
	return token, session, nil
}

func (m *Manager) Verify(token string) (Session, bool) {
	payload, sig, ok := strings.Cut(token, ".")
	if !ok {
		return Session{}, false
	}
	expected := m.mac(payload)
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, expected) {
		return Session{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return Session{}, false
	}
	var s Session
	if json.Unmarshal(raw, &s) != nil {
		return Session{}, false
	}
	if s.Exp < time.Now().Unix() || s.User == "" || s.Epoch != m.currentEpoch() {
		return Session{}, false
	}
	m.mu.Lock()
	record, ok := m.sessions[s.Nonce]
	if !ok {
		m.mu.Unlock()
		return Session{}, false
	}
	record.LastSeen = time.Now()
	m.sessions[s.Nonce] = record
	if time.Since(m.lastPersist) > 30*time.Second {
		m.saveSessionsLocked()
	}
	m.mu.Unlock()
	return s, true
}

// ListSessions returns active sessions, newest activity first.
func (m *Manager) ListSessions(currentNonce string) []SessionInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	out := make([]SessionInfo, 0, len(m.sessions))
	for nonce, record := range m.sessions {
		if now.After(record.Expires) {
			delete(m.sessions, nonce)
			continue
		}
		out = append(out, SessionInfo{
			Nonce: nonce, User: record.User, IP: record.IP,
			Created: record.Created, LastSeen: record.LastSeen, Expires: record.Expires,
			Current: nonce == currentNonce,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeen.After(out[j].LastSeen) })
	return out
}

// RevokeSession invalidates one session by nonce.
func (m *Manager) RevokeSession(nonce string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[nonce]; !ok {
		return false
	}
	delete(m.sessions, nonce)
	m.saveSessionsLocked()
	return true
}

// RevokeAll invalidates every signed-in session by rotating the session epoch.
// Existing cookies fail verification immediately; a new login receives the
// current epoch.
func (m *Manager) RevokeAll() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.epoch = time.Now().UnixNano()
	m.sessions = map[string]sessionRecord{}
	m.saveSessionsLocked()
	return m.epoch
}

func (m *Manager) secretPath() string {
	if m.persistDir == "" {
		return ""
	}
	return filepath.Join(m.persistDir, "auth-secret")
}

func (m *Manager) sessionsPath() string {
	if m.persistDir == "" {
		return ""
	}
	return filepath.Join(m.persistDir, "auth-sessions.json")
}

func (m *Manager) loadSecret() {
	path := m.secretPath()
	if path == "" {
		return
	}
	if raw, err := os.ReadFile(path); err == nil && len(raw) == 32 {
		m.secret = append([]byte(nil), raw...)
		return
	}
	if err := os.MkdirAll(m.persistDir, 0o750); err != nil {
		return
	}
	_ = os.WriteFile(path, m.secret, 0o600)
}

func (m *Manager) loadSessions() {
	path := m.sessionsPath()
	if path == "" {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var file struct {
		Epoch    int64                    `json:"epoch"`
		Sessions map[string]sessionRecord `json:"sessions"`
	}
	if json.Unmarshal(raw, &file) != nil {
		return
	}
	if file.Epoch > 0 {
		m.epoch = file.Epoch
	}
	now := time.Now()
	for nonce, record := range file.Sessions {
		if now.After(record.Expires) {
			continue
		}
		m.sessions[nonce] = record
	}
}

func (m *Manager) saveSessionsLocked() {
	path := m.sessionsPath()
	if path == "" {
		return
	}
	m.lastPersist = time.Now()
	if err := os.MkdirAll(m.persistDir, 0o750); err != nil {
		return
	}
	raw, err := json.MarshalIndent(struct {
		Epoch    int64                    `json:"epoch"`
		Sessions map[string]sessionRecord `json:"sessions"`
	}{Epoch: m.epoch, Sessions: m.sessions}, "", "  ")
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

func (m *Manager) currentEpoch() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.epoch
}

func (m *Manager) sign(s Session) (string, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	return payload + "." + base64.RawURLEncoding.EncodeToString(m.mac(payload)), nil
}

func (m *Manager) mac(payload string) []byte {
	h := hmac.New(sha256.New, m.secret)
	h.Write([]byte(payload))
	return h.Sum(nil)
}

func (m *Manager) Cookie(token string, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(m.ttl.Seconds()),
	}
}

func (m *Manager) ClearCookie() *http.Cookie {
	return &http.Cookie{
		Name: CookieName, Value: "", Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	}
}

func (m *Manager) FromRequest(r *http.Request) (Session, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return Session{}, false
	}
	return m.Verify(c.Value)
}

func (m *Manager) allow(ip string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.failures[ip]
	if !ok {
		return true
	}
	if time.Now().After(b.until) {
		delete(m.failures, ip)
		return true
	}
	return b.count < maxFailures
}

func (m *Manager) recordFailure(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.failures[ip]
	if !ok || time.Now().After(b.until) {
		b = &failureBucket{}
		m.failures[ip] = b
	}
	b.count++
	b.until = time.Now().Add(failureTTL)
}

func (m *Manager) clearFailures(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.failures, ip)
}

// lookupShadow uses getent first so NSS-backed directories work, then falls
// back to reading /etc/shadow directly.
func lookupShadow(username string) (string, error) {
	if out, err := exec.Command("/usr/bin/getent", "shadow", username).Output(); err == nil {
		if hash, err := parseShadowLine(strings.TrimSpace(string(out))); err == nil {
			return hash, nil
		}
	}
	raw, err := os.ReadFile("/etc/shadow")
	if err != nil {
		return "", fmt.Errorf("read /etc/shadow: %w", err)
	}
	hash, err := parseShadow(string(raw), username)
	if err != nil {
		return "", err
	}
	return hash, nil
}

func parseShadow(content, username string) (string, error) {
	prefix := username + ":"
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, prefix) {
			return parseShadowLine(line)
		}
	}
	return "", fmt.Errorf("shadow entry for %q not found", username)
}

func parseShadowLine(line string) (string, error) {
	fields := strings.Split(line, ":")
	if len(fields) < 2 {
		return "", errors.New("malformed shadow entry")
	}
	hash := fields[1]
	if hash == "" {
		return "", errors.New("shadow entry has no password hash")
	}
	if hash[0] == '!' || hash[0] == '*' {
		return "", errors.New("account is locked")
	}
	// Field 8 (index 7) is the account expiration in days since epoch.
	if len(fields) >= 8 {
		if days, err := strconv.ParseInt(fields[7], 10, 64); err == nil && days > 0 {
			if time.Now().Unix()/86400 > days {
				return "", errors.New("account has expired")
			}
		}
	}
	return hash, nil
}

func isUsableHash(hash string) bool {
	return hash != "" && hash[0] != '!' && hash[0] != '*'
}

func verifyShadowHash(password, hash string) error {
	if !crypt.IsHashSupported(hash) {
		return ErrUnsupportedHash
	}
	return crypt.NewFromHash(hash).Verify(hash, []byte(password))
}

func randomToken(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "fallback"
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

// ClientIP returns the immediate peer, trusting X-Forwarded-For only when the
// request comes from a local reverse proxy.
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			if first, _, ok := strings.Cut(fwd, ","); ok {
				return strings.TrimSpace(first)
			}
			return strings.TrimSpace(fwd)
		}
	}
	return host
}
