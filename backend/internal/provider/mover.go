package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurora/internal/model"
)

const moverHistoryLimit = 10

// moverMountPrefix is a variable so tests can exercise the tracker against a
// temporary directory instead of a real /mnt tree.
var moverMountPrefix = "/mnt/"

func (l *Live) Mover(ctx context.Context) (model.MoverStatus, error) {
	status, err := l.readMoverStatus(ctx)
	if err != nil {
		return status, err
	}
	status.Phase = moverPhase(status.Running, status.Action, status.Total, status.Remain)
	l.moverMu.Lock()
	if status.Running {
		// The watcher already tails the mover output, file lists and syslog.
		// Only enrich from its cache here so the HTTP poll stays fast on large
		// shares.
		l.enrichMoverStatusLocked(&status)
		l.moverLast = status
		status.Files = append([]string(nil), l.moverFiles...)
		if status.File == "" && len(l.moverFiles) > 0 {
			status.File = l.moverFiles[len(l.moverFiles)-1]
		}
	}
	if status.ProgressSource == "" {
		status.ProgressSource = "none"
	}
	if status.Files == nil {
		status.Files = []string{}
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
	status.ProgressEnabled = l.moverProgressEnabled()
	path, exists := l.moverStatusPath()
	if !exists {
		return status, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return status, nil
	}
	status.Updated = info.ModTime()
	ini, err := parseINI(path)
	if err != nil {
		return status, nil
	}
	status.ProgressSource = "mover.ini"
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
	status.Progress = progressPercent(status.Total, status.Remain)
	return status, nil
}

// moverStatusPath returns the first existing mover status file. Unraid writes
// to /usr/local/emhttp/state (a symlink to /var/local/emhttp on most installs),
// but the symlink can be missing after a failed boot.
func (l *Live) moverStatusPath() (string, bool) {
	candidates := []string{}
	if l.stateDir != "" {
		candidates = append(candidates, filepath.Join(l.stateDir, "mover.ini"))
	}
	candidates = append(candidates, "/usr/local/emhttp/state/mover.ini")
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, true
		}
	}
	return candidates[len(candidates)-1], false
}

// moverProgressEnabled reports whether Unraid's mover writes byte progress.
// share.cfg drives the mover itself; var.ini is the source of truth for the
// WebGUI on newer releases, so use it when share.cfg has no explicit value.
func (l *Live) moverProgressEnabled() bool {
	if ini, err := parseINI(l.schedulerShareConfigPath()); err == nil {
		if value, ok := ini.section("")["shareMoverProgress"]; ok {
			return strings.EqualFold(value, "yes")
		}
	}
	if l.stateDir == "" {
		return false
	}
	if ini, err := parseINI(filepath.Join(l.stateDir, "var.ini")); err == nil {
		return strings.EqualFold(ini.get("", "shareMoverProgress"), "yes")
	}
	return false
}

func progressPercent(total, remain uint64) float64 {
	if total == 0 || remain >= total {
		return 0
	}
	return round1(float64(total-remain) / float64(total) * 100)
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

func (l *Live) resetMoverTrackingLocked() {
	l.moverActive = true
	l.moverStarted = time.Now()
	l.moverStopped = false
	l.moverLast = model.MoverStatus{}
	l.moverFiles = nil
	l.moverSeen = map[string]bool{}
	l.moverCandidates = map[string]int64{}
	l.moverScannedLists = map[string]int64{}
	l.moverCandidateTotal = 0
	l.moverCandidateRemain = 0
	l.moverSyslogOffset = fileSize("/var/log/syslog")
	l.moverOutput = ""
	l.moverOutputOffset = 0
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return 0
	}
	return info.Size()
}

