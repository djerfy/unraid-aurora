package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigHistoryWriteDiffAndRollback(t *testing.T) {
	dataDir := t.TempDir()
	hostDir := t.TempDir()
	live := &Live{dataDir: dataDir}
	path := filepath.Join(hostDir, "my-plex.xml")
	if err := os.WriteFile(path, []byte("version=1\nname=plex\n"), 0o640); err != nil {
		t.Fatal(err)
	}

	if err := live.writeConfigFile("container-template.update", "Update template", path, []byte("version=2\nname=plex\nports=32400\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if raw, _ := os.ReadFile(path); !strings.Contains(string(raw), "version=2") {
		t.Fatalf("file not written: %q", raw)
	}

	history, err := live.ConfigHistory(context.Background())
	if err != nil || len(history) != 1 {
		t.Fatalf("history: %v (%d)", err, len(history))
	}
	if history[0].Action != "container-template.update" || len(history[0].Files) != 1 {
		t.Fatalf("entry = %+v", history[0])
	}

	detail, err := live.ConfigHistoryDetail(context.Background(), history[0].ID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	diff := detail.Files[0].Diff
	if !strings.Contains(diff, "-version=1") || !strings.Contains(diff, "+version=2") {
		t.Fatalf("diff = %q", diff)
	}

	if err := live.RollbackConfig(context.Background(), detail.ID); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "version=1\nname=plex\n" {
		t.Fatalf("rollback content = %q", raw)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}

	history, _ = live.ConfigHistory(context.Background())
	if len(history) != 2 || history[0].Action != "rollback" {
		t.Fatalf("rollback should be recorded: %+v", history)
	}
}

func TestConfigHistoryRejectsTraversal(t *testing.T) {
	live := &Live{dataDir: t.TempDir()}
	if _, err := live.ConfigHistoryDetail(context.Background(), "../../etc/passwd"); err == nil {
		t.Fatal("path traversal must be rejected")
	}
}

func TestConfigHistoryDeleteAndRestore(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(t.TempDir(), "unraid-autostart")
	if err := os.WriteFile(path, []byte("plex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	live := &Live{dataDir: dataDir}
	if err := live.deleteConfigFile("container.autostart", "Remove entry", path); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file should be deleted: %v", err)
	}
	history, _ := live.ConfigHistory(context.Background())
	if len(history) != 1 {
		t.Fatalf("history = %+v", history)
	}
	if err := live.RollbackConfig(context.Background(), history[0].ID); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "plex\n" {
		t.Fatalf("restored = %q", raw)
	}
}
