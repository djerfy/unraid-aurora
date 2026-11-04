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

	mu       sync.Mutex
	failures map[string]*failureBucket
	epoch    int64
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
	return s, true
}

// RevokeAll invalidates every signed-in session by rotating the session epoch.
// Existing cookies fail verification immediately; a new login receives the
// current epoch.
func (m *Manager) RevokeAll() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.epoch = time.Now().UnixNano()
	return m.epoch
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