// scanMoverFileListsLocked reads the per-share mover-files.* lists created by
// the official mover script. It gives a byte total and move candidates even
// when shareMoverProgress is disabled.
func (l *Live) scanMoverFileListsLocked() {
	if l.moverCandidates == nil {
		l.moverCandidates = map[string]int64{}
	}
	if l.moverScannedLists == nil {
		l.moverScannedLists = map[string]int64{}
	}
	lists, _ := filepath.Glob(filepath.Join(os.TempDir(), "mover-files.*"))
	if os.TempDir() != "/tmp" {
		extra, _ := filepath.Glob("/tmp/mover-files.*")
		lists = append(lists, extra...)
	}
	for _, path := range lists {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		if previous, ok := l.moverScannedLists[path]; ok && previous == info.Size() {
			continue
		}
		l.moverScannedLists[path] = info.Size()
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			candidate := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(candidate, moverMountPrefix) {
				continue
			}
			if len(l.moverCandidates) >= 50_000 {
				break
			}
			if _, exists := l.moverCandidates[candidate]; exists {
				continue
			}
			info, err := os.Stat(candidate)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			size := info.Size()
			if size < 0 {
				size = 0
			}
			l.moverCandidates[candidate] = size
			l.moverCandidateTotal += uint64(size)
			l.moverCandidateRemain += uint64(size)
		}
		_ = file.Close()
	}
}

// scanMoverTuningListsLocked reads CA Mover Tuning's filtered file lists.
// The plugin does not create /tmp/mover-files.* and does not write mover.ini
// when live progress is disabled, but its filtered list always contains the
// file size and path.
func (l *Live) scanMoverTuningListsLocked() {
	if l.moverCandidates == nil {
		l.moverCandidates = map[string]int64{}
	}
	if l.moverScannedLists == nil {
		l.moverScannedLists = map[string]int64{}
	}
	directories := []string{filepath.Join(os.TempDir(), "ca.mover.tuning")}
	if os.TempDir() != "/tmp" {
		directories = append(directories, "/tmp/ca.mover.tuning")
	}
	for _, directory := range directories {
		lists, _ := filepath.Glob(filepath.Join(directory, "Filtered_files_*.list"))
		for _, path := range lists {
			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				continue
			}
			if previous, ok := l.moverScannedLists[path]; ok && previous == info.Size() {
				continue
			}
			l.moverScannedLists[path] = info.Size()
			file, err := os.Open(path)
			if err != nil {
				continue
			}
			scanner := bufio.NewScanner(file)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for scanner.Scan() {
				fields := strings.Split(scanner.Text(), "|")
				if len(fields) < 13 || strings.TrimSpace(fields[11]) != "f" {
					continue
				}
				candidate := strings.TrimSpace(fields[12])
				if !strings.HasPrefix(candidate, moverMountPrefix) {
					continue
				}
				if _, exists := l.moverCandidates[candidate]; exists {
					continue
				}
				if len(l.moverCandidates) >= 50_000 {
					break
				}
				size := parseInt(fields[7])
				if size < 0 {
					size = 0
				}
				l.moverCandidates[candidate] = int64(size)
				l.moverCandidateTotal += uint64(size)
				l.moverCandidateRemain += uint64(size)
			}
			_ = file.Close()
		}
	}
}

func (l *Live) updateMoverCandidatesLocked(limit int) {
	checked := 0
	for path, size := range l.moverCandidates {
		if limit > 0 && checked >= limit {
			break
		}
		checked++
		if _, err := os.Stat(path); err == nil {
			continue
		}
		delete(l.moverCandidates, path)
		if uint64(size) <= l.moverCandidateRemain {
			l.moverCandidateRemain -= uint64(size)
		} else {
			l.moverCandidateRemain = 0
		}
		l.addMoverFileLocked(path)
	}
}

func (l *Live) addMoverFileLocked(path string) {
	path = strings.TrimSpace(path)
	if path == "" || !strings.HasPrefix(path, moverMountPrefix) || l.moverSeen[path] {
		return
	}
	l.moverSeen[path] = true
	if len(l.moverFiles) < 1000 {
		l.moverFiles = append(l.moverFiles, path)
	}
}

