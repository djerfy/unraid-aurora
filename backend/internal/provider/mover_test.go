package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aurora/internal/model"
)

func TestMoverProgressPercent(t *testing.T) {
	if progressPercent(1000, 400) != 60 {
		t.Fatalf("unexpected progress: %v", progressPercent(1000, 400))
	}
	if progressPercent(0, 100) != 0 || progressPercent(100, 100) != 0 {
		t.Fatal("zero or completed totals must progress from zero")
	}
}

func TestMoverFileFromSyslogLine(t *testing.T) {
	file, ok := moverFileFromLine("Nov  4 03:12:01 vault mover: move: /mnt/user/media/movie.mkv Success")
	if !ok || file != "/mnt/user/media/movie.mkv" {
		t.Fatalf("unexpected parse: %q %v", file, ok)
	}
	if _, ok := moverFileFromLine("Nov  4 03:12:01 vault mover: started"); ok {
		t.Fatal("unrelated line should be ignored")
	}
}

func TestEnrichMoverStatusUsesObservedTotals(t *testing.T) {
	live := &Live{
		moverStarted:         time.Now().Add(-10 * time.Second),
		moverCandidateTotal:  1000,
		moverCandidateRemain: 400,
	}
	status := model.MoverStatus{Running: true}
	live.enrichMoverStatusLocked(&status)
	if status.Total != 1000 || status.Remain != 400 {
		t.Fatalf("candidate totals were not applied: %+v", status)
	}
	if status.Progress != 60 {
		t.Fatalf("unexpected progress: %v", status.Progress)
	}
	if status.Throughput <= 0 || status.ETA <= 0 {
		t.Fatalf("throughput and ETA should be derived: %+v", status)
	}
	status.Remain = 0
	live.enrichMoverStatusLocked(&status)
	if status.Progress != 100 {
		t.Fatalf("completed mover should reach 100%%: %v", status.Progress)
	}
}

func TestReadMoverStatusUsesStateFiles(t *testing.T) {
	dir := t.TempDir()
	ini := `TotalSize=1000000
RemainSize=250000
Throughput=5000000
ETA=30
File="/mnt/cache/media/movie.mkv"
Action="Moving share media"
Filepercent=50
MoverPid=123
`
	if err := os.WriteFile(filepath.Join(dir, "mover.ini"), []byte(ini), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "var.ini"), []byte("shareMoverProgress=\"yes\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	live := &Live{stateDir: dir}
	status, err := live.readMoverStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Total != 1000000 || status.Remain != 250000 {
		t.Fatalf("unexpected totals: %+v", status)
	}
	if status.Progress != 75 {
		t.Fatalf("unexpected progress: %v", status.Progress)
	}
	if status.Throughput != 5000000 || status.ETA != 30 {
		t.Fatalf("unexpected throughput/eta: %+v", status)
	}
	if !status.ProgressEnabled || status.ProgressSource != "mover.ini" {
		t.Fatalf("unexpected progress metadata: %+v", status)
	}
	if status.File != "/mnt/cache/media/movie.mkv" || status.PID != 123 {
		t.Fatalf("unexpected file/pid: %+v", status)
	}
}

func TestMoverProgressFlagFromVarINI(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "var.ini"), []byte("shareMoverProgress=\"yes\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	live := &Live{stateDir: dir}
	if !live.moverProgressEnabled() {
		t.Fatal("shareMoverProgress from var.ini was ignored")
	}
}

func TestMoverFileListFallback(t *testing.T) {
	previousPrefix := moverMountPrefix
	root := t.TempDir()
	moverMountPrefix = root + "/"
	defer func() { moverMountPrefix = previousPrefix }()

	shareDir := filepath.Join(root, "media")
	if err := os.MkdirAll(shareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(shareDir, "a.bin")
	second := filepath.Join(shareDir, "b.bin")
	if err := os.WriteFile(first, make([]byte, 1000), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, make([]byte, 2000), 0o644); err != nil {
		t.Fatal(err)
	}

	listDir := t.TempDir()
	t.Setenv("TMPDIR", listDir)
	listPath := filepath.Join(listDir, "mover-files.test")
	if err := os.WriteFile(listPath, []byte(first+"\n"+second+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	live := &Live{moverSeen: map[string]bool{}}
	live.scanMoverFileListsLocked()
	if live.moverCandidateTotal != 3000 || live.moverCandidateRemain != 3000 {
		t.Fatalf("unexpected candidate totals: %d/%d", live.moverCandidateTotal, live.moverCandidateRemain)
	}
	if err := os.Remove(first); err != nil {
		t.Fatal(err)
	}
	live.updateMoverCandidatesLocked(0)

	status := model.MoverStatus{Running: true}
	live.enrichMoverStatusLocked(&status)
	if status.Total != 3000 || status.Remain != 2000 {
		t.Fatalf("unexpected enriched totals: %+v", status)
	}
	if status.Progress != 33.3 {
		t.Fatalf("unexpected progress: %v", status.Progress)
	}
	if status.ProgressSource != "file-list" {
		t.Fatalf("unexpected source: %q", status.ProgressSource)
	}
}

func TestMoverTuningFilteredListFallback(t *testing.T) {
	previousPrefix := moverMountPrefix
	root := t.TempDir()
	moverMountPrefix = root + "/"
	defer func() { moverMountPrefix = previousPrefix }()

	shareDir := filepath.Join(root, "media")
	if err := os.MkdirAll(shareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(shareDir, "episode.mkv")
	if err := os.WriteFile(candidate, make([]byte, 4096), 0o644); err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	dir := filepath.Join(tmp, "ca.mover.tuning")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := "cache|array|media|yes|1700000000|0|0|4096|1|1|42|f|" + candidate + "\n"
	if err := os.WriteFile(filepath.Join(dir, "Filtered_files_test.list"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	live := &Live{moverSeen: map[string]bool{}}
	live.scanMoverTuningListsLocked()
	if live.moverCandidateTotal != 4096 || live.moverCandidateRemain != 4096 {
		t.Fatalf("unexpected tuning totals: %d/%d", live.moverCandidateTotal, live.moverCandidateRemain)
	}
}
