package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequireUnraid7AcceptsOnlyMajorSeven(t *testing.T) {
	for _, version := range []string{"7", "7.0.0", "7.1.4", "v7.2.0-beta.1"} {
		if err := RequireUnraid7(version); err != nil {
			t.Fatalf("%q: %v", version, err)
		}
	}
	for _, version := range []string{"", "unraid", "6", "6.12.10", "8.0.0", "8.1.0-rc1"} {
		if err := RequireUnraid7(version); err == nil {
			t.Fatalf("%q must be rejected", version)
		}
	}
}

func TestDetectUnraidVersionPrefersSystemFile(t *testing.T) {
	dir := t.TempDir()
	system := filepath.Join(dir, "unraid-version")
	if err := os.WriteFile(system, []byte("version=\"7.1.4\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := unraidVersionFile
	unraidVersionFile = system
	t.Cleanup(func() { unraidVersionFile = previous })

	state := t.TempDir()
	if err := os.WriteFile(filepath.Join(state, "var.ini"), []byte("version=\"6.12.10\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	version, err := DetectUnraidVersion(state)
	if err != nil {
		t.Fatal(err)
	}
	if version != "7.1.4" {
		t.Fatalf("version = %q", version)
	}
}

func TestDetectUnraidVersionFallsBackToVarIni(t *testing.T) {
	previous := unraidVersionFile
	unraidVersionFile = filepath.Join(t.TempDir(), "missing")
	t.Cleanup(func() { unraidVersionFile = previous })

	state := t.TempDir()
	if err := os.WriteFile(filepath.Join(state, "var.ini"), []byte("NAME=\"tower\"\nversion=\"7.0.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	version, err := DetectUnraidVersion(state)
	if err != nil {
		t.Fatal(err)
	}
	if version != "7.0.0" {
		t.Fatalf("version = %q", version)
	}
}

func TestDetectUnraidVersionRejectsUnknownHost(t *testing.T) {
	previous := unraidVersionFile
	unraidVersionFile = filepath.Join(t.TempDir(), "missing")
	t.Cleanup(func() { unraidVersionFile = previous })

	if _, err := DetectUnraidVersion(t.TempDir()); err == nil || !strings.Contains(err.Error(), "cannot determine") {
		t.Fatalf("expected a detection error, got %v", err)
	}
}

func TestParseUnraidVersionPlainText(t *testing.T) {
	if got := parseUnraidVersion("7.2.1\n"); got != "7.2.1" {
		t.Fatalf("plain version = %q", got)
	}
	if got := parseUnraidVersion("version=\"7.0.0-rc2\"\n"); got != "7.0.0-rc2" {
		t.Fatalf("assignment version = %q", got)
	}
}
