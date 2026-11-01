package provider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAutostartFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unraid-autostart")
	if err := writeAutostartFile(path, "plex", true); err != nil {
		t.Fatal(err)
	}
	if err := writeAutostartFile(path, "sonarr", true); err != nil {
		t.Fatal(err)
	}
	if err := writeAutostartFile(path, "sonarr", false); err != nil {
		t.Fatal(err)
	}
	entries, err := readAutostartFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0] != "plex" {
		t.Fatalf("entries = %+v", entries)
	}

	// A wait suffix ("name;10") must survive a disable operation.
	if err := os.WriteFile(path, []byte("plex;10\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeAutostartFile(path, "plex", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("empty autostart file should be removed: %v", err)
	}
}
