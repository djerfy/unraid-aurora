package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildNvidiaVersions(t *testing.T) {
	kernel := "6.12.90-Unraid"
	names := []string{
		"nvidia-580.76.05-" + kernel + "-1.txz",
		"nvidia-580.76.05-" + kernel + "-1.txz.md5",
		"nvidia-575.64.05-" + kernel + "-1.txz",
		"nvidia-470.256.02-" + kernel + "-1.txz",
		"nvidia-470.199.02-" + kernel + "-1.txz",
		"nvos-575.57.08-" + kernel + "-1.txz",
		"nvidia-580.76.05-6.12.89-Unraid-1.txz",
	}
	assets := nvidiaAssetsFromNames(names, kernel)
	versions := buildNvidiaVersions(kernel, assets, map[string][]string{
		"production": {"575.64.05", "580.76.05"},
		"newfeature": {"575.64.05"},
	})
	if versions.Latest != "580.76.05" {
		t.Fatalf("unexpected latest: %+v", versions)
	}
	if versions.Production != "580.76.05" || versions.NewFeature != "575.64.05" {
		t.Fatalf("unexpected branches: %+v", versions)
	}
	if versions.OpenSource != "575.57.08" {
		t.Fatalf("unexpected open source branch: %+v", versions)
	}
	// Only the best 470.x legacy package is kept.
	if len(versions.Versions) != 3 || versions.Versions[2] != "470.256.02" {
		t.Fatalf("unexpected versions: %+v", versions.Versions)
	}

	cases := map[string]struct {
		openSource bool
		version    string
	}{
		"latest":     {false, "580.76.05"},
		"latest_prb": {false, "580.76.05"},
		"latest_nfb": {false, "575.64.05"},
		"latest_nos": {true, "575.57.08"},
		"470.256.02": {false, "470.256.02"},
	}
	for selection, expected := range cases {
		version, openSource, err := resolveNvidiaSelection(versions, selection)
		if err != nil {
			t.Fatalf("%s: %v", selection, err)
		}
		if version != expected.version || openSource != expected.openSource {
			t.Fatalf("%s: got %s open=%v", selection, version, openSource)
		}
	}
	if _, _, err := resolveNvidiaSelection(versions, "999.0.0"); err == nil {
		t.Fatal("unknown version should be rejected")
	}
}

func TestNvidiaPackageNames(t *testing.T) {
	kernel := "6.12.90-Unraid"
	if got := nvidiaPackageName("580.76.05", kernel, false); got != "nvidia-580.76.05-6.12.90-Unraid-1.txz" {
		t.Fatalf("unexpected proprietary package: %s", got)
	}
	if got := nvidiaPackageName("575.57.08", kernel, true); got != "nvos-575.57.08-6.12.90-Unraid-1.txz" {
		t.Fatalf("unexpected open source package: %s", got)
	}
	if got := nvidiaVersionFromPackage("nvidia-580.76.05-6.12.90-Unraid-1.txz", kernel); got != "580.76.05" {
		t.Fatalf("unexpected parsed version: %s", got)
	}
	if got := nvidiaVersionFromPackage("nvos-575.57.08-6.12.90-Unraid-1.txz", kernel); got != "575.57.08" {
		t.Fatalf("unexpected parsed open source version: %s", got)
	}
}

func TestParseNvidiaModinfo(t *testing.T) {
	version, license := parseNvidiaModinfo("filename:       /lib/modules/nvidia.ko\nlicense:        NVIDIA\nversion:        580.76.05\n")
	if version != "580.76.05" || license != "NVIDIA" {
		t.Fatalf("unexpected modinfo: %s %s", version, license)
	}
	if got := nvidiaLicenseName("Dual MIT/GPL"); got != "opensource" {
		t.Fatalf("unexpected license mapping: %s", got)
	}
	if got := nvidiaLicenseName(""); got != "none" {
		t.Fatalf("unexpected empty license mapping: %s", got)
	}
}

func TestSaveNvidiaSelection(t *testing.T) {
	boot := t.TempDir()
	settingsDir := filepath.Join(boot, "config", "plugins", "nvidia-driver")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(settingsDir, "settings.cfg")
	if err := os.WriteFile(path, []byte("driver_version=latest\ndisable_xconfig=false\nupdate_check=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	live := &Live{bootDir: boot}
	if err := live.saveNvidiaSelection(context.Background(), "580.76.05"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, expected := range []string{"driver_version=580.76.05", "update_check=false", "disable_xconfig=false"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("settings.cfg is missing %q:\n%s", expected, text)
		}
	}
	if strings.Contains(text, `"580.76.05"`) {
		t.Fatalf("nvidia-driver settings must stay unquoted:\n%s", text)
	}

	if err := live.saveNvidiaSelection(context.Background(), "latest_nos"); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(path)
	if !strings.Contains(string(raw), "driver_version=latest_nos") || !strings.Contains(string(raw), "update_check=false") {
		t.Fatalf("unexpected open source selection:\n%s", string(raw))
	}
}

func TestNvidiaStatusReadsSettingsAndPackage(t *testing.T) {
	boot := t.TempDir()
	settingsDir := filepath.Join(boot, "config", "plugins", "nvidia-driver")
	packages := filepath.Join(settingsDir, "packages", "6.12.90")
	if err := os.MkdirAll(packages, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.cfg"),
		[]byte("driver_version=latest_prb\nupdate_check=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packages, "nvidia-580.76.05-6.12.90-Unraid-1.txz"), []byte("pkg"), 0o644); err != nil {
		t.Fatal(err)
	}
	originalKernel := nvidiaKernel
	nvidiaKernel = func(context.Context) (string, error) { return "6.12.90-Unraid", nil }
	t.Cleanup(func() { nvidiaKernel = originalKernel })
	live := &Live{bootDir: boot}
	status, err := live.NvidiaStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.PluginInstalled || status.SelectedVersion != "latest_prb" || !status.UpdateCheck {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.DownloadedVersion != "580.76.05" {
		t.Fatalf("unexpected downloaded version: %+v", status)
	}
}
