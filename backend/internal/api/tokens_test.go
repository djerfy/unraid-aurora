package api

import (
	"net/http"
	"testing"
	"time"

	"aurora/internal/model"
)

func TestTokenStoreLifecycle(t *testing.T) {
	dir := t.TempDir()
	store := newTokenStore(dir)
	token, secret, err := store.create("prometheus", TokenRoleViewer, nil)
	if err != nil {
		t.Fatal(err)
	}
	if secret == "" || token.Name != "prometheus" || token.Role != TokenRoleViewer || !token.ReadOnly {
		t.Fatalf("unexpected token: %+v", token)
	}
	if _, ok := store.validate(secret); !ok {
		t.Fatal("issued token was not accepted")
	}
	if _, ok := store.validate("wrong"); ok {
		t.Fatal("invalid token was accepted")
	}
	reloaded := newTokenStore(dir)
	if _, ok := reloaded.validate(secret); !ok {
		t.Fatal("token was not persisted")
	}
	if err := store.revoke(token.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.validate(secret); ok {
		t.Fatal("revoked token was accepted")
	}
}

func TestTokenRolePolicy(t *testing.T) {
	cases := []struct {
		role   string
		method string
		path   string
		want   bool
	}{
		{TokenRoleViewer, http.MethodGet, "/api/v1/docker/containers", true},
		{TokenRoleViewer, http.MethodPost, "/api/v1/docker/containers/plex/start", false},
		{TokenRoleOperator, http.MethodPost, "/api/v1/docker/containers/plex/start", true},
		{TokenRoleOperator, http.MethodPost, "/api/v1/vms/ubuntu/start", true},
		{TokenRoleOperator, http.MethodPost, "/api/v1/settings", false},
		{TokenRoleOperator, http.MethodPut, "/api/v1/users/root", false},
		{TokenRoleAdmin, http.MethodPut, "/api/v1/settings", true},
		{TokenRoleViewer, http.MethodGet, "/api/v1/settings", false},
		{TokenRoleViewer, http.MethodGet, "/api/v1/terminal", false},
		{TokenRoleViewer, http.MethodGet, "/api/v1/users/root/icon", true},
		{TokenRoleOperator, http.MethodGet, "/api/v1/vms/test/console", true},
		{TokenRoleOperator, http.MethodGet, "/api/v1/docker/containers/test/config", true},
		{TokenRoleViewer, http.MethodGet, "/api/v1/config/export", false},
		{TokenRoleAdmin, http.MethodGet, "/api/v1/config/export", true},
	}
	for _, tc := range cases {
		if got := tokenRoleAllows(tc.role, tc.method, tc.path); got != tc.want {
			t.Fatalf("tokenRoleAllows(%q, %s, %s) = %v, want %v", tc.role, tc.method, tc.path, got, tc.want)
		}
	}
}

func TestRedactSettings(t *testing.T) {
	redacted := redactSettings(model.Settings{
		DiscordWebhook: "https://discord.test/secret",
		NtfyURL:        "https://ntfy.test/secret",
		WebhookURL:     "https://example.test/secret",
		WebhookHeaders: map[string]string{"Authorization": "Bearer secret"},
		WebhookBody:    "secret",
		Theme:          "dark",
	})
	if redacted.DiscordWebhook != "" || redacted.NtfyURL != "" || redacted.WebhookURL != "" || redacted.WebhookHeaders != nil || redacted.WebhookBody != "" {
		t.Fatalf("secrets were not removed: %+v", redacted)
	}
	if redacted.Theme != "dark" {
		t.Fatal("unrelated settings were changed")
	}
}

func TestTokenExpiry(t *testing.T) {
	store := newTokenStore(t.TempDir())
	past := time.Now().Add(-time.Hour)
	token, secret, err := store.create("expired", TokenRoleViewer, &past)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.validate(secret); ok {
		t.Fatalf("expired token %s was accepted", token.ID)
	}
	future := time.Now().Add(time.Hour)
	if _, secret, err = store.create("valid", TokenRoleViewer, &future); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.validate(secret); !ok {
		t.Fatal("valid token was rejected")
	}
}
