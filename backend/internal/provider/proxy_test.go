package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"aurora/internal/model"
	"aurora/internal/outbound"
)

func TestSaveSettingsAppliesProxy(t *testing.T) {
	t.Cleanup(func() { _ = outbound.Configure(false, "", "", "") })
	boot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(boot, "config", "aurora"), 0o755); err != nil {
		t.Fatal(err)
	}
	live := &Live{bootDir: boot}
	settings := model.Settings{
		Theme: "dark", Accent: "ember", RefreshInterval: 5,
		ProxyEnabled: true, ProxyActiveID: "p1",
		Proxies: []model.ProxyConfig{{ID: "p1", Name: "Home", URL: "http://127.0.0.1:3128", Username: "user", Password: "secret"}},
	}
	if err := live.SaveSettings(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	proxy := outbound.Current()
	if proxy == nil || proxy.Host != "127.0.0.1:3128" {
		t.Fatalf("proxy was not applied: %v", proxy)
	}

	settings.ProxyEnabled = false
	if err := live.SaveSettings(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	if outbound.Current() != nil {
		t.Fatalf("proxy should be cleared")
	}
}
