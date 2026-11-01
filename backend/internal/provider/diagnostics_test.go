package provider

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDiagnosticsArchive(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, "state")
	boot := filepath.Join(root, "boot")
	data := filepath.Join(root, "data")
	for _, dir := range []string{state, filepath.Join(boot, "config"), data} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(state, "var.ini"), []byte("NAME=\"vault\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(boot, "config", "disk.cfg"), []byte("spindownDelay=0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "settings.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	live := &Live{stateDir: state, bootDir: boot, dataDir: data}
	raw, err := live.Diagnostics(context.Background())
	if err != nil {
		t.Fatalf("diagnostics: %v", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	names := map[string]bool{}
	for _, file := range reader.File {
		names[file.Name] = true
	}
	for _, wanted := range []string{"README.txt", "state/var.ini", "boot/disk.cfg", "data/settings.json"} {
		if !names[wanted] {
			t.Errorf("missing %s in archive (%v)", wanted, names)
		}
	}
}
