package provider

import (
	"context"
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
