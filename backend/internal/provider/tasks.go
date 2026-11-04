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
			path := "/mnt/user/" + name
			task := model.Task{
				ID: "share:" + name, Name: name, Category: "share-usage", Status: "queued",
				Description: "Share usage scan", Path: path, Command: "du -skx " + path,
				Trigger: "automatic", Progress: -1,
				Detail: "Waiting for the first scan (cache empty)",
			}
			entry, cached := l.shareUsage[name]
			if started, running := l.shareUsageStarted[name]; running {
				task.Status = "running"
				task.StartedAt = &started
				task.Trigger = l.shareUsageTrigger[name]
				task.Detail = "Scanning " + path
			} else if cached {
				updated := entry.Updated
				task.UpdatedAt = &updated
				task.FinishedAt = &updated
				task.Trigger = entry.Trigger
				task.Duration = entry.Duration
				task.Detail = fmt.Sprintf("%s used", formatBytesForTask(entry.Used))
				if now.Sub(entry.Updated) >= shareUsageTTL {
					task.Status = "queued"
					task.Detail += " · cache expired after 6 h"
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
			ID: "balance", Name: "Balance", Category: "balance", Status: "running",
			Description: "Disk balance job", Path: job.Source, Trigger: "manual",
			Detail: job.Phase, Progress: jobProgress(job),
		}
		if !job.Started.IsZero() {
			task.StartedAt = &job.Started
		}
		tasks = append(tasks, task)
	}
	if mover, err := l.Mover(ctx); err == nil && mover.Running {
		task := model.Task{
			ID: "mover", Name: "Mover", Category: "mover", Status: "running",
			Description: "Official Unraid mover run", Command: "/usr/local/sbin/mover start",
			Trigger: "scheduler or manual", Detail: mover.Action, Progress: mover.Progress,
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
			Description: "Trivy binary/database operation", Trigger: "manual or scheduled",
			Detail: trivy.Stage,
		}
		if trivy.Error != "" {
			task.Error = trivy.Error
		}
		tasks = append(tasks, task)
	}

	// Image scans do not change TrivyStatus, so they need their own entries.
	if l.trivyScans != nil {
		for _, scan := range l.trivyScans() {
			tasks = append(tasks, trivyScanTask(scan))
		}
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

func trivyScanTask(scan model.TrivyScan) model.Task {
	name := scan.Image
	if name == "" {
		name = scan.ImageID
	}
	status := scan.Status
	detail := "Waiting for the Trivy worker"
	switch status {
	case "running":
		detail = "Scanning image layers for vulnerabilities"
	case "completed":
		detail = "Scan complete"
	case "failed":
		detail = "Scan failed"
	case "":
		status = "queued"
	}
	started := scan.StartedAt
	task := model.Task{
		ID: "trivy-scan:" + name + ":" + started.UTC().Format(time.RFC3339), Name: name,
		Category: "security", Status: status,
		Description: "Trivy image vulnerability scan",
		Command:     "trivy image " + name,
		Trigger:     "automatic after pull or manual",
		Detail:      detail, Progress: -1,
	}
	if !started.IsZero() {
		task.StartedAt = &started
	}
	if scan.FinishedAt != nil {
		finished := *scan.FinishedAt
		task.FinishedAt = &finished
		task.UpdatedAt = &finished
		task.Duration = int64(finished.Sub(started).Seconds())
	}
	if status == "completed" {
		task.Progress = 100
	}
	if scan.Error != "" {
		task.Error = scan.Error
	}
	return task
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
