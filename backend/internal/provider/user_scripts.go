package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"aurora/internal/model"
)

var userScriptIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
var userScriptSchedulePattern = regexp.MustCompile(`^[0-9*/,\ -]+$`)

var userScriptFrequencies = map[string]bool{
	"disabled": true, "hourly": true, "daily": true, "weekly": true, "monthly": true,
	"start": true, "stop": true, "boot": true, "custom": true,
}

type userScriptScheduleEntry struct {
	Script    string `json:"script"`
	Frequency string `json:"frequency"`
	ID        string `json:"id"`
	Custom    string `json:"custom"`
}

type userScriptScheduleMap map[string]userScriptScheduleEntry

func (l *Live) UserScripts(ctx context.Context) ([]model.UserScript, error) {
	root := l.userScriptsRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	schedules := l.userScriptSchedules()
	var out []model.UserScript
	for _, entry := range entries {
		if !entry.IsDir() || !userScriptIDPattern.MatchString(entry.Name()) {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		script := filepath.Join(dir, "script")
		if _, err := os.Stat(script); err != nil {
			continue
		}
		name := strings.TrimSpace(readTrim(filepath.Join(dir, "name")))
		if name == "" {
			name = entry.Name()
		}
		content := ""
		if raw, err := os.ReadFile(script); err == nil {
			content = string(raw)
		}
		frequency, cron := "", ""
		if schedule, ok := schedules[script]; ok {
			frequency, cron = schedule.Frequency, schedule.Custom
		}
		display := ""
		switch frequency {
		case "", "disabled":
		case "custom":
			display = cron
		default:
			display = frequency
		}
		out = append(out, model.UserScript{
			ID: entry.Name(), Name: name,
			Description: strings.TrimSpace(readTrim(filepath.Join(dir, "description"))),
			Schedule:    display, Frequency: frequency, Cron: cron, Content: content,
		})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

func (l *Live) UserScript(ctx context.Context, id string) (string, error) {
	path, err := l.userScriptPath(id)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (l *Live) RunUserScript(ctx context.Context, id string) (string, error) {
	path, err := l.userScriptPath(id)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", path)
	cmd.Env = append(os.Environ(), "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("script exited with an error: %w", err)
	}
	return string(output), nil
}

// UpdateUserScript writes the script and stores its schedule in the same JSON
// file and cron fragment as the User Scripts plugin.
func (l *Live) UpdateUserScript(ctx context.Context, id, content, frequency, cron string) error {
	path, err := l.userScriptPath(id)
	if err != nil {
		return err
	}
	if len(content) > 1<<20 {
		return errors.New("script is larger than 1 MiB")
	}
	frequency = strings.ToLower(strings.TrimSpace(frequency))
	if frequency == "" {
		frequency = "disabled"
	}
	if !userScriptFrequencies[frequency] {
		return errors.New("unknown schedule frequency")
	}
	if frequency == "custom" {
		cron = strings.TrimSpace(cron)
		if !validCronExpression(cron) {
			return errors.New("custom schedule must be a five-field cron expression")
		}
	} else {
		cron = ""
	}
	// #nosec G306 -- user scripts must be executable.
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		return err
	}
	schedules := l.userScriptSchedules()
	schedules[path] = userScriptScheduleEntry{
		Script: path, Frequency: frequency, ID: id, Custom: cron,
	}
	if err := l.saveUserScriptSchedules(schedules); err != nil {
		return err
	}
	l.refreshUserScriptCron(ctx)
	return nil
}

func (l *Live) userScriptsRoot() string {
	if l.bootDir == "" {
		return "/boot/config/plugins/user.scripts/scripts"
	}
	return filepath.Join(l.bootDir, "config", "plugins", "user.scripts", "scripts")
}

func (l *Live) userScriptsConfigPath(name string) string {
	if l.bootDir == "" {
		return filepath.Join("/boot/config/plugins/user.scripts", name)
	}
	return filepath.Join(l.bootDir, "config", "plugins", "user.scripts", name)
}

func (l *Live) userScriptPath(id string) (string, error) {
	id = strings.TrimSpace(id)
	if !userScriptIDPattern.MatchString(id) {
		return "", errors.New("invalid script id")
	}
	return filepath.Join(l.userScriptsRoot(), id, "script"), nil
}

func (l *Live) userScriptSchedules() userScriptScheduleMap {
	schedules := userScriptScheduleMap{}
	raw, err := os.ReadFile(l.userScriptsConfigPath("schedule.json"))
	if err != nil {
		return schedules
	}
	if err := json.Unmarshal(raw, &schedules); err != nil {
		slog.Warn("parse user scripts schedule", "error", err)
		return userScriptScheduleMap{}
	}
	return schedules
}

func (l *Live) saveUserScriptSchedules(schedules userScriptScheduleMap) error {
	raw, err := json.MarshalIndent(schedules, "", "  ")
	if err != nil {
		return err
	}
	configPath := l.userScriptsConfigPath("schedule.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o750); err != nil {
		return err
	}
	if err := writeFileAtomic(configPath, raw, 0o644); err != nil {
		return err
	}
	// The plugin reads the runtime copy first, so keep it in sync when present.
	runtimePath := "/tmp/user.scripts/schedule.json"
	if info, err := os.Stat(filepath.Dir(runtimePath)); err == nil && info.IsDir() {
		_ = os.WriteFile(runtimePath, raw, 0o600)
	}
	return l.writeUserScriptCron(schedules)
}

func (l *Live) writeUserScriptCron(schedules userScriptScheduleMap) error {
	keys := make([]string, 0, len(schedules))
	for key := range schedules {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		entry := schedules[key]
		if entry.Frequency != "custom" || strings.TrimSpace(entry.Custom) == "" {
			continue
		}
		builder.WriteString(strings.TrimSpace(entry.Custom))
		builder.WriteString(" /usr/local/emhttp/plugins/user.scripts/startCustom.php ")
		builder.WriteString(entry.Script)
		builder.WriteString(" > /dev/null 2>&1\n")
	}
	path := l.userScriptsConfigPath("customSchedule.cron")
	if builder.Len() == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	content := "# Generated cron schedule for user.scripts\n" + builder.String()
	return writeFileAtomic(path, []byte(content), 0o644)
}

// refreshUserScriptCron asks Unraid to rebuild /etc/cron.d from the plugin
// fragment. A missing helper is fine: the file is applied at the next refresh.
func (l *Live) refreshUserScriptCron(ctx context.Context) {
	helper := "/usr/local/sbin/update_cron"
	if info, err := os.Stat(helper); err != nil || info.IsDir() {
		return
	}
	runCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if output, err := runCronHelper(runCtx, helper); err != nil {
		slog.Warn("refresh user scripts cron", "error", err, "output", strings.TrimSpace(string(output)))
	}
}

func validCronExpression(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 || !userScriptSchedulePattern.MatchString(value) {
		return false
	}
	return len(strings.Fields(value)) == 5
}
