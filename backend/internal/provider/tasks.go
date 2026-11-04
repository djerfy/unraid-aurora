package provider

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"time"

	"aurora/internal/model"
)

// Tasks aggregates the work Aurora currently runs outside an HTTP request.
// Finished work is reported by the "tasks" log source instead.
func (l *Live) Tasks(ctx context.Context) ([]model.Task, error) {
	tasks := []model.Task{}

	if shares, err := parseINI(filepath.Join(l.stateDir, "shares.ini")); err == nil {
		l.shareUsageMu.Lock()
		for name := range shares {
			started, running := l.shareUsageStarted[name]
			if !running {
				continue
			}
			path := "/mnt/user/" + name
			task := model.Task{
				ID: "share:" + name, Name: name, Category: "share-usage", Status: "running",
				Description: "Share usage scan", Path: path, Command: "du -skx " + path,
				Trigger: l.shareUsageTrigger[name], Detail: "Scanning " + path,
				Progress: -1, PID: l.shareUsagePID[name], StartedAt: &started,
			}
			tasks = append(tasks, task)
		}
		l.shareUsageMu.Unlock()
	}

	if job, err := l.BalanceStatus(ctx); err == nil && (job.State == "scanning" || job.State == "running") {
		task := model.Task{
			ID: "balance", Name: "Balance", Category: "balance", Status: "running",
			Description: "Disk balance job", Path: job.Source, Trigger: "manual",
			Detail: job.Phase, Progress: jobProgress(job), PID: os.Getpid(),
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
			PID: mover.PID,
		}
		if task.PID == 0 {
			task.PID = readMoverPID()
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
			Detail: trivy.Stage, PID: os.Getpid(),
		}
		if trivy.Error != "" {
			task.Error = trivy.Error
		}
		tasks = append(tasks, task)
	}

	// Image scans do not change TrivyStatus, so they need their own entries.
	if l.trivyScans != nil {
		for _, scan := range l.trivyScans() {
			// Queued scans have no process yet; they join the page when the
			// worker starts them.
			if scan.Status != "running" {
				continue
			}
			tasks = append(tasks, trivyScanTask(scan))
		}
	}

	sort.Slice(tasks, func(i, j int) bool {
		left, right := taskTimestamp(tasks[i]), taskTimestamp(tasks[j])
		if left.Equal(right) {
			return tasks[i].Name < tasks[j].Name
		}
		return left.After(right)
	})
	return tasks, nil
}

// taskTimestamp is the most recent activity for a task: its last update when
// known, otherwise when it started.
func taskTimestamp(task model.Task) time.Time {
	if task.UpdatedAt != nil {
		return *task.UpdatedAt
	}
	if task.StartedAt != nil {
		return *task.StartedAt
	}
	return time.Time{}
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
		Detail:      detail, Progress: -1, PID: scan.PID,
	}
	if !started.IsZero() {
		task.StartedAt = &started
	}
	return task
}

func jobProgress(job model.BalanceJob) float64 {
	if job.TotalBytes == 0 {
		return 0
	}
	return round1(float64(job.DoneBytes) / float64(job.TotalBytes) * 100)
}
