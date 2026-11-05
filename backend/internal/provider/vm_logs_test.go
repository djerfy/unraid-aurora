package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withVMLogDir(t *testing.T, dir string) {
	t.Helper()
	previous := vmLogDirs
	vmLogDirs = []string{dir}
	t.Cleanup(func() { vmLogDirs = previous })
}

func TestReadVMLogParsesLibvirtLines(t *testing.T) {
	dir := t.TempDir()
	withVMLogDir(t, dir)
	content := strings.Join([]string{
		"2026-11-05 18:00:00.000+0000: starting up",
		"2026-11-05 18:00:01.250+0000: warning: spice keyboard not found",
		"plain line without timestamp",
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, "Test VM.log"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	lines, err := readVMLog("Test VM", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0].Message != "starting up" || lines[0].Time.Year() != 2026 {
		t.Fatalf("unexpected first line: %+v", lines[0])
	}
	if lines[1].Level != "warn" || !strings.HasPrefix(lines[1].Message, "warning:") {
		t.Fatalf("unexpected second line: %+v", lines[1])
	}
	if lines[2].Message != "plain line without timestamp" {
		t.Fatalf("unexpected third line: %+v", lines[2])
	}
}

func TestReadVMLogRejectsPathSeparators(t *testing.T) {
	dir := t.TempDir()
	withVMLogDir(t, dir)
	for _, name := range []string{"", ".", "..", "../etc/passwd", "qemu/domain", `qemu\domain`} {
		if _, err := readVMLog(name, 10); err == nil {
			t.Fatalf("expected %q to be rejected", name)
		}
	}
}

func TestReadVMLogIgnoresSymlinks(t *testing.T) {
	dir := t.TempDir()
	withVMLogDir(t, dir)
	target := filepath.Join(dir, "outside.txt")
	if err := os.WriteFile(target, []byte("2026-11-05 18:00:00.000+0000: secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "linked.log")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	lines, err := readVMLog("linked", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 0 {
		t.Fatalf("symlinks must not be followed, got %d lines", len(lines))
	}
}

func TestReadVMLogMissingFileIsEmpty(t *testing.T) {
	withVMLogDir(t, t.TempDir())
	lines, err := readVMLog("never-started", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 0 {
		t.Fatalf("expected no lines, got %d", len(lines))
	}
}
