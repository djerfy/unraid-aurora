package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTasksShowsShareScans(t *testing.T) {
	state := t.TempDir()
	// One completed share and one running share.
	if err := os.WriteFile(filepath.Join(state, "shares.ini"), []byte("[\"media\"]\n[\"appdata\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	live := &Live{
		stateDir:          state,
		shareUsage:        map[string]shareUsageEntry{"media": {Used: 1024, Updated: now}},
		shareUsageStarted: map[string]time.Time{"appdata": now},
		shareUsageErrors:  map[string]string{},
	}
	tasks, err := live.Tasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	status := map[string]string{}
	for _, task := range tasks {
		status[task.Name] = task.Status
	}
	if status["media"] != "completed" || status["appdata"] != "running" {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
}