func (l *Live) tailMoverSyslogLocked() {
	const path = "/var/log/syslog"
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return
	}
	if l.moverSyslogOffset <= 0 || info.Size() < l.moverSyslogOffset {
		l.moverSyslogOffset = 0
	}
	if info.Size() == l.moverSyslogOffset {
		return
	}
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	if _, err := file.Seek(l.moverSyslogOffset, io.SeekStart); err != nil {
		return
	}
	raw, err := io.ReadAll(io.LimitReader(file, 2<<20))
	if err != nil {
		return
	}
	l.moverSyslogOffset += int64(len(raw))
	for _, line := range strings.Split(string(raw), "\n") {
		if file, ok := moverFileFromLine(line); ok {
			l.addMoverFileLocked(file)
		}
	}
}

func (l *Live) enrichMoverStatusLocked(status *model.MoverStatus) {
	if status.Total == 0 && l.moverCandidateTotal > 0 {
		status.Total = l.moverCandidateTotal
		status.Remain = l.moverCandidateRemain
		status.ProgressSource = "file-list"
	}
	if status.Total > 0 {
		status.Progress = progressPercent(status.Total, status.Remain)
		if status.Remain == 0 || status.Progress >= 99.9 {
			status.Progress = 100
		}
	}
	if status.Throughput <= 0 && status.Total > status.Remain && !l.moverStarted.IsZero() {
		elapsed := time.Since(l.moverStarted).Seconds()
		if elapsed > 0 {
			status.Throughput = float64(status.Total-status.Remain) / elapsed
		}
	}
	if status.ETA <= 0 && status.Throughput > 0 && status.Remain > 0 {
		status.ETA = int64(float64(status.Remain) / status.Throughput)
	}
}

