// Package redact removes credentials from support archives before they leave
// the host. It is intentionally conservative: a value is replaced when its key
// looks sensitive, while unrelated settings stay readable for troubleshooting.
package redact

import (
	"encoding/json"
	"strings"
)

const placeholder = "[redacted]"

var sensitiveParts = []string{
	"password", "passwd", "secret", "token", "webhook", "ntfy",
	"apikey", "api_key", "privatekey", "private_key", "psk", "hash",
}

// Key reports whether a configuration key should be hidden.
func Key(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	for _, part := range sensitiveParts {
		if strings.Contains(lower, part) {
			return true
		}
	}
	return false
}

// JSON redacts sensitive object keys. Invalid JSON is returned untouched so a
// malformed file still reaches support instead of silently disappearing.
func JSON(data []byte) []byte {
	var value any
	if json.Unmarshal(data, &value) != nil {
		return data
	}
	redactValue(value)
	out, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return data
	}
	return out
}

func redactValue(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if Key(key) {
				typed[key] = placeholder
				continue
			}
			redactValue(item)
		}
	case []any:
		for _, item := range typed {
			redactValue(item)
		}
	}
}

// Config redacts key=value lines from INI/cfg files while preserving comments,
// ordering and line endings.
func Config(data []byte) []byte {
	lines := strings.Split(string(data), "\n")
	for index, raw := range lines {
		cr := strings.HasSuffix(raw, "\r")
		line := strings.TrimSuffix(raw, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "[") {
			continue
		}
		key, _, ok := strings.Cut(trimmed, "=")
		if !ok || !Key(key) {
			continue
		}
		prefix := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		line = prefix + strings.TrimSpace(key) + "=\"" + placeholder + "\""
		if cr {
			line += "\r"
		}
		lines[index] = line
	}
	return []byte(strings.Join(lines, "\n"))
}
