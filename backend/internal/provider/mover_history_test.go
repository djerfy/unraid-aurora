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
