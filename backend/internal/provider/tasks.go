package provider

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"aurora/internal/model"
)

// Tasks aggregates the work Aurora runs outside an HTTP request.
func (l *Live) Tasks(ctx context.Context) ([]model.Task, error) {
	tasks := []model.Task{}
	now := time.Now()

	if shares, err := parseINI(filepath.Join(l.stateDir, "shares.ini")); err == nil {
		l.shareUsageMu.Lock()
		for name := range shares {
			task := model.Task{
				ID: "share:" + name, Name: name, Category: "storage", Status: "queued",
				Detail: "Waiting for the first usage scan",
			}
			entry, cached := l.shareUsage[name]
			if started, running := l.shareUsageStarted[name]; running {
				task.Status = "running"
				task.StartedAt = &started
				task.Detail = "Scanning /mnt/user/" + name
				task.Progress = 0
			} else if cached {
				updated := entry.Updated
				task.UpdatedAt = &updated
				task.FinishedAt = &updated
				task.Detail = fmt.Sprintf("%s used", formatBytesForTask(entry.Used))
				if now.Sub(entry.Updated) >= shareUsageTTL {
					task.Status = "queued"
					task.Detail += " · cache expired"
				} else {
					task.Status = "completed"
					task.Progress = 100
				}
			}
			if message := l.shareUsageErrors[name]; message != "" {
				task.Status = "failed"
				task.Error = message
			}
			tasks = append(tasks, task)
		}
		l.shareUsageMu.Unlock()
	}

	if job, err := l.BalanceStatus(ctx); err == nil && (job.State == "scanning" || job.State == "running") {
		task := model.Task{
			ID: "balance", Name: "Balance", Category: "storage", Status: "running",
			Detail: job.Phase, Progress: jobProgress(job),
		}
		if !job.Started.IsZero() {
			task.StartedAt = &job.Started
		}
		tasks = append(tasks, task)
	}
	if mover, err := l.Mover(ctx); err == nil && mover.Running {
		task := model.Task{
			ID: "mover", Name: "Mover", Category: "storage", Status: "running",
			Detail: mover.Action, Progress: mover.Progress,
		}
		if !mover.Updated.IsZero() {
			updated := mover.Updated
			task.UpdatedAt = &updated
		}
		tasks = append(tasks, task)
	}
	if trivy, err := l.TrivyStatus(ctx); err == nil && (trivy.Installing || trivy.UpdatingDB) {
		task := model.Task{
			ID: "trivy", Name: "Trivy", Category: "security", Status: "running",
			Detail: trivy.Stage,
		}
		if trivy.Error != "" {
			task.Error = trivy.Error
		}
		tasks = append(tasks, task)
	}

	rank := map[string]int{"running": 0, "queued": 1, "failed": 2, "completed": 3}
	sort.Slice(tasks, func(i, j int) bool {
		if rank[tasks[i].Status] != rank[tasks[j].Status] {
			return rank[tasks[i].Status] < rank[tasks[j].Status]
		}
		return tasks[i].Name < tasks[j].Name
	})
	return tasks, nil
}

func jobProgress(job model.BalanceJob) float64 {
	if job.TotalBytes == 0 {
		return 0
	}
	return round1(float64(job.DoneBytes) / float64(job.TotalBytes) * 100)
}

func formatBytesForTask(value uint64) string {
	const unit = 1024
	if value < unit {
		return strconv.FormatUint(value, 10) + " B"
	}
	div, exp := uint64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(div), "KMGTPE"[exp])
}
