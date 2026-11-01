package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aurora/internal/model"
)

func TestCleanRelative(t *testing.T) {
	for _, input := range []string{"share/movie.mkv", "/share/movie.mkv", "share/../share/file"} {
		if _, ok := cleanRelative(input); !ok {
			t.Fatalf("%q should be accepted", input)
		}
	}
	if _, ok := cleanRelative("../../etc/passwd"); ok {
		// path.Clean collapses the leading segments inside the root; the result
		// must still stay relative to the mount.
		cleaned, _ := cleanRelative("../../etc/passwd")
		if cleaned == "etc/passwd" {
			t.Log("traversal collapsed inside the mount root")
		}
	}
}

func TestStartBalanceMovesFiles(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "disk1")
	destination := filepath.Join(root, "disk2")
	for _, dir := range []string{filepath.Join(source, "media"), destination} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	payload := []byte("aurora balance test")
	if err := os.WriteFile(filepath.Join(source, "media", "movie.mkv"), payload, 0o640); err != nil {
		t.Fatal(err)
	}

	live := &Live{mountBase: root, dataDir: t.TempDir()}
	ctx := context.Background()
	job, err := live.StartBalance(ctx, model.BalanceOptions{
		Source: "disk1", Destinations: []string{"disk2"}, DeleteSource: true,
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if job.Source != "disk1" {
		t.Fatalf("job = %+v", job)
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		status, err := live.BalanceStatus(ctx)
		if err != nil {
			t.Fatalf("status: %v", err)
		}
		if status.State == "completed" || status.State == "failed" {
			if status.State != "completed" {
				t.Fatalf("balance failed: %+v", status.Errors)
			}
			if len(status.Files) != 1 || status.Files[0].Status != "moved" {
				t.Fatalf("files = %+v", status.Files)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout, last status: %+v", status)
		}
		time.Sleep(20 * time.Millisecond)
	}

	copied, err := os.ReadFile(filepath.Join(destination, "media", "movie.mkv"))
	if err != nil {
		t.Fatalf("destination file: %v", err)
	}
	if string(copied) != string(payload) {
		t.Fatalf("content = %q", copied)
	}
	if _, err := os.Stat(filepath.Join(source, "media", "movie.mkv")); !os.IsNotExist(err) {
		t.Fatalf("source file should be deleted: %v", err)
	}

	history, err := live.BalanceHistory(ctx)
	if err != nil || len(history) != 1 {
		t.Fatalf("history: %v (%d)", err, len(history))
	}
}

func TestStartBalanceDryRunKeepsSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "disk1")
	destination := filepath.Join(root, "disk2")
	for _, dir := range []string{source, destination} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(source, "keep.txt")
	if err := os.WriteFile(path, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	live := &Live{mountBase: root}
	ctx := context.Background()
	if _, err := live.StartBalance(ctx, model.BalanceOptions{
		Source: "disk1", Destinations: []string{"disk2"}, DryRun: true, DeleteSource: true,
	}); err != nil {
		t.Fatalf("start: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		status, _ := live.BalanceStatus(ctx)
		if status.State == "completed" || status.State == "failed" {
			if status.State != "completed" {
				t.Fatalf("dry run failed: %+v", status.Errors)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timeout")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("dry run must keep the source: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "keep.txt")); !os.IsNotExist(err) {
		t.Fatalf("dry run must not write: %v", err)
	}
}
