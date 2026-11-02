package provider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRAPLDeltaPower(t *testing.T) {
	previous := map[string]raplReading{
		"package-0": {energy: 100_000_000, max: 262_143_328_850},
		"package-1": {energy: 40_000_000, max: 262_143_328_850},
	}
	current := map[string]raplReading{
		"package-0": {energy: 200_000_000, max: 262_143_328_850},
		"package-1": {energy: 60_000_000, max: 262_143_328_850},
	}
	if got := raplDeltaPower(previous, current, 2); got != 60 {
		t.Fatalf("raplDeltaPower = %v, want 60", got)
	}
}

func TestRAPLDeltaPowerHandlesWrap(t *testing.T) {
	previous := map[string]raplReading{"package-0": {energy: 9_000_000, max: 10_000_000}}
	current := map[string]raplReading{"package-0": {energy: 1_000_000, max: 10_000_000}}
	if got := raplDeltaPower(previous, current, 2); got != 1 {
		t.Fatalf("wrapped counter = %v, want 1", got)
	}
}

func TestRAPLDeltaPowerIgnoresMissingSamples(t *testing.T) {
	current := map[string]raplReading{"package-0": {energy: 200_000_000}}
	if got := raplDeltaPower(map[string]raplReading{}, current, 2); got != 0 {
		t.Fatalf("missing baseline = %v, want 0", got)
	}
	if got := raplDeltaPower(current, current, 0); got != 0 {
		t.Fatalf("zero interval = %v, want 0", got)
	}
}

func TestReadEnergyCountersFrom(t *testing.T) {
	root := t.TempDir()
	powercap := filepath.Join(root, "powercap")
	hwmon := filepath.Join(root, "hwmon")

	writeFile := func(path, value string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(filepath.Join(powercap, "intel-rapl:0", "name"), "package-0")
	writeFile(filepath.Join(powercap, "intel-rapl:0", "energy_uj"), "1234000")
	writeFile(filepath.Join(powercap, "intel-rapl:0", "max_energy_range_uj"), "262143328850")
	writeFile(filepath.Join(powercap, "intel-rapl:0:0", "name"), "core")
	writeFile(filepath.Join(powercap, "intel-rapl:0:0", "energy_uj"), "999")
	writeFile(filepath.Join(hwmon, "hwmon0", "name"), "zenpower")
	writeFile(filepath.Join(hwmon, "hwmon0", "energy1_input"), "4321000")
	writeFile(filepath.Join(hwmon, "hwmon1", "name"), "nct6775")
	writeFile(filepath.Join(hwmon, "hwmon1", "energy1_input"), "777")

	readings := readEnergyCountersFrom(powercap, hwmon)
	if len(readings) != 2 {
		t.Fatalf("readings = %+v, want package and CPU hwmon counters", readings)
	}
	packageReading, ok := readings[filepath.Join(powercap, "intel-rapl:0")]
	if !ok || packageReading.energy != 1234000 || packageReading.max != 262143328850 {
		t.Fatalf("package reading = %+v", packageReading)
	}
	if _, ok := readings["hwmon:"+filepath.Join(hwmon, "hwmon0")]; !ok {
		t.Fatalf("CPU hwmon energy was not read: %+v", readings)
	}
}
