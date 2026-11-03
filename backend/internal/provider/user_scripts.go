package provider

import (
	"context"
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

const userScriptsRoot = "/boot/config/plugins/user.scripts/scripts"

var userScriptIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

func (l *Live) UserScripts(ctx context.Context) ([]model.UserScript, error) {
	entries, err := os.ReadDir(userScriptsRoot)
	if err != nil {
		return nil, err
	}
	var out []model.UserScript
	for _, entry := range entries {
		if !entry.IsDir() || !userScriptIDPattern.MatchString(entry.Name()) {
			continue
		}
		dir := filepath.Join(userScriptsRoot, entry.Name())
		script := filepath.Join(dir, "script")
		if _, err := os.Stat(script); err != nil {
			continue
		}
		name := strings.TrimSpace(readTrim(filepath.Join(dir, "name")))
		if name == "" {
			name = entry.Name()
		}
		out = append(out, model.UserScript{
			ID: entry.Name(), Name: name,
			Description: strings.TrimSpace(readTrim(filepath.Join(dir, "description"))),
			Schedule:    strings.TrimSpace(readTrim(filepath.Join(dir, "schedule"))),
		})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

func (l *Live) UserScript(ctx context.Context, id string) (string, error) {
	path, err := userScriptPath(id)
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
	path, err := userScriptPath(id)
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

func userScriptPath(id string) (string, error) {
	id = strings.TrimSpace(id)
	if !userScriptIDPattern.MatchString(id) {
		return "", errors.New("invalid script id")
	}
	return filepath.Join(userScriptsRoot, id, "script"), nil
}
