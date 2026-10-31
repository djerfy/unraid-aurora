package main

import (
	"os"
	"path/filepath"
	"testing"

	"aurora/internal/config"
)

func TestResolveTLSFromServerName(t *testing.T) {
	state := t.TempDir()
	certs := t.TempDir()
	if err := os.WriteFile(filepath.Join(state, "var.ini"), []byte("NAME=\"vault\"\nmdState=\"STARTED\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(certs, "vault_unraid_bundle.pem")
	if err := os.WriteFile(want, []byte("placeholder"), 0o600); err != nil {
		t.Fatal(err)
	}

	cert, key, err := resolveTLS(config.Config{StateDir: state, TLSCertDir: certs})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cert != want || key != want {
		t.Fatalf("cert=%q key=%q, want %q", cert, key, want)
	}
}

func TestResolveTLSExplicitWins(t *testing.T) {
	cert, key, err := resolveTLS(config.Config{
		TLSCert: "/tmp/explicit_bundle.pem",
		TLSKey:  "/tmp/explicit_key.pem",
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cert != "/tmp/explicit_bundle.pem" || key != "/tmp/explicit_key.pem" {
		t.Fatalf("explicit paths were not respected: %q %q", cert, key)
	}
}

func TestResolveTLSSingleBundleFallback(t *testing.T) {
	certs := t.TempDir()
	want := filepath.Join(certs, "oldsrv_unraid_bundle.pem")
	if err := os.WriteFile(want, []byte("placeholder"), 0o600); err != nil {
		t.Fatal(err)
	}
	cert, _, err := resolveTLS(config.Config{TLSCertDir: certs, ServerName: "newsrv"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cert != want {
		t.Fatalf("fallback = %q, want %q", cert, want)
	}
}

func TestResolveTLSAmbiguousBundles(t *testing.T) {
	certs := t.TempDir()
	for _, name := range []string{"one_unraid_bundle.pem", "two_unraid_bundle.pem"} {
		if err := os.WriteFile(filepath.Join(certs, name), []byte("placeholder"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := resolveTLS(config.Config{TLSCertDir: certs, ServerName: "expected"}); err == nil {
		t.Fatal("expected an ambiguity error")
	}
}

func TestResolveTLSRejectsTraversal(t *testing.T) {
	certs := t.TempDir()
	if _, _, err := resolveTLS(config.Config{TLSCertDir: certs, ServerName: "../../etc/passwd"}); err == nil {
		t.Fatal("expected a name error")
	}
}
