package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"aurora/internal/model"
)

const upsConfigPath = "/boot/config/plugins/dynamix.apcupsd/dynamix.apcupsd.cfg"

func (l *Live) UPSSettings(ctx context.Context) (model.UPSSettings, error) {
	raw, err := os.ReadFile(upsConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return model.UPSSettings{
				Service: "disable", Cable: "usb", Type: "usb",
				BatteryLevel: 10, Minutes: 10, Timeout: 0, KillUPS: "no",
			}, nil
		}
		return model.UPSSettings{}, err
	}
	return parseUPSSettings(string(raw)), nil
}

func parseUPSSettings(raw string) model.UPSSettings {
	values := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	battery, batteryErr := strconv.Atoi(values["BATTERYLEVEL"])
	minutes, minutesErr := strconv.Atoi(values["MINUTES"])
	timeout, _ := strconv.Atoi(values["TIMEOUT"])
	capacity, _ := strconv.Atoi(values["OVERRIDE_UPS_CAPACITY"])
	if batteryErr != nil {
		battery = 10
	}
	if minutesErr != nil {
		minutes = 10
	}
	return model.UPSSettings{
		Service: values["SERVICE"], Cable: values["UPSCABLE"], Type: values["UPSTYPE"],
		Device: values["DEVICE"], BatteryLevel: battery, Minutes: minutes, Timeout: timeout,
		KillUPS: values["KILLUPS"], OverrideCapacity: capacity,
	}
}

func (l *Live) SaveUPSSettings(ctx context.Context, settings model.UPSSettings) error {
	if settings.BatteryLevel < 0 || settings.BatteryLevel > 100 {
		return errors.New("battery level must be between 0 and 100")
	}
	if settings.Minutes < 0 || settings.Minutes > 999 || settings.Timeout < 0 || settings.Timeout > 9999 {
		return errors.New("invalid UPS timing values")
	}
	if settings.OverrideCapacity < 0 {
		return errors.New("nominal power override must be positive")
	}
	for _, value := range []string{settings.Device, settings.Cable, settings.Type} {
		if strings.ContainsAny(value, "\"\n\r") {
			return errors.New("UPS settings contain invalid characters")
		}
	}
	if settings.KillUPS != "yes" && settings.KillUPS != "no" {
		settings.KillUPS = "no"
	}
	if settings.Service != "enable" && settings.Service != "disable" {
		settings.Service = "disable"
	}
	if settings.Cable == "" {
		settings.Cable = "usb"
	}
	if settings.Type == "" {
		settings.Type = "usb"
	}
	raw := fmt.Sprintf(`SERVICE="%s"
UPSCABLE="%s"
CUSTOMUPSCABLE=""
UPSTYPE="%s"
DEVICE="%s"
OVERRIDE_UPS_CAPACITY="%d"
BATTERYLEVEL="%d"
MINUTES="%d"
TIMEOUT="%d"
KILLUPS="%s"
`, settings.Service, settings.Cable, settings.Type, settings.Device,
		settings.OverrideCapacity, settings.BatteryLevel, settings.Minutes, settings.Timeout, settings.KillUPS)
	if err := os.MkdirAll(filepath.Dir(upsConfigPath), 0o750); err != nil {
		return err
	}
	tmp := upsConfigPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(raw), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, upsConfigPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if settings.Service == "enable" {
		if output, err := exec.CommandContext(ctx, "/etc/rc.d/rc.apcupsd", "restart").CombinedOutput(); err != nil {
			return fmt.Errorf("UPS settings saved but the service restart failed: %w (%s)", err, strings.TrimSpace(string(output)))
		}
	}
	return nil
}
