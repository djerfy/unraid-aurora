package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// newLogger builds the process logger. format is "text" (default) or "json";
// file redirects output to an append-only file instead of stdout.
func newLogger(level slog.Level, format, file string) (*slog.Logger, func(), error) {
	normalized, err := normalizeLogFormat(format)
	if err != nil {
		return nil, nil, err
	}

	var writer io.Writer = os.Stdout
	closeWriter := func() {}
	if strings.TrimSpace(file) != "" {
		handle, err := openLogFile(file)
		if err != nil {
			return nil, nil, err
		}
		writer = handle
		closeWriter = func() { _ = handle.Close() }
	}

	options := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if normalized == "json" {
		handler = slog.NewJSONHandler(writer, options)
	} else {
		handler = slog.NewTextHandler(writer, options)
	}
	return slog.New(handler), closeWriter, nil
}

func normalizeLogFormat(format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text":
		return "text", nil
	case "json":
		return "json", nil
	default:
		return "", fmt.Errorf("unsupported log format %q (use text or json)", format)
	}
}

func openLogFile(path string) (*os.File, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("log file path is empty")
	}
	if directory := filepath.Dir(path); directory != "" && directory != "." {
		if err := os.MkdirAll(directory, 0o750); err != nil {
			return nil, fmt.Errorf("create log directory: %w", err)
		}
	}
	handle, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return handle, nil
}
