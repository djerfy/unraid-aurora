package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"aurora/internal/model"
)

const (
	unraidAPIBinary = "/usr/local/bin/unraid-api"
	unraidAPIRc     = "/etc/rc.d/rc.unraid-api"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)

// UnraidAPIStatus exposes the official unraid-api process, the same component
// shown by the Unraid WebGUI.
func (l *Live) UnraidAPIStatus(ctx context.Context) (model.UnraidAPIStatus, error) {
	status := model.UnraidAPIStatus{State: "not-installed"}
	binary := unraidAPIExecutable()
	if binary == "" {
		return status, nil
	}
	status.Installed = true
	status.State = "unknown"

	if version := l.cachedUnraidAPIVersion(ctx, binary); version != "" {
		status.Version = version
	}
	output, err := l.runUnraidAPI(ctx, 20*time.Second, "status")
	status.Raw = strings.TrimSpace(output)
	if err == nil {
		status.Running, status.State = interpretUnraidAPIStatus(output)
	} else if running, state := unraidAPIProcessState(ctx); running {
		status.Running, status.State = running, state
		if status.Raw == "" {
			status.Raw = strings.TrimSpace(output)
		}
	} else {
		status.State = "stopped"
	}
	status.Config = l.cachedUnraidAPIConfig(ctx)
	return status, nil
}

// UnraidAPIAction starts, stops or restarts the service through the official
// rc script, falling back to the CLI when the script is absent.
func (l *Live) UnraidAPIAction(ctx context.Context, action string) (string, error) {
	switch action {
	case "start", "stop", "restart":
	default:
		return "", errors.New("unsupported Unraid API action")
	}
	if binary := unraidAPIExecutable(); binary == "" {
		return "", errors.New("the Unraid API is not installed")
	}
	if info, err := os.Stat(unraidAPIRc); err == nil && !info.IsDir() {
		runCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		output, err := exec.CommandContext(runCtx, unraidAPIRc, action).CombinedOutput()
		if err != nil {
			return strings.TrimSpace(string(output)), fmt.Errorf("unraid-api %s failed: %w", action, err)
		}
		return strings.TrimSpace(ansiPattern.ReplaceAllString(string(output), "")), nil
	}
	return l.runUnraidAPI(ctx, 2*time.Minute, action)
}

// UnraidAPILogs tails the most recent API or PM2 log file.
func (l *Live) UnraidAPILogs(ctx context.Context, lines int) ([]model.LogLine, error) {
	if lines <= 0 || lines > 2000 {
		lines = 200
	}
	path := newestLogFile([]string{
		"/var/log/unraid-api/*.log",
		"/var/log/.pm2/logs/unraid-api-*.log",
	})
	if path == "" {
		return nil, errors.New("no Unraid API log file was found")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	collected := make([]string, 0, lines)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 512*1024)
	for scanner.Scan() {
		collected = append(collected, scanner.Text())
		if len(collected) > lines {
			collected = collected[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	out := make([]model.LogLine, 0, len(collected))
	for _, raw := range collected {
		line := strings.TrimRight(ansiPattern.ReplaceAllString(raw, ""), "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		entry := model.LogLine{Time: time.Now(), Level: logLevel(line), Source: filepath.Base(path), Message: line}
		if stamp, rest, ok := splitLogTimestamp(line); ok {
			entry.Time = stamp
			entry.Message = rest
		}
		out = append(out, entry)
	}
	_ = ctx
	return out, nil
}

func (l *Live) runUnraidAPI(ctx context.Context, timeout time.Duration, args ...string) (string, error) {
	binary := unraidAPIExecutable()
	if binary == "" {
		return "", errors.New("the Unraid API is not installed")
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(runCtx, binary, args...)
	output, err := command.CombinedOutput()
	text := strings.TrimSpace(ansiPattern.ReplaceAllString(string(output), ""))
	if err != nil {
		return text, err
	}
	return text, nil
}

func (l *Live) cachedUnraidAPIVersion(ctx context.Context, binary string) string {
	l.apiMu.Lock()
	defer l.apiMu.Unlock()
	if l.apiVersion != "" && time.Since(l.apiVersionAt) < 10*time.Minute {
		return l.apiVersion
	}
	if version := l.unraidAPIVersion(ctx, binary); version != "" {
		l.apiVersion = version
		l.apiVersionAt = time.Now()
	}
	return l.apiVersion
}

func (l *Live) cachedUnraidAPIConfig(ctx context.Context) string {
	l.apiMu.Lock()
	defer l.apiMu.Unlock()
	if l.apiConfig != "" && time.Since(l.apiConfigAt) < 5*time.Minute {
		return l.apiConfig
	}
	config, err := l.runUnraidAPI(ctx, 10*time.Second, "config")
	if err != nil {
		return l.apiConfig
	}
	l.apiConfig = strings.TrimSpace(ansiPattern.ReplaceAllString(config, ""))
	l.apiConfigAt = time.Now()
	return l.apiConfig
}

func (l *Live) unraidAPIVersion(ctx context.Context, binary string) string {
	runCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(runCtx, binary, "version", "--json").Output(); err == nil {
		var payload struct {
			Version  string `json:"version"`
			Combined string `json:"combined"`
		}
		if json.Unmarshal(output, &payload) == nil {
			if payload.Combined != "" {
				return payload.Combined
			}
			if payload.Version != "" {
				return payload.Version
			}
		}
	}
	if output, err := exec.CommandContext(runCtx, binary, "version").Output(); err == nil {
		text := strings.TrimSpace(ansiPattern.ReplaceAllString(string(output), ""))
		if index := strings.LastIndex(text, "v"); index >= 0 {
			return strings.TrimSpace(text[index+1:])
		}
		return text
	}
	return ""
}

func unraidAPIExecutable() string {
	if info, err := os.Stat(unraidAPIBinary); err == nil && !info.IsDir() {
		return unraidAPIBinary
	}
	return ""
}

func interpretUnraidAPIStatus(output string) (bool, string) {
	lower := strings.ToLower(output)
	if strings.Contains(lower, "online") {
		return true, "online"
	}
	for _, state := range []string{"errored", "stopped", "launching", "stopping"} {
		if strings.Contains(lower, state) {
			return false, state
		}
	}
	return false, "unknown"
}

func unraidAPIProcessState(ctx context.Context) (bool, string) {
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(runCtx, "pgrep", "-f", "unraid-api").Output()
	if err == nil && len(strings.TrimSpace(string(output))) > 0 {
		return true, "online"
	}
	return false, "stopped"
}

func newestLogFile(patterns []string) string {
	type candidate struct {
		path    string
		modTime time.Time
	}
	found := make([]candidate, 0)
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || info.IsDir() || info.Size() == 0 {
				continue
			}
			found = append(found, candidate{path: match, modTime: info.ModTime()})
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].modTime.After(found[j].modTime) })
	if len(found) == 0 {
		return ""
	}
	return found[0].path
}

func logLevel(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "error") || strings.Contains(lower, "fatal"):
		return "error"
	case strings.Contains(lower, "warn"):
		return "warn"
	default:
		return "info"
	}
}

func splitLogTimestamp(line string) (time.Time, string, bool) {
	fields := strings.SplitN(line, " ", 2)
	if len(fields) != 2 {
		return time.Time{}, line, false
	}
	stamp := strings.TrimRight(fields[0], ":")
	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000Z0700", "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, stamp); err == nil {
			return parsed, fields[1], true
		}
	}
	return time.Time{}, line, false
}
