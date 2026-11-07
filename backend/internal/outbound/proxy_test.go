package outbound

import (
	"net/http"
	"testing"
)

func TestConfigureAndTransport(t *testing.T) {
	t.Cleanup(func() { _ = Configure(false, "", "", "") })

	if err := Configure(true, "http://proxy.local:3128", "user", "secret"); err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	proxy, err := Transport().Proxy(request)
	if err != nil {
		t.Fatal(err)
	}
	if proxy == nil || proxy.Host != "proxy.local:3128" {
		t.Fatalf("unexpected proxy: %v", proxy)
	}
	if proxy.User == nil || proxy.User.Username() != "user" {
		t.Fatalf("credentials were not applied: %v", proxy.User)
	}
	if password, _ := proxy.User.Password(); password != "secret" {
		t.Fatalf("unexpected password")
	}
	env := Env()
	if len(env) != 4 {
		t.Fatalf("expected 4 proxy variables, got %v", env)
	}

	if err := Configure(false, "", "", ""); err != nil {
		t.Fatal(err)
	}
	proxy, err = Transport().Proxy(request)
	if err != nil || proxy != nil {
		t.Fatalf("expected no proxy, got %v (%v)", proxy, err)
	}
	if Env() != nil {
		t.Fatalf("expected no proxy variables")
	}
}

func TestConfigureRejectsInvalidURLs(t *testing.T) {
	for _, value := range []string{"ftp://proxy.local", "proxy.local:3128", ""} {
		if err := Configure(true, value, "", ""); err == nil {
			t.Fatalf("%q should be rejected", value)
		}
	}
}
