package config

import "testing"

func TestTLSDefaults(t *testing.T) {
	t.Setenv("AURORA_TLS_CERT", "")
	cfg, err := Load("test")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.TLSCert != "" {
		t.Fatalf("cert should be derived, got %q", cfg.TLSCert)
	}
	if cfg.TLSCertDir != "/boot/config/ssl/certs" {
		t.Fatalf("cert dir = %q", cfg.TLSCertDir)
	}
	if cfg.TLSKey != "" {
		t.Fatalf("key should follow the resolved bundle, got %q", cfg.TLSKey)
	}
}

func TestTLSOverrides(t *testing.T) {
	t.Setenv("AURORA_TLS_CERT", "/tmp/custom_bundle.pem")
	t.Setenv("AURORA_TLS_KEY", "/tmp/custom_key.pem")
	t.Setenv("AURORA_TLS_CERT_DIR", "/tmp/certs")
	t.Setenv("AURORA_SERVER_NAME", "custom")
	cfg, err := Load("test")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.TLSCert != "/tmp/custom_bundle.pem" || cfg.TLSKey != "/tmp/custom_key.pem" {
		t.Fatalf("unexpected TLS override: %+v", cfg)
	}
	if cfg.TLSCertDir != "/tmp/certs" || cfg.ServerName != "custom" {
		t.Fatalf("unexpected cert lookup override: %+v", cfg)
	}
}
