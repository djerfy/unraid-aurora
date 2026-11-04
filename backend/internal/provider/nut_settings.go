package provider

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"aurora/internal/model"
)

const nutConfigPath = "/boot/config/plugins/nut-dw/nut-dw.cfg"

var nutSettingNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

var nutDriverPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

func (l *Live) NUTSettings(ctx context.Context) (model.NUTSettings, error) {
	_ = ctx
	info, err := os.Stat(nutConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return model.NUTSettings{Available: false}, nil
		}
		return model.NUTSettings{}, err
	}
	if info.IsDir() {
		return model.NUTSettings{Available: false}, nil
	}
	ini, err := parseINI(nutConfigPath)
	if err != nil {
		return model.NUTSettings{}, err
	}
	get := func(key string) string { return ini.get("", key) }
	settings := model.NUTSettings{
		Available:    true,
		Service:      defaultString(get("SERVICE"), "disable"),
		Mode:         defaultString(get("MODE"), "standalone"),
		Name:         defaultString(get("NAME"), "ups"),
		Driver:       defaultString(get("DRIVER"), "usbhid-ups"),
		Port:         defaultString(get("PORT"), "auto"),
		IPAddr:       defaultString(get("IPADDR"), "127.0.0.1"),
		BatteryLevel: intOrDefault(get("BATTERYLEVEL"), 20),
		RuntimeValue: intOrDefault(get("RTVALUE"), 240),
		Timeout:      intOrDefault(get("TIMEOUT"), 240),
		Poll:         intOrDefault(get("POLL"), 15),
		UPSKill:      defaultString(get("UPSKILL"), "disable"),
		PasswordSet:  strings.TrimSpace(get("MONPASS")) != "",
	}
	return settings, nil
}

func (l *Live) SaveNUTSettings(ctx context.Context, settings model.NUTSettings) error {
	if _, err := os.Stat(nutConfigPath); err != nil {
		return errors.New("the NUT plugin is not installed")
	}
	switch settings.Service {
	case "enable", "disable":
	default:
		return errors.New("NUT service must be enable or disable")
	}
	switch settings.Mode {
	case "standalone", "netserver", "slave":
	default:
		return errors.New("NUT mode must be standalone, netserver or slave")
	}
	if !nutSettingNamePattern.MatchString(settings.Name) {
		return errors.New("invalid UPS name")
	}
	if !nutDriverPattern.MatchString(settings.Driver) {
		return errors.New("invalid UPS driver")
	}
	if strings.ContainsAny(settings.Port, "\"\n\r;") {
		return errors.New("invalid UPS port")
	}
	if settings.IPAddr != "" && net.ParseIP(settings.IPAddr) == nil && !nutSettingNamePattern.MatchString(settings.IPAddr) {
		return errors.New("invalid NUT monitor address")
	}
	if settings.BatteryLevel < 0 || settings.BatteryLevel > 100 {
		return errors.New("battery level must be between 0 and 100")
	}
	if settings.RuntimeValue < 0 || settings.RuntimeValue > 86400 || settings.Timeout < 0 || settings.Timeout > 86400 {
		return errors.New("invalid NUT runtime or timeout")
	}
	if settings.Poll < 1 || settings.Poll > 300 {
		return errors.New("poll interval must be between 1 and 300 seconds")
	}
	if settings.UPSKill != "enable" && settings.UPSKill != "disable" {
		settings.UPSKill = "disable"
	}
	values := map[string]string{
		"SERVICE":      settings.Service,
		"MODE":         settings.Mode,
		"NAME":         settings.Name,
		"DRIVER":       settings.Driver,
		"PORT":         settings.Port,
		"IPADDR":       settings.IPAddr,
		"BATTERYLEVEL": strconv.Itoa(settings.BatteryLevel),
		"RTVALUE":      strconv.Itoa(settings.RuntimeValue),
		"TIMEOUT":      strconv.Itoa(settings.Timeout),
		"POLL":         strconv.Itoa(settings.Poll),
		"UPSKILL":      settings.UPSKill,
	}
	if strings.TrimSpace(settings.MonitorPassword) != "" {
		values["MONPASS"] = base64.StdEncoding.EncodeToString([]byte(settings.MonitorPassword))
	}
	raw, err := os.ReadFile(nutConfigPath)
	if err != nil {
		return err
	}
	updated := upsertINIValues(string(raw), "", values)
	if err := l.writeConfigFile("ups.nut", "Update NUT settings", nutConfigPath, []byte(updated)); err != nil {
		return err
	}
	if _, err := os.Stat("/etc/rc.d/rc.nut"); err == nil {
		action := "restart"
		if settings.Service == "disable" {
			action = "stop"
		}
		if output, err := exec.CommandContext(ctx, "/etc/rc.d/rc.nut", action).CombinedOutput(); err != nil {
			return fmt.Errorf("NUT settings saved but the service %s failed: %w (%s)", action, err, strings.TrimSpace(string(output)))
		}
	}
	return nil
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func intOrDefault(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}
