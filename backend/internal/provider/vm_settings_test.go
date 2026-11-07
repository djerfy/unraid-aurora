package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurora/internal/model"
)

func TestVMSettingsRoundTrip(t *testing.T) {
	boot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(boot, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(boot, "config", "domain.cfg")
	initial := `SERVICE="enable"
DISABLE="no"
VIRTIOISO=""
DOMAINDIR="/mnt/user/domains/"
MEDIADIR="/mnt/user/isos/"
UNKNOWN="keep"
`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	live := &Live{bootDir: boot}

	settings, err := live.VMSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !settings.Enabled || settings.DisableAutostart || settings.VirtioISO != "" {
		t.Fatalf("unexpected settings: %+v", settings)
	}

	settings.Enabled = false
	settings.DisableAutostart = true
	settings.VirtioISO = "/mnt/user/isos/virtio-win.iso"
	if err := live.SaveVMSettings(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, expected := range []string{`SERVICE="disable"`, `DISABLE="yes"`, `VIRTIOISO="/mnt/user/isos/virtio-win.iso"`, `UNKNOWN="keep"`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("domain.cfg is missing %q:\n%s", expected, text)
		}
	}
}

func TestVMSettingsValidation(t *testing.T) {
	live := &Live{bootDir: t.TempDir()}
	for _, iso := range []string{"relative.iso", "/etc/driver.iso", "/mnt/user/isos/driver.txt", "/mnt/user/isos/driver\n.iso"} {
		if err := live.SaveVMSettings(context.Background(), model.VMSettings{VirtioISO: iso}); err == nil {
			t.Fatalf("%q should be rejected", iso)
		}
	}
	if err := live.SaveVMSettings(context.Background(), model.VMSettings{VirtioISO: "/mnt/user/isos/virtio-win.iso"}); err != nil {
		t.Fatal(err)
	}
}
