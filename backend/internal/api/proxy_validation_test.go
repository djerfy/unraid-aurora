package api

import (
	"strings"
	"testing"

	"aurora/internal/model"
)

func TestNormalizeProxySettings(t *testing.T) {
	settings := model.Settings{
		ProxyEnabled: true,
		Proxies: []model.ProxyConfig{
			{Name: "Home", URL: "http://127.0.0.1:3128", Username: "user", Password: "secret"},
			{Name: "Backup", URL: "socks5://127.0.0.1:1080"},
		},
	}
	if err := normalizeProxySettings(&settings); err != nil {
		t.Fatal(err)
	}
	if settings.ProxyActiveID != settings.Proxies[0].ID {
		t.Fatalf("expected the first proxy to be active, got %q", settings.ProxyActiveID)
	}
	if settings.Proxies[0].ID == "" || settings.Proxies[1].ID == "" {
		t.Fatalf("expected generated identifiers: %+v", settings.Proxies)
	}

	cases := map[string]model.Settings{
		"too many": {Proxies: []model.ProxyConfig{
			{Name: "1", URL: "http://a:1"}, {Name: "2", URL: "http://b:2"},
			{Name: "3", URL: "http://c:3"}, {Name: "4", URL: "http://d:4"}, {Name: "5", URL: "http://e:5"},
		}},
		"missing name":     {Proxies: []model.ProxyConfig{{URL: "http://a:1"}}},
		"missing url":      {Proxies: []model.ProxyConfig{{Name: "Home"}}},
		"bad scheme":       {Proxies: []model.ProxyConfig{{Name: "Home", URL: "ftp://a:1"}}},
		"password no user": {Proxies: []model.ProxyConfig{{Name: "Home", URL: "http://a:1", Password: "x"}}},
		"enabled empty":    {ProxyEnabled: true},
	}
	for name, value := range cases {
		value := value
		if err := normalizeProxySettings(&value); err == nil {
			t.Fatalf("%s should be rejected", name)
		}
	}
}

func TestRedactSettingsClearsProxyPasswords(t *testing.T) {
	settings := redactSettings(model.Settings{
		Proxies: []model.ProxyConfig{{ID: "p1", Name: "Home", URL: "http://a:1", Username: "u", Password: "secret"}},
	})
	if settings.Proxies[0].Password != "" {
		t.Fatalf("proxy password was not redacted")
	}
	if !strings.EqualFold(settings.Proxies[0].Name, "Home") {
		t.Fatalf("non-secret proxy fields should be kept")
	}
}
