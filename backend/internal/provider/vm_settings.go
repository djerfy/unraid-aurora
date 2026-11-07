package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"aurora/internal/model"
)

func (l *Live) vmConfigPath() string {
	if l.bootDir == "" {
		return ""
	}
	return filepath.Join(l.bootDir, "config", "domain.cfg")
}

// libvirtRunning mirrors rc.libvirt: the pid file must point at a live process.
func libvirtRunning() bool {
	raw, err := os.ReadFile("/var/run/libvirt/libvirtd.pid")
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		return false
	}
	_, err = os.Stat(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	return err == nil
}

func (l *Live) VMSettings(ctx context.Context) (model.VMSettings, error) {
	values := map[string]string{}
	if path := l.vmConfigPath(); path != "" {
		if ini, err := parseINI(path); err == nil {
			values = ini.section("")
		} else if !errors.Is(err, os.ErrNotExist) {
			return model.VMSettings{}, fmt.Errorf("read domain.cfg: %w", err)
		}
	}
	return model.VMSettings{
		Enabled:          strings.EqualFold(configValue(values, "SERVICE", "disable"), "enable"),
		Running:          libvirtRunning(),
		DisableAutostart: configBool(values, "DISABLE", false),
		VirtioISO:        configValue(values, "VIRTIOISO", ""),
	}, nil
}

func validVirtioISO(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	if strings.ContainsAny(value, "\x00\n\r\\") || !strings.HasPrefix(value, "/mnt/") {
		return false
	}
	return strings.HasSuffix(strings.ToLower(value), ".iso")
}

func (l *Live) SaveVMSettings(ctx context.Context, settings model.VMSettings) error {
	settings.VirtioISO = strings.TrimSpace(settings.VirtioISO)
	if !validVirtioISO(settings.VirtioISO) {
		return errors.New("VirtIO driver ISO must be an absolute /mnt path ending with .iso, or empty")
	}
	path := l.vmConfigPath()
	if path == "" {
		return errors.New("boot directory is unavailable")
	}
	raw, err := updateConfigValues(path, map[string]string{
		"SERVICE":   map[bool]string{true: "enable", false: "disable"}[settings.Enabled],
		"DISABLE":   yesNo(settings.DisableAutostart),
		"VIRTIOISO": settings.VirtioISO,
	})
	if err != nil {
		return err
	}
	return l.writeConfigFile("vm.settings", "Updated VM settings", path, raw)
}
