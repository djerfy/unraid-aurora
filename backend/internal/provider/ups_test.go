package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseAPCUPSDStatus(t *testing.T) {
	raw := `APC      : 001,036,0885
DATE     : 2026-11-02 18:00:00 +0100
MODEL    : Back-UPS RS 1500MS2
STATUS   : ONLINE
LINEV    : 230.0 Volts
LOADPCT  : 23.0 Percent
BCHARGE  : 100.0 Percent
TIMELEFT : 42.0 Minutes
OUTPUTV  : 230.0 Volts
NOMPOWER : 900.0 Watts`
	ups, ok := parseAPCUPSDStatus(raw)
	if !ok {
		t.Fatal("apcupsd output was rejected")
	}
	if ups.Model != "Back-UPS RS 1500MS2" || ups.Status != "ONLINE" || ups.OnBattery {
		t.Fatalf("unexpected identity: %+v", ups)
	}
	if ups.Battery != 100 || ups.Load != 23 || ups.Runtime != 42 {
		t.Fatalf("unexpected measurements: %+v", ups)
	}
	if ups.InputVoltage != 230 || ups.OutputVoltage != 230 || ups.NominalPower != 900 {
		t.Fatalf("unexpected voltages: %+v", ups)
	}
}

func TestParseAPCUPSDStatusReportsOnBattery(t *testing.T) {
	ups, ok := parseAPCUPSDStatus("MODEL : Smart-UPS\nSTATUS : ONBATT\nBCHARGE : 74.0 Percent\nTIMELEFT : 12.0 Minutes")
	if !ok || !ups.OnBattery {
		t.Fatalf("on-battery state was not detected: %+v (%v)", ups, ok)
	}
}

func TestParseAPCUPSDStatusRejectsErrors(t *testing.T) {
	if _, ok := parseAPCUPSDStatus("Error contacting apcupsd @ localhost:3551: Connection refused"); ok {
		t.Fatal("an apcupsd error was accepted as a UPS")
	}
}

func TestParseNUTStatus(t *testing.T) {
	raw := `battery.charge: 100
battery.charge.low: 10
battery.runtime: 3600
device.mfr: EATON
device.model: 5PX 1500
input.voltage: 230.0
output.voltage: 231.0
ups.load: 23
ups.mfr: EATON
ups.model: 5PX 1500
ups.power.nominal: 1500
ups.realpower.nominal: 1350
ups.status: OL CHRG`
	ups, ok := parseNUTStatus(raw)
	if !ok {
		t.Fatal("NUT output was rejected")
	}
	if ups.Model != "EATON 5PX 1500" || ups.Status != "ONLINE CHARGING" || ups.OnBattery {
		t.Fatalf("unexpected identity: %+v", ups)
	}
	if ups.Battery != 100 || ups.Load != 23 || ups.Runtime != 60 {
		t.Fatalf("unexpected measurements: %+v", ups)
	}
	if ups.InputVoltage != 230 || ups.OutputVoltage != 231 || ups.NominalPower != 1350 {
		t.Fatalf("unexpected voltages: %+v", ups)
	}
}

func TestParseNUTStatusOnBattery(t *testing.T) {
	ups, ok := parseNUTStatus("ups.status: OB LB DISCHRG\nbattery.charge: 9\nbattery.runtime: 120\nups.load: 41")
	if !ok || !ups.OnBattery {
		t.Fatalf("on-battery state was not detected: %+v (%v)", ups, ok)
	}
	if ups.Status != "ONBATT LOWBATT DISCHARGING" {
		t.Fatalf("unexpected status: %q", ups.Status)
	}
}

func TestParseNUTStatusFallsBackToDeviceIdentity(t *testing.T) {
	ups, ok := parseNUTStatus("device.mfr: CyberPower\ndevice.model: CP1500PFCLCD\nups.status: OL\nbattery.charge: 100")
	if !ok || ups.Model != "CyberPower CP1500PFCLCD" {
		t.Fatalf("device identity was not used: %+v (%v)", ups, ok)
	}
}

func TestParseNUTStatusRejectsErrors(t *testing.T) {
	if _, ok := parseNUTStatus("Error: Connection failure: Connection refused"); ok {
		t.Fatal("a NUT error was accepted as a UPS")
	}
}

func TestParseNUTConfig(t *testing.T) {
	raw := `# Network UPS Tools
[ups]
driver = usbhid-ups
port = auto

[rack-ups]
driver = snmp-ups
port = 192.168.1.20
`
	names := parseNUTConfig(raw)
	if len(names) != 2 || names[0] != "ups" || names[1] != "rack-ups" {
		t.Fatalf("unexpected config names: %v", names)
	}
}

func TestParseNUTPluginConfig(t *testing.T) {
	raw := `SERVICE="enable"
NAME="rack-ups"
IPADDR="192.168.1.20"
DRIVER="snmp-ups"
`
	name, ip := parseNUTPluginConfig(raw)
	if name != "rack-ups" || ip != "192.168.1.20" {
		t.Fatalf("unexpected NUT plugin target: %q @ %q", name, ip)
	}
}

func TestParseNUTPluginConfigDefaults(t *testing.T) {
	name, ip := parseNUTPluginConfig("SERVICE=\"enable\"\n")
	if name != "ups" || ip != "127.0.0.1" {
		t.Fatalf("unexpected NUT plugin defaults: %q @ %q", name, ip)
	}
}

func TestNUTRuntimeConvertsSecondsToMinutes(t *testing.T) {
	cases := map[string]int{"0": 0, "30": 1, "90": 2, "3600": 60}
	for input, want := range cases {
		if got := nutRuntime(input); got != want {
			t.Errorf("nutRuntime(%q) = %d, want %d", input, got, want)
		}
	}
}

func fakeCommand(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestLiveUPSPrefersAPCUPSD(t *testing.T) {
	dir := t.TempDir()
	fakeCommand(t, dir, "apcaccess", `echo "MODEL : Back-UPS 1500"
echo "STATUS : ONLINE"
echo "BCHARGE : 100.0 Percent"
echo "TIMELEFT : 30.0 Minutes"`)
	t.Setenv("PATH", dir)

	ups, err := (&Live{}).UPS(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !ups.Present || ups.Model != "Back-UPS 1500" || ups.Runtime != 30 {
		t.Fatalf("apcupsd was not preferred: %+v", ups)
	}
}

func TestLiveUPSFallsBackToNUT(t *testing.T) {
	dir := t.TempDir()
	fakeCommand(t, dir, "apcaccess", "exit 1")
	fakeCommand(t, dir, "upsc", `if [ "$1" = "-l" ]; then
  echo "rackups"
  exit 0
fi
if [ "$1" = "rackups" ]; then
  echo "ups.mfr: EATON"
  echo "ups.model: 5PX 1500"
  echo "ups.status: OB"
  echo "battery.charge: 74"
  echo "battery.runtime: 900"
  echo "ups.load: 32"
  echo "input.voltage: 229"
  exit 0
fi
exit 1`)
	t.Setenv("PATH", dir)

	ups, err := (&Live{}).UPS(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !ups.Present || ups.Model != "EATON 5PX 1500" || !ups.OnBattery {
		t.Fatalf("NUT fallback was not used: %+v", ups)
	}
	if ups.Runtime != 15 || ups.Battery != 74 || ups.Load != 32 {
		t.Fatalf("unexpected NUT measurements: %+v", ups)
	}
}
