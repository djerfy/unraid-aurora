package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurora/internal/model"
)

func (l *Live) Mover(ctx context.Context) (model.MoverStatus, error) {
	status, err := l.readMoverStatus(ctx)
	if err != nil {
		return status, err
	}
	status.Phase = moverPhase(status.Running, status.Action, status.Total, status.Remain)
	l.moverMu.Lock()
	if status.Running {
		l.moverLast = status
		status.Files = append([]string(nil), l.moverFiles...)
	}
	status.History = append([]model.MoverRun(nil), l.moverHistory...)
	if status.History == nil {
		status.History = []model.MoverRun{}
	}
	l.moverMu.Unlock()
	return status, nil
}

func (l *Live) readMoverStatus(ctx context.Context) (model.MoverStatus, error) {
	status := model.MoverStatus{}
	status.Running = moverRunning(ctx)
	path := filepath.Join(l.stateDir, "mover.ini")
	info, err := os.Stat(path)
	if err != nil {
		return status, nil
	}
	status.Updated = info.ModTime()
	ini, err := parseINI(path)
	if err != nil {
		return status, nil
	}
	get := func(key string) string { return ini.get("", key) }
	status.Total = uint64(parseInt(get("TotalSize")))
	status.Remain = uint64(parseInt(get("RemainSize")))
	status.Throughput = float64(parseInt(get("Throughput")))
	status.ETA = int64(parseInt(get("ETA")))
	status.ShareTotal = uint64(parseInt(get("ShareTotal")))
	status.ShareRemain = uint64(parseInt(get("ShareRemain")))
	status.ShareThroughput = float64(parseInt(get("ShareThroughput")))
	status.ShareETA = int64(parseInt(get("ShareETA")))
	status.File = get("File")
	status.Action = get("Action")
	status.FilePercent = parseFloat(get("Filepercent"))
	status.PID = parseInt(get("MoverPid"))
	if status.Total > 0 {
		moved := status.Total - status.Remain
		status.Progress = round1(float64(moved) / float64(status.Total) * 100)
	}
	return status, nil
}

func moverPhase(running bool, action string, total, remain uint64) string {
	if !running {
		return "idle"
	}
	lower := strings.ToLower(action)
	switch {
	case strings.Contains(lower, "stop"):
		return "stopping"
	case strings.Contains(lower, "calculating"):
		return "calculating"
	case strings.Contains(lower, "calculated"):
		return "moving"
	case total == 0:
		return "scanning"
	case remain == 0:
		return "finishing"
	default:
		return "moving"
	}
}

func (l *Live) watchMover() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-l.moverStop:
			return
		case <-ticker.C:
			status, _ := l.readMoverStatus(context.Background())
			l.moverMu.Lock()
			if status.Running {
				if !l.moverActive {
					l.moverActive = true
					l.moverStarted = time.Now()
					l.moverStopped = false
					l.moverFiles = nil
					l.moverSeen = map[string]bool{}
				}
				l.trackMoverFile(status)
				l.moverLast = status
			} else if l.moverActive {
				last := l.moverLast
				moved := uint64(0)
				if last.Total > last.Remain {
					moved = last.Total - last.Remain
				}
				finished := time.Now()
				run := model.MoverRun{
					Started: l.moverStarted, Finished: finished,
					Duration: int64(finished.Sub(l.moverStarted).Seconds()),
					Total:    last.Total, Moved: moved,
				}
				if l.moverStopped {
					run.Status = "stopped"
				} else {
					run.Status = "completed"
				}
				l.moverHistory = append([]model.MoverRun{run}, l.moverHistory...)
				if len(l.moverHistory) > 20 {
					l.moverHistory = l.moverHistory[:20]
				}
				l.moverActive = false
				l.moverStopped = false
				l.moverLast = model.MoverStatus{}
				l.moverFiles = nil
				l.moverSeen = map[string]bool{}
				_ = l.saveMoverHistory()
			}
			l.moverMu.Unlock()
		}
	}
}

// trackMoverFile records each file announced by mover.ini while the mover is
// relocating data. mover.ini changes faster than the poll interval for tiny
// files, so the list is a best-effort record of the run.
func (l *Live) trackMoverFile(status model.MoverStatus) {
	file := strings.TrimSpace(status.File)
	if file == "" || l.moverSeen[file] {
		return
	}
	action := strings.ToLower(status.Action)
	if !strings.Contains(action, "moving") && !strings.Contains(action, "copying") && !strings.Contains(action, "cleanup") {
		return
	}
	l.moverSeen[file] = true
	if len(l.moverFiles) < 1000 {
		l.moverFiles = append(l.moverFiles, file)
	}
}

func (l *Live) loadMoverHistory() {
	if l.dataDir == "" {
		return
	}
	raw, err := os.ReadFile(filepath.Join(l.dataDir, "mover-history.json"))
	if err != nil {
		return
	}
	var history []model.MoverRun
	if json.Unmarshal(raw, &history) == nil && len(history) > 20 {
		history = history[:20]
	}
	l.moverHistory = history
}

func (l *Live) saveMoverHistory() error {
	if l.dataDir == "" {
		return nil
	}
	if err := os.MkdirAll(l.dataDir, 0o750); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(l.moverHistory, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(l.dataDir, "mover-history.json"), raw, 0o600)
}

func (l *Live) StartMover(ctx context.Context) error {
	if moverRunning(ctx) {
		return fmt.Errorf("mover is already running")
	}
	script := moverScript()
	if script == "" {
		return fmt.Errorf("mover script not found")
	}
	// Use a detached command: the request context is cancelled as soon as the
	// response is sent, but the mover must keep running.
	cmd := exec.Command(script, "start")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start mover: %w", err)
	}
	l.moverMu.Lock()
	l.moverActive = true
	l.moverStarted = time.Now()
	l.moverStopped = false
	l.moverLast = model.MoverStatus{}
	l.moverFiles = nil
	l.moverSeen = map[string]bool{}
	l.moverMu.Unlock()
	// Do not Wait: the mover runs for minutes or hours and must survive the
	// HTTP request. The status endpoint follows it through mover.ini.
	go func() { _ = cmd.Wait() }()
	return nil
}

func parseFloat(value string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return v
}

func (l *Live) StopMover(ctx context.Context) error {
	l.moverMu.Lock()
	l.moverStopped = true
	l.moverMu.Unlock()
	script := moverScript()
	if script == "" {
		return fmt.Errorf("mover script not found")
	}
	stopCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(stopCtx, script, "stop").CombinedOutput(); err != nil {
		return fmt.Errorf("stop mover: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func moverRunning(ctx context.Context) bool {
	script := moverScript()
	if script == "" {
		return false
	}
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return exec.CommandContext(checkCtx, script, "status").Run() == nil
}

func moverScript() string {
	for _, path := range []string{
		"/usr/local/sbin/mover",
		"/usr/local/emhttp/plugins/dynamix/scripts/mover",
	} {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}
