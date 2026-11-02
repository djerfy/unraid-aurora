package provider

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type raplReading struct {
	energy uint64
	max    uint64
}

// cpuPower samples package energy counters. The first call only records a
// baseline; subsequent samples return the average package power over the
// interval. hwmon is used when RAPL is unavailable.
func (l *Live) cpuPower(now time.Time) float64 {
	readings := readEnergyCounters()
	if len(readings) == 0 {
		return readHWMonPower()
	}
	if l.raplAt.IsZero() {
		l.raplLast = readings
		l.raplAt = now
		return readHWMonPower()
	}
	seconds := now.Sub(l.raplAt).Seconds()
	previous := l.raplLast
	l.raplLast = readings
	l.raplAt = now
	if seconds <= 0 {
		return readHWMonPower()
	}
	if power := raplDeltaPower(previous, readings, seconds); power > 0 {
		return power
	}
	return readHWMonPower()
}

func raplDeltaPower(previous, current map[string]raplReading, seconds float64) float64 {
	if seconds <= 0 {
		return 0
	}
	var deltaUJ float64
	for path, reading := range current {
		last, ok := previous[path]
		if !ok {
			continue
		}
		delta := float64(reading.energy) - float64(last.energy)
		if reading.energy < last.energy && reading.max > 0 {
			delta = float64(reading.max - last.energy + reading.energy)
		}
		if delta > 0 {
			deltaUJ += delta
		}
	}
	if deltaUJ <= 0 {
		return 0
	}
	return round1(deltaUJ / 1_000_000 / seconds)
}

func readEnergyCounters() map[string]raplReading {
	return readEnergyCountersFrom("/sys/class/powercap", "/sys/class/hwmon")
}

func readEnergyCountersFrom(powercapBase, hwmonBase string) map[string]raplReading {
	out := map[string]raplReading{}
	for _, namePath := range glob(filepath.Join(powercapBase, "intel-rapl:*/name")) {
		if !strings.HasPrefix(strings.ToLower(readTrim(namePath)), "package") {
			continue
		}
		dir := filepath.Dir(namePath)
		energy, err := readUint64(filepath.Join(dir, "energy_uj"))
		if err != nil {
			continue
		}
		max, _ := readUint64(filepath.Join(dir, "max_energy_range_uj"))
		out[dir] = raplReading{energy: energy, max: max}
	}
	for _, energyPath := range glob(filepath.Join(hwmonBase, "hwmon*/energy1_input")) {
		dir := filepath.Dir(energyPath)
		name := strings.ToLower(readTrim(filepath.Join(dir, "name")))
		if !isCPUPowerSensor(name) {
			continue
		}
		energy, err := readUint64(energyPath)
		if err != nil {
			continue
		}
		out["hwmon:"+dir] = raplReading{energy: energy}
	}
	return out
}

func readHWMonPower() float64 {
	best := 0.0
	for _, path := range glob("/sys/class/hwmon/hwmon*/power1_input") {
		name := strings.ToLower(readTrim(filepath.Join(filepath.Dir(path), "name")))
		if !isCPUPowerSensor(name) {
			continue
		}
		if watts := readFloat(path) / 1_000_000; watts > best {
			best = watts
		}
	}
	return round1(best)
}

func isCPUPowerSensor(name string) bool {
	for _, marker := range []string{"cpu", "core", "rapl", "zen", "k10", "amd_energy"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func readUint64(path string) (uint64, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
}

func glob(pattern string) []string {
	matches, _ := filepath.Glob(pattern)
	return matches
}
