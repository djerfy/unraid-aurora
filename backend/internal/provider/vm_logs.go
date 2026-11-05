package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"aurora/internal/model"
)

// vmLogDirs lists where libvirt keeps per-domain logs. Unraid writes QEMU
// domains under qemu/, and this is the only directory Aurora reads from.
//
// It is a variable so tests can point it at a temporary directory.
var vmLogDirs = []string{"/var/log/libvirt/qemu"}

// VMLogs returns the tail of the libvirt log for a domain. The identifier is
// resolved against `virsh list --all --name`, so callers cannot read arbitrary
// files by crafting a path.
func (l *Live) VMLogs(ctx context.Context, id string, lines int) ([]model.LogLine, error) {
	if lines <= 0 || lines > 1000 {
		lines = 240
	}
	name, err := vmDomainName(ctx, id)
	if err != nil {
		return nil, err
	}
	return readVMLog(name, lines)
}

func vmDomainName(ctx context.Context, id string) (string, error) {
	out, err := exec.CommandContext(ctx, "virsh", "list", "--all", "--name").Output()
	if err != nil {
		return "", fmt.Errorf("list virtual machines: %w", err)
	}
	for _, raw := range strings.Split(string(out), "\n") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if name == id || "vm-"+slug(name) == id {
			return name, nil
		}
	}
	return "", errors.New("virtual machine not found")
}

// readVMLog resolves <domain>.log in the known log directories. Reads are
// limited to regular files and the name may not contain a path separator.
func readVMLog(name string, lines int) ([]model.LogLine, error) {
	base := strings.TrimSpace(name)
	if base == "" || base == "." || base == ".." || strings.ContainsAny(base, `/\`) || strings.ContainsRune(base, 0) {
		return nil, errors.New("invalid virtual machine name")
	}
	for _, dir := range vmLogDirs {
		path := filepath.Join(dir, base+".log")
		info, err := os.Lstat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		tail, err := tailLines(path, lines)
		if err != nil {
			return nil, err
		}
		return vmLogLines(tail), nil
	}
	// A domain that has never started has no log file yet.
	return []model.LogLine{}, nil
}

// vmLogLines parses the libvirt timestamp prefix so the UI shows when a line
// was written instead of the time it was fetched.
func vmLogLines(lines []string) []model.LogLine {
	now := time.Now()
	out := make([]model.LogLine, 0, len(lines))
	for _, raw := range lines {
		message := raw
		stamp := now
		if len(raw) > 28 && raw[4] == '-' && raw[7] == '-' && raw[10] == ' ' && raw[13] == ':' {
			if head, tail, ok := strings.Cut(raw, ": "); ok {
				if parsed, err := time.Parse("2006-01-02 15:04:05.000-0700", head); err == nil {
					stamp = parsed
					message = tail
				}
			}
		}
		level := "info"
		lower := strings.ToLower(message)
		switch {
		case strings.Contains(lower, "error") || strings.Contains(lower, "fail"):
			level = "error"
		case strings.Contains(lower, "warn"):
			level = "warn"
		case strings.Contains(lower, "debug"):
			level = "debug"
		}
		out = append(out, model.LogLine{Time: stamp, Level: level, Source: "libvirt", Message: message})
	}
	return out
}
