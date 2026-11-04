package auth

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Hashes generated with openssl passwd for the password "secret".
const (
	sha512Hash = "$6$testsalt$51yaaBMXXXt5vK522YOlIMZ267vqHtMIzc2klcsu3EEv/bkRDU9g3UmkypXf.NvlpPpIPK1nX5zdbCeJBiQbB/"
	sha256Hash = "$5$testsalt$0nJaoNBqGOzZuuF0ljY7kJtJzaDPzCc.//FOE7ialA8"
	md5Hash    = "$1$testsalt$OJ.YJJoS0dMNJv3Q6WOHX/"
)

func fixtureManager(ttl time.Duration, hash string) *Manager {
	return New(ttl,
		WithShadowLookup(func(username string) (string, error) {
			if username == "root" {
				return hash, nil
			}
			return "", errors.New("no entry")
		}),
		WithPasswordVerifier(verifyShadowHash),
	)
}

func TestShadowHashVerification(t *testing.T) {
	for name, hash := range map[string]string{"sha512": sha512Hash, "sha256": sha256Hash, "md5": md5Hash} {
		if err := verifyShadowHash("secret", hash); err != nil {
			t.Fatalf("%s hash should verify: %v", name, err)
		}
		if err := verifyShadowHash("wrong", hash); err == nil {
			t.Fatalf("%s hash accepted a wrong password", name)
		}
	}
	if err := verifyShadowHash("secret", "$y$j9T$unsupported"); !errors.Is(err, ErrUnsupportedHash) {
		t.Fatalf("expected unsupported hash error, got %v", err)
	}
}

func TestLoginRoot(t *testing.T) {
	m := fixtureManager(time.Hour, sha512Hash)
	token, session, err := m.Login("root", "secret", "127.0.0.1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if session.User != "root" {
		t.Fatalf("user = %q", session.User)
	}
	verified, ok := m.Verify(token)
	if !ok || verified.User != "root" {
		t.Fatal("token should verify")
	}
	tampered := token[:len(token)-2] + "aa"
	if _, ok := m.Verify(tampered); ok {
		t.Fatal("tampered token must not verify")
	}
}

func TestLoginRejectsNonRoot(t *testing.T) {
	m := fixtureManager(time.Hour, sha512Hash)
	for i, username := range []string{"admin", "Root", "", "root "} {
		if _, _, err := m.Login(username, "secret", fmt.Sprintf("10.1.0.%d", i+1)); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("username %q should be rejected, got %v", username, err)
		}
	}
}

func TestLoginRejectsLockedAndMissingAccounts(t *testing.T) {
	for _, hash := range []string{"!", "!!", "*", ""} {
		m := fixtureManager(time.Hour, hash)
		if _, _, err := m.Login("root", "secret", "127.0.0.1"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("locked hash %q should be rejected, got %v", hash, err)
		}
	}
	missing := New(time.Hour, WithShadowLookup(func(string) (string, error) {
		return "", errors.New("not found")
	}))
	if _, _, err := missing.Login("root", "secret", "127.0.0.1"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("missing account should be rejected, got %v", err)
	}
}

func TestLoginRateLimit(t *testing.T) {
	m := fixtureManager(time.Hour, sha512Hash)
	for i := 0; i < maxFailures; i++ {
		_, _, _ = m.Login("root", "bad", "10.0.0.1")
	}
	if _, _, err := m.Login("root", "secret", "10.0.0.1"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected rate limiting, got %v", err)
	}
	if _, _, err := m.Login("root", "secret", "10.0.0.2"); err != nil {
		t.Fatalf("another IP should be allowed: %v", err)
	}
}

func TestExpiredSession(t *testing.T) {
	m := fixtureManager(-time.Minute, sha512Hash)
	token, _, err := m.Login("root", "secret", "127.0.0.1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, ok := m.Verify(token); ok {
		t.Fatal("expired session must not verify")
	}
}

func TestParseShadow(t *testing.T) {
	content := strings.Join([]string{
		"daemon:*:19000:0:99999:7:::",
		"root:" + sha512Hash + ":19000:0:99999:7:::",
		"nobody:!::::::",
	}, "\n")

	hash, err := parseShadow(content, "root")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if hash != sha512Hash {
		t.Fatalf("unexpected hash %q", hash)
	}
	if _, err := parseShadow(content, "missing"); err == nil {
		t.Fatal("expected error for a missing account")
	}
	if _, err := parseShadow(content, "daemon"); err == nil {
		t.Fatal("locked account must be rejected")
	}
}

func TestParseShadowLineExpiry(t *testing.T) {
	expired := "root:" + sha512Hash + ":19000:0:99999:7::1:"
	if _, err := parseShadowLine(expired); err == nil {
		t.Fatal("expired account must be rejected")
	}
	future := "root:" + sha512Hash + ":19000:0:99999:7::99999:"
	if _, err := parseShadowLine(future); err != nil {
		t.Fatalf("valid account rejected: %v", err)
	}
}

func TestRevokeAllInvalidatesSessions(t *testing.T) {
	manager := fixtureManager(time.Hour, sha512Hash)
	token, _, err := manager.Login("root", "secret", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := manager.Verify(token); !ok {
		t.Fatal("session was rejected before revocation")
	}
	manager.RevokeAll()
	if _, ok := manager.Verify(token); ok {
		t.Fatal("session was accepted after revocation")
	}
	fresh, _, err := manager.Login("root", "secret", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := manager.Verify(fresh); !ok {
		t.Fatal("session issued after revocation was rejected")
	}
}

func TestSessionInventoryAndRevocation(t *testing.T) {
	manager := fixtureManager(time.Hour, sha512Hash)
	firstToken, firstSession, err := manager.Login("root", "secret", "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	secondToken, secondSession, err := manager.Login("root", "secret", "10.0.0.2")
	if err != nil {
		t.Fatal(err)
	}
	sessions := manager.ListSessions(firstSession.Nonce)
	if len(sessions) != 2 || !sessions[0].Current && !sessions[1].Current {
		t.Fatalf("unexpected sessions: %+v", sessions)
	}
	if !manager.RevokeSession(secondSession.Nonce) {
		t.Fatal("second session was not revoked")
	}
	if _, ok := manager.Verify(secondToken); ok {
		t.Fatal("revoked session was accepted")
	}
	if _, ok := manager.Verify(firstToken); !ok {
		t.Fatal("unrelated session was rejected")
	}
	if len(manager.ListSessions(firstSession.Nonce)) != 1 {
		t.Fatal("revoked session is still listed")
	}
}
