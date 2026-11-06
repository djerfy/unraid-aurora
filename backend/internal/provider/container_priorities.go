package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"sort"
	"strings"

	"aurora/internal/model"
)

const (
	containerPriorityFile    = "container-priorities.json"
	defaultContainerPriority = 50
)

type containerPriorityState struct {
	Version    int            `json:"version"`
	Priorities map[string]int `json:"priorities"`
}

func applyContainerPriorities(containers []model.Container, priorities map[string]int) {
	for index := range containers {
		value, ok := priorities[containers[index].Name]
		if !ok {
			value = defaultContainerPriority
		}
		containers[index].Priority = value
	}
}

func (l *Live) containerPriorityMap() map[string]int {
	raw, err := l.readConfigFile(containerPriorityFile)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("read container priorities", "error", err)
		}
		return map[string]int{}
	}
	var state containerPriorityState
	if err := json.Unmarshal(raw, &state); err != nil {
		slog.Warn("parse container priorities", "error", err)
		return map[string]int{}
	}
	out := make(map[string]int, len(state.Priorities))
	for name, value := range state.Priorities {
		name = strings.TrimSpace(name)
		if name == "" || !containerNamePattern.MatchString(name) || value < 0 || value > 100 {
			continue
		}
		// 50 is the default: keeping it out of the file keeps it compact.
		if value != defaultContainerPriority {
			out[name] = value
		}
	}
	return out
}

func (l *Live) ContainerPriorities(_ context.Context) (map[string]int, error) {
	return l.containerPriorityMap(), nil
}

func (l *Live) SaveContainerPriorities(_ context.Context, priorities map[string]int) (map[string]int, error) {
	if len(priorities) > 500 {
		return nil, errors.New("too many containers")
	}
	saved := l.containerPriorityMap()
	for name, value := range priorities {
		name = strings.TrimSpace(name)
		if name == "" || !containerNamePattern.MatchString(name) {
			return nil, fmt.Errorf("invalid container name %q", name)
		}
		if value < 0 || value > 100 {
			return nil, fmt.Errorf("priority for %s must be between 0 and 100", name)
		}
		if value == defaultContainerPriority {
			delete(saved, name)
			continue
		}
		saved[name] = value
	}
	raw, err := json.MarshalIndent(containerPriorityState{Version: 1, Priorities: saved}, "", "  ")
	if err != nil {
		return nil, err
	}
	path := l.configPath(containerPriorityFile)
	if path == "" {
		return nil, errors.New("configuration directory is unavailable")
	}
	if err := l.writeConfigFile("container.priorities", "Updated container startup priorities", path, raw); err != nil {
		return nil, err
	}
	if err := l.sortContainerAutostart(saved); err != nil {
		return nil, err
	}
	return saved, nil
}

// sortContainerAutostart keeps Unraid's boot order aligned with the priority
// values. Unraid reads this file top-to-bottom at array start.
func (l *Live) sortContainerAutostart(priorities map[string]int) error {
	entries, err := readAutostartFile(l.autostartPath())
	if err != nil {
		return fmt.Errorf("read container autostart: %w", err)
	}
	if len(entries) < 2 {
		return nil
	}
	sorted := sortAutostartEntries(entries, priorities)
	if slices.Equal(entries, sorted) {
		return nil
	}
	return l.writeConfigFile(
		"container.autostart.order",
		"Sorted container autostart by priority",
		l.autostartPath(),
		[]byte(strings.Join(sorted, "\n")+"\n"),
	)
}

func sortAutostartEntries(entries []string, priorities map[string]int) []string {
	sorted := append([]string(nil), entries...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return autostartPriority(sorted[i], priorities) < autostartPriority(sorted[j], priorities)
	})
	return sorted
}

func autostartPriority(entry string, priorities map[string]int) int {
	name, _, _ := strings.Cut(entry, ";")
	name = strings.TrimSpace(name)
	if value, ok := priorities[name]; ok {
		return value
	}
	return defaultContainerPriority
}
