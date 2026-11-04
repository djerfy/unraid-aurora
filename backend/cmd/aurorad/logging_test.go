package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeLogFormat(t *testing.T) {
	for _, input := range []string{"", "text", "TEXT", " json "} {
		if _, err := normalizeLogFormat(input); err != nil {
			t.Fatalf("%q: %v", input, err)
		}
	}
	if _, err := normalizeLogFormat("xml"); err == nil {
		t.Fatal("unsupported formats must be rejected")
	}
}

func TestNormalizeLogLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"":        slog.LevelInfo,
		"INFO":    slog.LevelInfo,
		"debug":   slog.LevelDebug,
		" warn ":  slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
	}
	for input, want := range cases {
		got, err := normalizeLogLevel(input)
		if err != nil {
			t.Fatalf("%q: %v", input, err)
		}
		if got != want {
			t.Fatalf("%q: got %v, want %v", input, got, want)
		}
	}
	if _, err := normalizeLogLevel("trace"); err == nil {
		t.Fatal("unsupported levels must be rejected")
	}
}

func TestNewLoggerHonoursLevel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aurora.log")
	logger, closeLog, err := newLogger(slog.LevelWarn, "text", path)
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	logger.Info("hidden")
	logger.Warn("visible")
	closeLog()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if strings.Contains(string(raw), "hidden") || !strings.Contains(string(raw), "visible") {
		t.Fatalf("level filtering failed: %q", string(raw))
	}
}

func TestNewLoggerJSONToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "aurora.log")
	logger, closeLog, err := newLogger(slog.LevelInfo, "json", path)
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	logger.Info("hello", "component", "test")
	closeLog()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &payload); err != nil {
		t.Fatalf("log line is not JSON: %v (%q)", err, string(raw))
	}
	if payload["msg"] != "hello" || payload["component"] != "test" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestNewLoggerTextToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aurora.log")
	logger, closeLog, err := newLogger(slog.LevelInfo, "text", path)
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	logger.Warn("disk hot", "device", "sdb")
	closeLog()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	line := string(raw)
	if !strings.Contains(line, "msg=\"disk hot\"") || !strings.Contains(line, "device=sdb") {
		t.Fatalf("unexpected text log: %q", line)
	}
}

func TestNewLoggerRejectsInvalidFormat(t *testing.T) {
	if _, _, err := newLogger(slog.LevelInfo, "yaml", ""); err == nil {
		t.Fatal("invalid format should fail")
	}
}
