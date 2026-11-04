package provider

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"aurora/internal/model"
)

const nutProbeTimeout = 3 * time.Second

var (
	nutNamePattern   = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	nutTargetPattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.@:\[\]-]*$`)
)

// UPS reads the built-in Unraid apcupsd service first, then falls back to the
// NUT plugin. Both are best-effort: a missing daemon simply reports no UPS.
func (l *Live) UPS(ctx context.Context) (model.UPS, error) {
	l.upsMu.Lock()
	if !l.upsCacheAt.IsZero() && time.Since(l.upsCacheAt) < 2*time.Second {
		cached := l.upsCache
		l.upsMu.Unlock()
		return cached, nil
	}
	l.upsMu.Unlock()

	ups, err := l.readUPS(ctx)
	if err != nil {
		return ups, err
	}
	l.upsMu.Lock()
	l.upsCache = ups
	l.upsCacheAt = time.Now()
	l.upsMu.Unlock()
	return ups, nil
}

func (l *Live) readUPS(ctx context.Context) (model.UPS, error) {
	if path := findCommand("apcaccess", "/sbin/apcaccess", "/usr/sbin/apcaccess", "/usr/bin/apcaccess", "/usr/local/bin/apcaccess"); path != "" {
		if out, err := exec.CommandContext(ctx, path, "status").Output(); err == nil {
			if ups, ok := parseAPCUPSDStatus(string(out)); ok {
				return ups, nil
			}
		}
	}
	if ups, ok := nutUPS(ctx); ok {
		return ups, nil
	}
	return model.UPS{Present: false}, nil
}

func findCommand(names ...string) string {
	for _, name := range names {
		if strings.ContainsRune(name, '/') {
			info, err := os.Stat(name)
			if err == nil && !info.IsDir() {
				return name
			}
			continue
		}
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func parseKeyValues(output string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if key != "" {
			values[key] = value
		}
	}
	return values
}

func parseAPCUPSDStatus(output string) (model.UPS, bool) {
	values := parseKeyValues(output)
	if len(values) == 0 {
		return model.UPS{}, false
	}
	status := values["STATUS"]
	ups := model.UPS{
		Present:       true,
		Model:         values["MODEL"],
		Status:        status,
		OnBattery:     strings.Contains(status, "ONBATT"),
		Battery:       firstFloat(values["BCHARGE"]),
		Load:          firstFloat(values["LOADPCT"]),
		Runtime:       int(firstFloat(values["TIMELEFT"])),
		InputVoltage:  firstFloat(values["LINEV"]),
		OutputVoltage: firstFloat(values["OUTPUTV"]),
		NominalPower:  firstFloat(values["NOMPOWER"]),
		LastEvent:     values["LASTXFER"],
	}
	if ups.Model == "" && ups.Status == "" && ups.Battery == 0 && ups.Runtime == 0 {
		return model.UPS{}, false
	}
	return ups, true
}

func nutUPS(ctx context.Context) (model.UPS, bool) {
	client := findCommand("upsc", "/usr/bin/upsc", "/usr/local/bin/upsc", "/usr/sbin/upsc", "/sbin/upsc")
	if client == "" {
		return model.UPS{}, false
	}
	probeCtx, cancel := context.WithTimeout(ctx, nutProbeTimeout)
	defer cancel()

	// Probe the NUT plugin's configured target first: upsc -l can block when a
	// local upsd is absent but the plugin points at a remote server.
	if name, ip := nutPluginTarget(); name != "" {
		var primary []string
		if ip != "" {
			primary = append(primary, name+"@"+ip)
		}
		primary = append(primary, name)
		if ups, ok := probeNUT(probeCtx, client, primary); ok {
			return ups, true
		}
	}
	return probeNUT(probeCtx, client, nutFallbackCandidates(probeCtx, client))
}

func probeNUT(ctx context.Context, client string, names []string) (model.UPS, bool) {
	seen := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] || !nutTargetPattern.MatchString(name) {
			continue
		}
		seen[name] = true
		out, err := exec.CommandContext(ctx, client, name).Output()
		if err != nil {
			continue
		}
		if ups, ok := parseNUTStatus(string(out)); ok {
			return ups, true
		}
	}
	return model.UPS{}, false
}

func nutFallbackCandidates(ctx context.Context, client string) []string {
	seen := map[string]bool{}
	var names []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] || !nutTargetPattern.MatchString(name) {
			return
		}
		seen[name] = true
		names = append(names, name)
	}

	if out, err := exec.CommandContext(ctx, client, "-l").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			add(line)
		}
	}
	for _, name := range nutConfigNames() {
		add(name)
	}
	// The NUT plugin uses "ups" by default and upsc cannot list a UPS when
	// upsd only answers single-variable queries.
	add("ups")
	return names
}

func nutPluginTarget() (string, string) {
	raw, err := os.ReadFile(filepath.Clean("/boot/config/plugins/nut-dw/nut-dw.cfg"))
	if err != nil {
		return "", ""
	}
	return parseNUTPluginConfig(string(raw))
}

func parseNUTPluginConfig(raw string) (string, string) {
	name, ip := "ups", "127.0.0.1"
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.ToUpper(strings.TrimSpace(key))
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		switch key {
		case "NAME":
			if nutNamePattern.MatchString(value) {
				name = value
			}
		case "IPADDR":
			if value != "" && nutTargetPattern.MatchString(value) {
				ip = value
			}
		}
	}
	return name, ip
}

func nutConfigNames() []string {
	var names []string
	for _, path := range []string{
		"/etc/nut/ups.conf",
		"/boot/config/plugins/nut-dw/ups.conf",
	} {
		raw, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			continue
		}
		names = append(names, parseNUTConfig(string(raw))...)
	}
	return names
}

func parseNUTConfig(raw string) []string {
	var names []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.HasPrefix(line, "[") {
			continue
		}
		name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
		if nutNamePattern.MatchString(name) {
			names = append(names, name)
		}
	}
	return names
}

func parseNUTStatus(output string) (model.UPS, bool) {
	values := parseKeyValues(output)
	if len(values) == 0 {
		return model.UPS{}, false
	}
	status := values["ups.status"]
	ups := model.UPS{
		Present:       true,
		Model:         nutModel(values),
		Status:        nutStatusText(status),
		OnBattery:     nutHasFlag(status, "OB"),
		Battery:       firstFloat(values["battery.charge"]),
		Load:          firstFloat(values["ups.load"]),
		Runtime:       nutRuntime(values["battery.runtime"]),
		InputVoltage:  firstFloat(values["input.voltage"]),
		OutputVoltage: firstFloat(values["output.voltage"]),
		NominalPower:  nutNominalPower(values),
		LastEvent:     strings.TrimSpace(values["ups.alarm"]),
	}
	if ups.Model == "" && ups.Status == "" && ups.Battery == 0 && ups.Runtime == 0 && ups.Load == 0 {
		return model.UPS{}, false
	}
	return ups, true
}

func nutModel(values map[string]string) string {
	modelName := strings.TrimSpace(values["ups.model"])
	if modelName == "" {
		modelName = strings.TrimSpace(values["device.model"])
	}
	brand := strings.TrimSpace(values["ups.mfr"])
	if brand == "" {
		brand = strings.TrimSpace(values["device.mfr"])
	}
	if modelName == "" {
		return brand
	}
	if brand == "" || strings.Contains(strings.ToLower(modelName), strings.ToLower(brand)) {
		return modelName
	}
	return brand + " " + modelName
}

// NUT reports runtime in seconds, while apcupsd and Aurora's UI use minutes.
func nutRuntime(value string) int {
	seconds := firstFloat(value)
	if seconds <= 0 {
		return 0
	}
	return int(math.Round(seconds / 60))
}

func nutNominalPower(values map[string]string) float64 {
	for _, key := range []string{
		"ups.realpower.nominal",
		"ups.power.nominal",
		"output.power.nominal",
	} {
		if value := firstFloat(values[key]); value > 0 {
			return value
		}
	}
	return 0
}

func nutHasFlag(status, flag string) bool {
	for _, field := range strings.Fields(status) {
		if field == flag {
			return true
		}
	}
	return false
}

func nutStatusText(status string) string {
	labels := map[string]string{
		"OL":      "ONLINE",
		"OB":      "ONBATT",
		"LB":      "LOWBATT",
		"RB":      "REPLACEBATT",
		"CHRG":    "CHARGING",
		"DISCHRG": "DISCHARGING",
		"BYPASS":  "BYPASS",
		"CAL":     "CALIBRATE",
		"OFF":     "OFF",
		"ALARM":   "ALARM",
		"FSD":     "SHUTTING DOWN",
	}
	fields := strings.Fields(status)
	for i, field := range fields {
		if label, ok := labels[field]; ok {
			fields[i] = label
		}
	}
	return strings.Join(fields, " ")
}
