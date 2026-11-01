package provider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadLogSourceFallsBackToExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "maillog")
	if err := os.WriteFile(path, []byte("warning: queue slow\nfatal: error connecting\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lines, err := readLogSource("maillog", 10, filepath.Join(dir, "missing"), path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if len(lines) != 2 || lines[0].Source != "maillog" {
		t.Fatalf("lines = %+v", lines)
	}
	if lines[0].Level != "warn" || lines[1].Level != "error" {
		t.Fatalf("levels = %q / %q", lines[0].Level, lines[1].Level)
	}
}

func TestBuildLogLinesDebug(t *testing.T) {
	lines := buildLogLines("libvirt", []string{"debug: domain started"})
	if len(lines) != 1 || lines[0].Level != "debug" {
		t.Fatalf("lines = %+v", lines)
	}
}
