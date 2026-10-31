package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"

	"aurora/internal/auth"
	"aurora/internal/config"
	"aurora/internal/metrics"
	"aurora/internal/model"
	"aurora/internal/provider"
)

// fakeProvider exposes the handful of methods the HTTP tests exercise. The
// embedded interface intentionally panics if a handler reaches an
// unimplemented method, which keeps the test surface honest.
type fakeProvider struct {
	provider.Provider
	settings model.Settings
}

func (f *fakeProvider) Name() string { return "live" }

func (f *fakeProvider) Snapshot(ctx context.Context) (model.Snapshot, error) {
	return model.Snapshot{
		System: model.System{Name: "Vault", Version: "7.2.4"},
		Array:  model.Array{State: "STARTED", Status: "protected"},
		Containers: []model.Container{
			{ID: "abc", Name: "plex", State: "running", Ports: []model.Port{}},
		},
		Notifications: []model.Notification{},
		Disks:         []model.Disk{},
		Shares:        []model.Share{},
		Pools:         []model.Pool{},
		VMs:           []model.VM{},
		Users:         []model.User{},
		Plugins:       []model.Plugin{},
		History:       []model.MetricPoint{},
		Metrics:       model.Metrics{CPU: 10, Cores: []float64{10}},
		Settings:      f.settings,
	}, nil
}

func (f *fakeProvider) Metrics(ctx context.Context) (model.Metrics, error) {
	return model.Metrics{CPU: 10, Cores: []float64{10}}, nil
}

func (f *fakeProvider) History(ctx context.Context) ([]model.MetricPoint, error) {
	return []model.MetricPoint{}, nil
}

func (f *fakeProvider) Settings(ctx context.Context) (model.Settings, error) {
	return f.settings, nil
}

func (f *fakeProvider) SaveSettings(ctx context.Context, s model.Settings) error {
	f.settings = s
	return nil
}

func (f *fakeProvider) Perform(ctx context.Context, action model.Action) error {
	if action.Kind == provider.ActionArrayStart {
		return nil
	}
	return errors.New("unsupported in fake")
}

func fakeAuth() *auth.Manager {
	return auth.New(time.Hour,
		auth.WithShadowLookup(func(username string) (string, error) {
			if username == "root" {
				return "$6$fake$hash", nil
			}
			return "", errors.New("missing")
		}),
		auth.WithPasswordVerifier(func(password, hash string) error {
			if password == "secret" {
				return nil
			}
			return errors.New("mismatch")
		}),
	)
}

type testEnv struct {
	server *httptest.Server
	client *http.Client
}

func newTestEnv(t *testing.T, readOnly bool) *testEnv {
	t.Helper()
	cfg := config.Config{
		Version:    "test",
		SessionTTL: time.Hour,
		ReadOnly:   readOnly,
	}
	p := &fakeProvider{}
	hub := metrics.NewHub(p)
	srv := New(cfg, p, hub, fakeAuth(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	server := httptest.NewServer(srv.Handler())
	t.Cleanup(server.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &testEnv{server: server, client: &http.Client{Jar: jar}}
}

func (e *testEnv) request(t *testing.T, method, path string, body any, headers map[string]string) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, e.server.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestHealth(t *testing.T) {
	env := newTestEnv(t, false)
	resp := env.request(t, http.MethodGet, "/api/v1/health", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d", resp.StatusCode)
	}
	var health map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatal(err)
	}
	if health["status"] != "ok" || health["provider"] != "live" {
		t.Fatalf("unexpected health payload: %v", health)
	}
}

func TestBootstrapRequiresAuthentication(t *testing.T) {
	env := newTestEnv(t, false)
	resp := env.request(t, http.MethodGet, "/api/v1/bootstrap", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthenticationFlow(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}

	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "wrong"}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad password, got %d", resp.StatusCode)
	}

	resp = env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for valid login, got %d", resp.StatusCode)
	}

	resp = env.request(t, http.MethodGet, "/api/v1/auth/session", nil, nil)
	defer resp.Body.Close()
	var session struct {
		Authenticated bool   `json:"authenticated"`
		User          string `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	if !session.Authenticated || session.User != "root" {
		t.Fatalf("unexpected session: %+v", session)
	}

	resp = env.request(t, http.MethodGet, "/api/v1/bootstrap", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with session cookie, got %d", resp.StatusCode)
	}
	var snap model.Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		t.Fatal(err)
	}
	if len(snap.Containers) != 1 || snap.Containers[0].Name != "plex" {
		t.Fatalf("unexpected snapshot: %+v", snap.Containers)
	}

	resp = env.request(t, http.MethodPost, "/api/v1/auth/logout", map[string]string{}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logout status = %d", resp.StatusCode)
	}
	resp = env.request(t, http.MethodGet, "/api/v1/bootstrap", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", resp.StatusCode)
	}
}

func TestLoginRejectsNonRoot(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "admin", "password": "secret"}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestActionsRequireCSRFHeader(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	resp.Body.Close()

	resp = env.request(t, http.MethodPost, "/api/v1/actions",
		model.Action{Kind: provider.ActionArrayStart}, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 without header, got %d", resp.StatusCode)
	}

	resp = env.request(t, http.MethodPost, "/api/v1/actions",
		model.Action{Kind: provider.ActionArrayStart}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with header, got %d", resp.StatusCode)
	}
}

func TestReadOnlyBlocksMutations(t *testing.T) {
	env := newTestEnv(t, true)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	resp.Body.Close()

	resp = env.request(t, http.MethodPost, "/api/v1/actions",
		model.Action{Kind: provider.ActionArrayStart}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 in read-only mode, got %d", resp.StatusCode)
	}
}

func TestSettingsValidation(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	resp.Body.Close()

	settings := model.Settings{Theme: "neon", Accent: "ember", RefreshInterval: 5}
	resp = env.request(t, http.MethodPut, "/api/v1/settings", settings, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for invalid theme, got %d", resp.StatusCode)
	}

	settings.Theme = "dark"
	resp = env.request(t, http.MethodPut, "/api/v1/settings", settings, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for valid settings, got %d", resp.StatusCode)
	}
}

func TestUnknownActionIsRejected(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	resp.Body.Close()

	resp = env.request(t, http.MethodPost, "/api/v1/actions",
		model.Action{Kind: "array.explode"}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}
