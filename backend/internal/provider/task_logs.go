package provider

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"aurora/internal/model"
)

// taskLogs rebuilds the completed background work from the histories each
// subsystem already keeps. Finished tasks leave the Tasks page; this is where
// they remain visible.
func (l *Live) taskLogs(lines int) []model.LogLine {
	entries := []model.LogLine{}
	appendEntry := func(at time.Time, level, message string) {
		if at.IsZero() {
			return
		}
		entries = append(entries, model.LogLine{Time: at, Level: level, Source: "tasks", Message: message})
	}

	l.shareUsageMu.Lock()
	for name, entry := range l.shareUsage {
		if entry.Updated.IsZero() {
			continue
		}
		level := "info"
		message := fmt.Sprintf("share usage scan · %s · %s · %s · %s",
			name, formatBytesForTask(entry.Used), durationLabel(entry.Duration), triggerLabel(entry.Trigger))
		if message2 := l.shareUsageErrors[name]; message2 != "" {
			level = "error"
			message += " · failed: " + message2
		}
		appendEntry(entry.Updated, level, message)
	}
	l.shareUsageMu.Unlock()

	l.balanceMu.Lock()
	for _, job := range l.balanceHistory {
		if job.Finished.IsZero() {
			continue
		}
		level := "info"
		if len(job.Errors) > 0 {
			level = "error"
		}
		message := fmt.Sprintf("balance · %s · %s · %s · %s",
			job.Source, job.State, formatBytesForTask(job.DoneBytes), durationBetween(job.Started, job.Finished))
		if len(job.Errors) > 0 {
			message += " · " + job.Errors[0]
		}
		appendEntry(job.Finished, level, message)
	}
	l.balanceMu.Unlock()

	l.moverMu.Lock()
	for _, run := range l.moverHistory {
		if run.Finished.IsZero() {
			continue
		}
		level := "info"
		if run.Status != "" && run.Status != "success" && run.Status != "completed" {
			level = "warn"
		}
		appendEntry(run.Finished, level, fmt.Sprintf("mover · %s · %s moved · %s",
			run.Status, formatBytesForTask(run.Moved), durationLabel(run.Duration)))
	}
	l.moverMu.Unlock()

	if l.trivyScanHistory != nil {
		for _, scan := range l.trivyScanHistory() {
			if scan.FinishedAt == nil {
				continue
			}
			name := scan.Image
			if name == "" {
				name = scan.ImageID
			}
			level := "info"
			message := fmt.Sprintf("trivy scan · %s · %s · %s",
				name, scan.Status, durationBetween(scan.StartedAt, *scan.FinishedAt))
			if scan.Error != "" {
				level = "error"
				message += " · " + scan.Error
			}
			appendEntry(*scan.FinishedAt, level, message)
		}
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Time.After(entries[j].Time) })
	if lines > 0 && len(entries) > lines {
		entries = entries[:lines]
	}
	return entries
}

func durationLabel(seconds int64) string {
	if seconds <= 0 {
		return "—"
	}
	return time.Duration(seconds * int64(time.Second)).String()
}

func durationBetween(start, end time.Time) string {
	if start.IsZero() || end.Before(start) {
		return "—"
	}
	return end.Sub(start).Round(time.Second).String()
}

func triggerLabel(trigger string) string {
	if trigger == "" {
		return "automatic"
	}
	return trigger
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
