package provider

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aurora/internal/model"
)

func TestLoadMoverHistoryCapsAtTen(t *testing.T) {
	dir := t.TempDir()
	runs := make([]model.MoverRun, 12)
	for index := range runs {
		runs[index].Status = "completed"
	}
	raw, err := json.Marshal(runs)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mover-history.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	live := &Live{dataDir: dir}
	live.loadMoverHistory()
	if len(live.moverHistory) != moverHistoryLimit {
		t.Fatalf("history length = %d, want %d", len(live.moverHistory), moverHistoryLimit)
	}
}

func TestMoverFileFromLine(t *testing.T) {
	cases := []struct {
		line string
		file string
		ok   bool
	}{
		{"move: /mnt/cache/media/movie.mkv Success", "/mnt/cache/media/movie.mkv", true},
		{"move: /mnt/cache/media/My File.mkv File exists", "/mnt/cache/media/My File.mkv", true},
		{"move: /mnt/cache/media/partial.mkv Not moved", "/mnt/cache/media/partial.mkv", true},
		{"skip: /mnt/cache/iso/linux.iso", "", false},
		{"mover: started", "", false},
		{"move: relative/path Success", "", false},
	}
	for _, tc := range cases {
		file, ok := moverFileFromLine(tc.line)
		if ok != tc.ok || file != tc.file {
			t.Fatalf("moverFileFromLine(%q) = %q, %v; want %q, %v", tc.line, file, ok, tc.file, tc.ok)
		}
	}
}

func TestTailMoverOutputDeduplicates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mover-output.log")
	content := "move: /mnt/cache/a.mkv Success\n" +
		"move: /mnt/cache/a.mkv Success\n" +
		"skip: /mnt/cache/b.mkv\n" +
		"move: /mnt/cache/c.mkv File exists\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	live := &Live{moverOutput: path, moverSeen: map[string]bool{}}
	live.tailMoverOutputLocked()
	if len(live.moverFiles) != 2 {
		t.Fatalf("files = %v, want 2 entries", live.moverFiles)
	}
	if live.moverFiles[0] != "/mnt/cache/a.mkv" || live.moverFiles[1] != "/mnt/cache/c.mkv" {
		t.Fatalf("unexpected files: %v", live.moverFiles)
	}
	live.tailMoverOutputLocked()
	if len(live.moverFiles) != 2 {
		t.Fatalf("files were not deduplicated: %v", live.moverFiles)
	}
}