func (l *Live) watchMover() {
	// A short run can complete between two polls; half a second keeps the
	// mover card responsive without spawning a status process each tick.
	ticker := time.NewTicker(500 * time.Millisecond)
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
					l.resetMoverTrackingLocked()
				}
				l.tailMoverOutputLocked()
				l.scanMoverFileListsLocked()
				l.scanMoverTuningListsLocked()
				l.updateMoverCandidatesLocked(2000)
				l.tailMoverSyslogLocked()
				l.enrichMoverStatusLocked(&status)
				if status.ProgressSource == "" {
					status.ProgressSource = "none"
				}
				l.trackMoverFile(status)
				l.moverLast = status
			} else if l.moverActive {
				l.tailMoverOutputLocked()
				l.scanMoverFileListsLocked()
				l.scanMoverTuningListsLocked()
				l.updateMoverCandidatesLocked(0)
				l.tailMoverSyslogLocked()
				l.enrichMoverStatusLocked(&l.moverLast)
				last := l.moverLast
				total := last.Total
				if total == 0 {
					total = last.ShareTotal
				}
				if total == 0 {
					total = l.moverCandidateTotal
				}
				moved := uint64(0)
				if total > last.Remain {
					moved = total - last.Remain
				}
				if moved == 0 && l.moverCandidateTotal > l.moverCandidateRemain {
					moved = l.moverCandidateTotal - l.moverCandidateRemain
				}
				if total == 0 && moved > 0 {
					total = moved
				}
				finished := time.Now()
				run := model.MoverRun{
					Started: l.moverStarted, Finished: finished,
					Duration: int64(finished.Sub(l.moverStarted).Seconds()),
					Total:    total, Moved: moved,
				}
				if l.moverStopped {
					run.Status = "stopped"
				} else {
					run.Status = "completed"
				}
				run.Files = append([]string{}, l.moverFiles...)
				l.moverHistory = append([]model.MoverRun{run}, l.moverHistory...)
				if len(l.moverHistory) > moverHistoryLimit {
					l.moverHistory = l.moverHistory[:moverHistoryLimit]
				}
				l.moverActive = false
				l.moverStopped = false
				l.moverLast = model.MoverStatus{}
				l.moverFiles = nil
				l.moverSeen = map[string]bool{}
				l.moverCandidates = nil
				l.moverScannedLists = nil
				l.moverCandidateTotal = 0
				l.moverCandidateRemain = 0
				l.moverSyslogOffset = 0
				l.moverOutput = ""
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

func (l *Live) moverOutputPath() string {
	if l.dataDir == "" {
		return ""
	}
	return filepath.Join(l.dataDir, "mover-output.log")
}

// tailMoverOutputLocked parses the mover's own stdout so file-level progress
// works even when Unraid's optional mover progress (shareMoverProgress) is off.
func (l *Live) tailMoverOutputLocked() {
	if l.moverOutput == "" {
		return
	}
	info, err := os.Stat(l.moverOutput)
	if err != nil || info.IsDir() {
		return
	}
	if info.Size() < l.moverOutputOffset {
		l.moverOutputOffset = 0
	}
	if info.Size() == l.moverOutputOffset {
		return
	}
	file, err := os.Open(l.moverOutput)
	if err != nil {
		return
	}
	defer file.Close()
	if _, err := file.Seek(l.moverOutputOffset, io.SeekStart); err != nil {
		return
	}
	raw, err := io.ReadAll(io.LimitReader(file, 2<<20))
	if err != nil {
		return
	}
	l.moverOutputOffset += int64(len(raw))
	for _, line := range strings.Split(string(raw), "\n") {
		file, ok := moverFileFromLine(line)
		if !ok || l.moverSeen[file] {
			continue
		}
		l.moverSeen[file] = true
		if len(l.moverFiles) < 1000 {
			l.moverFiles = append(l.moverFiles, file)
		}
	}
}

func moverFileFromLine(line string) (string, bool) {
	line = strings.TrimSpace(line)
	// Syslog lines from the official helper look like
	// "move: move: /mnt/cache/file Success": the first prefix is the logger
	// tag, so the last match is the actual path.
	index := strings.LastIndex(line, "move: ")
	if index < 0 {
		return "", false
	}
	file := strings.TrimSpace(line[index+len("move: "):])
	for _, suffix := range []string{" Success", " File exists", " Not moved", " Failed", " failed"} {
		if strings.HasSuffix(file, suffix) {
			file = strings.TrimSpace(strings.TrimSuffix(file, suffix))
			break
		}
	}
	if file == "" || !strings.HasPrefix(file, moverMountPrefix) {
		return "", false
	}
	return file, true
}

func (l *Live) loadMoverHistory() {
	if l.configDir() == "" {
		return
	}
	raw, err := l.readConfigFile("mover-history.json")
	if err != nil {
		return
	}
	var history []model.MoverRun
	if json.Unmarshal(raw, &history) == nil && len(history) > moverHistoryLimit {
		history = history[:moverHistoryLimit]
	}
	l.moverHistory = history
}

func (l *Live) saveMoverHistory() error {
	if l.configDir() == "" {
		return nil
	}
	raw, err := json.MarshalIndent(l.moverHistory, "", "  ")
	if err != nil {
		return err
	}
	return l.saveConfigFile("mover-history.json", raw)
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
	outputPath := l.moverOutputPath()
	var output *os.File
	if outputPath != "" {
		_ = os.MkdirAll(filepath.Dir(outputPath), 0o750)
		if file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600); err == nil {
			output = file
			cmd.Stdout = output
			cmd.Stderr = output
		}
	}
	if err := cmd.Start(); err != nil {
		if output != nil {
			_ = output.Close()
		}
		return fmt.Errorf("start mover: %w", err)
	}
	if output != nil {
		_ = output.Close()
	}
	l.moverMu.Lock()
	l.resetMoverTrackingLocked()
	l.moverOutput = outputPath
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
	if pid := readMoverPID(); pid > 0 {
		if raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid)); err == nil {
			return strings.Contains(strings.ReplaceAll(string(raw), "\x00", " "), "mover")
		}
	}
	script := moverScript()
	if script == "" {
		return false
	}
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return exec.CommandContext(checkCtx, script, "status").Run() == nil
}

func readMoverPID() int {
	raw, err := os.ReadFile("/var/run/mover.pid")
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return 0
	}
	return pid
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
