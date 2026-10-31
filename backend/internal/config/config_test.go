package config

import "testing"

func TestTLSDefaults(t *testing.T) {
	t.Setenv("AURORA_TLS_CERT", "")
	if _, err := Load("test"); err == nil {
		t.Fatal("empty AURORA_TLS_CERT must be rejected")
	}

	t.Setenv("AURORA_TLS_CERT", "/tmp/certificate_bundle.pem")
	t.Setenv("AURORA_TLS_KEY", "")
	cfg, err := Load("test")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.TLSCert != "/tmp/certificate_bundle.pem" {
		t.Fatalf("cert = %q", cfg.TLSCert)
	}
	if cfg.TLSKey != cfg.TLSCert {
		t.Fatalf("key should default to the bundle path, got %q", cfg.TLSKey)
	}
}

func TestTLSKeyOverride(t *testing.T) {
	t.Setenv("AURORA_TLS_CERT", "/tmp/cert.pem")
	t.Setenv("AURORA_TLS_KEY", "/tmp/key.pem")
	cfg, err := Load("test")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.TLSKey != "/tmp/key.pem" {
		t.Fatalf("key = %q", cfg.TLSKey)
	}
}
