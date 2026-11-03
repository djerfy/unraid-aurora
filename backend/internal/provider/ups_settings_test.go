package provider

import "testing"

func TestParseUPSSettings(t *testing.T) {
	raw := `SERVICE="enable"
UPSCABLE="usb"
UPSTYPE="usb"
DEVICE=""
OVERRIDE_UPS_CAPACITY="900"
BATTERYLEVEL="20"
MINUTES="15"
TIMEOUT="60"
KILLUPS="yes"
`
	settings := parseUPSSettings(raw)
	if settings.Service != "enable" || settings.BatteryLevel != 20 || settings.Minutes != 15 || settings.Timeout != 60 || settings.KillUPS != "yes" || settings.OverrideCapacity != 900 {
		t.Fatalf("unexpected UPS settings: %+v", settings)
	}
}
