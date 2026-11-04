package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurora/internal/model"
)

func TestTasksShowsOnlyRunningShareScans(t *testing.T) {
	state := t.TempDir()
	// One finished share and one running share.
	if err := os.WriteFile(filepath.Join(state, "shares.ini"), []byte("[\"media\"]\n[\"appdata\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	live := &Live{
		stateDir:          state,
		shareUsage:        map[string]shareUsageEntry{"media": {Used: 1024, Updated: now}},
		shareUsageStarted: map[string]time.Time{"appdata": now},
		shareUsageTrigger: map[string]string{"appdata": "manual"},
		shareUsagePID:     map[string]int{"appdata": 4242},
		shareUsageErrors:  map[string]string{},
	}
	tasks, err := live.Tasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Name != "appdata" || tasks[0].Status != "running" {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
	if tasks[0].PID != 4242 || tasks[0].Path != "/mnt/user/appdata" || tasks[0].Command == "" {
		t.Fatalf("task is missing context: %+v", tasks[0])
	}
}

func TestTasksShowsTrivyImageScans(t *testing.T) {
	now := time.Now()
	finished := now
	live := &Live{
		shareUsage:        map[string]shareUsageEntry{},
		shareUsageStarted: map[string]time.Time{},
		shareUsageErrors:  map[string]string{},
		trivyScans: func() []model.TrivyScan {
			return []model.TrivyScan{
				{Image: "linuxserver/jackett:latest", Status: "running", StartedAt: now, Running: true, PID: 5150},
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
	if len(tasks) != 1 {
		t.Fatalf("only running tasks should remain: %+v", tasks)
	}
	running, ok := byName["linuxserver/jackett:latest"]
	if !ok || running.Status != "running" || running.Category != "security" || running.PID != 5150 {
		t.Fatalf("running scan is missing: %+v", tasks)
	}
	if _, ok := byName["bitnami/mongodb:latest"]; ok {
		t.Fatalf("queued scans have no process yet: %+v", tasks)
	}
	if _, ok := byName["redis:7"]; ok {
		t.Fatalf("finished scans must leave Tasks: %+v", tasks)
	}
	if running.Command != "trivy image linuxserver/jackett:latest" {
		t.Fatalf("unexpected command: %q", running.Command)
	}
}

func TestTasksAreSortedByLatestActivity(t *testing.T) {
	now := time.Now()
	live := &Live{
		shareUsage:        map[string]shareUsageEntry{},
		shareUsageStarted: map[string]time.Time{},
		shareUsageErrors:  map[string]string{},
		trivyScans: func() []model.TrivyScan {
			return []model.TrivyScan{
				{Image: "old:1", Status: "running", StartedAt: now.Add(-10 * time.Minute)},
				{Image: "new:1", Status: "running", StartedAt: now},
			}
		},
	}
	tasks, err := live.Tasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
	if tasks[0].Name != "new:1" || tasks[1].Name != "old:1" {
		t.Fatalf("tasks are not sorted by latest activity: %+v", tasks)
	}
}

func TestTaskLogsReportsFinishedWork(t *testing.T) {
	now := time.Now()
	live := &Live{
		shareUsage: map[string]shareUsageEntry{
			"media": {Used: 2048, Updated: now.Add(-3 * time.Minute), Trigger: "automatic", Duration: 12},
		},
		shareUsageErrors: map[string]string{},
		moverHistory: []model.MoverRun{
			{Started: now.Add(-2 * time.Hour), Finished: now.Add(-90 * time.Minute), Duration: 1800, Moved: 4096, Status: "success"},
		},
		balanceHistory: []model.BalanceJob{
			{Source: "/mnt/disk1", State: "completed", Started: now.Add(-20 * time.Minute), Finished: now.Add(-10 * time.Minute), DoneBytes: 8192},
		},
		trivyScanHistory: func() []model.TrivyScan {
			finished := now.Add(-time.Minute)
			return []model.TrivyScan{
				{Image: "redis:7", Status: "completed", StartedAt: now.Add(-2 * time.Minute), FinishedAt: &finished},
			}
		},
	}
	logs := live.taskLogs(10)
	if len(logs) != 4 {
		t.Fatalf("unexpected logs: %+v", logs)
	}
	if !strings.Contains(logs[0].Message, "trivy scan") || logs[0].Source != "tasks" {
		t.Fatalf("newest log should be the Trivy scan: %+v", logs[0])
	}
	if !strings.Contains(logs[1].Message, "share usage scan") ||
		!strings.Contains(logs[2].Message, "balance") ||
		!strings.Contains(logs[3].Message, "mover") {
		t.Fatalf("logs are not sorted newest first: %+v", logs)
	}
}
