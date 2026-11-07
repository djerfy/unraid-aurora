package provider

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Unraid stores its manager settings as flat shell-style KEY="value" files
// that PHP reads with parse_ini_file.
func configValue(values map[string]string, key, fallback string) string {
	if value, ok := values[key]; ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func configBool(values map[string]string, key string, fallback bool) bool {
	value, ok := values[key]
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.EqualFold(strings.TrimSpace(value), "yes")
}

func configInt(values map[string]string, key string, fallback int) int {
	value, ok := values[key]
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return number
}

// updateConfigValues rewrites only the requested keys so unknown entries
// written by other Unraid versions stay untouched.
func updateConfigValues(path string, updates map[string]string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return rewriteConfigValues(string(raw), updates, true), nil
}

// rewriteConfigValues preserves unknown keys and rewrites the requested ones.
// Unraid managers mix quoted (docker.cfg) and plain (nvidia-driver/settings.cfg)
// values, so callers pick the formatting.
func rewriteConfigValues(raw string, updates map[string]string, quote bool) []byte {
	remaining := make(map[string]string, len(updates))
	for key, value := range updates {
		remaining[key] = value
	}
	format := func(key, value string) string { return key + "=" + value }
	if quote {
		format = quoteConfigValue
	}
	lines := []string{}
	for _, line := range strings.Split(strings.TrimRight(raw, "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
			lines = append(lines, line)
			continue
		}
		key, _, ok := strings.Cut(trimmed, "=")
		if !ok {
			lines = append(lines, line)
			continue
		}
		key = strings.TrimSpace(key)
		value, exists := remaining[key]
		if !exists {
			lines = append(lines, line)
			continue
		}
		lines = append(lines, format(key, value))
		delete(remaining, key)
	}
	if len(remaining) > 0 {
		keys := make([]string, 0, len(remaining))
		for key := range remaining {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			lines = append(lines, format(key, remaining[key]))
		}
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func intOrEmpty(value int) string {
	if value <= 0 {
		return ""
	}
	return strconv.Itoa(value)
}

func quoteConfigValue(key, value string) string {
	return key + "=\"" + strings.ReplaceAll(value, "\"", "\\\"") + "\""
}
