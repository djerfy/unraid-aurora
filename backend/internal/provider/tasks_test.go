package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aurora/internal/model"
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
	for _, task := range tasks {
		if task.Path == "" || task.Command == "" || task.Description == "" {
			t.Fatalf("task is missing context: %+v", task)
		}
	}
}

func TestTasksShowsTrivyImageScans(t *testing.T) {
	now := time.Now()
	live := &Live{
		shareUsage:        map[string]shareUsageEntry{},
		shareUsageStarted: map[string]time.Time{},
		shareUsageErrors:  map[string]string{},
		trivyScans: func() []model.TrivyScan {
			finished := now
			return []model.TrivyScan{
				{Image: "linuxserver/jackett:latest", Status: "running", StartedAt: now, Running: true},
				{Image: "bitnami/mongodb:latest", Status: "queued", StartedAt: now.Add(-time.Minute)},
				{Image: "redis:7", Status: "completed", StartedAt: now.Add(-2 * time.Minute), FinishedAt: &finished},
			}
		},
	}
	tasks, err := live.Tasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]model.Task{}
	for _, task := range tasks {
		byName[task.Name] = task
	}
	running, ok := byName["linuxserver/jackett:latest"]
	if !ok || running.Status != "running" || running.Category != "security" {
		t.Fatalf("running scan is missing: %+v", tasks)
	}
	queued, ok := byName["bitnami/mongodb:latest"]
	if !ok || queued.Status != "queued" {
		t.Fatalf("queued scan is missing: %+v", tasks)
	}
	if running.Command != "trivy image linuxserver/jackett:latest" {
		t.Fatalf("unexpected command: %q", running.Command)
	}
	completed, ok := byName["redis:7"]
	if !ok || completed.Status != "completed" || completed.FinishedAt == nil || completed.Duration != 120 {
		t.Fatalf("completed scan is missing: %+v", completed)
	}
}
