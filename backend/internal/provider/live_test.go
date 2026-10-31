package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"aurora/internal/model"
)

func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fixtureHost(t *testing.T) (stateDir, bootDir, dataDir string) {
	t.Helper()
	stateDir = t.TempDir()
	bootDir = t.TempDir()
	dataDir = t.TempDir()

	writeFixture(t, stateDir, "var.ini", `NAME="Vault"
mdState="STARTED"
mdColor="green-on"
mdNumDisks="2"
mdResync="0"
regTy="pro"
flashGUID="0781-5583"
`)
	writeFixture(t, stateDir, "disks.ini", `["parity"]
device="sda"
type="Parity"
fsType="xfs"
fsSize="19531250000"
fsUsed="0"
temp="38"
smart="passed"
color="green-on"
rotational="1"

["disk1"]
device="sdc"
type="Data"
fsType="xfs"
fsSize="17578125000"
fsUsed="8789062500"
temp="35"
smart="passed"
color="green-on"
rotational="1"
deviceSb="WDC WD180EDGZ-11"
serial="2LG7K8PZ"

["disk2"]
device="sdd"
type="Data"
fsType="xfs"
fsSize="7812500000"
fsUsed="1200000000"
temp="41"
warning="8"
color="yellow-on"
rotational="1"
smart="warning"
`)
	writeFixture(t, stateDir, "shares.ini", `["media"]
free="3906250000"
size="7812500000"
comment="Movies and series"
allocator="highwater"
splitLevel="auto"
useCache="no"
security="secure"
export="e"
exportNFS="yes"

["appdata"]
free="1953125000"
size="3906250000"
comment="Container state"
allocator="highwater"
useCache="only"
cachePool="cache"
`)
	if err := os.MkdirAll(filepath.Join(bootDir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(bootDir, "config"), "parity-checks.log", `30-09-2026 18:16:29|104400|174.2|0|0|check|80000000000000
`)
	return stateDir, bootDir, dataDir
}

func TestLiveReadsUnraidState(t *testing.T) {
	stateDir, bootDir, dataDir := fixtureHost(t)
	l, err := NewLive(stateDir, bootDir, filepath.Join(t.TempDir(), "absent.sock"), dataDir)
	if err != nil {
		t.Fatalf("NewLive: %v", err)
	}
	defer l.Close()
	ctx := context.Background()

	sys, err := l.System(ctx)
	if err != nil {
		t.Fatalf("system: %v", err)
	}
	if sys.Name != "Vault" || sys.License != "Unraid Pro" {
		t.Fatalf("unexpected system identity: %+v", sys)
	}

	arr, err := l.Array(ctx)
	if err != nil {
		t.Fatalf("array: %v", err)
	}
	if arr.State != "STARTED" || !arr.Parity.Configured {
		t.Fatalf("unexpected array: %+v", arr)
	}
	if arr.Disks != 2 {
		t.Fatalf("data disks = %d, want 2", arr.Disks)
	}
	// fsSize is KiB in Unraid state files.
	if arr.Total != (17578125000+7812500000)*1024 {
		t.Fatalf("array total = %d", arr.Total)
	}

	disks, err := l.Disks(ctx)
	if err != nil {
		t.Fatalf("disks: %v", err)
	}
	if len(disks) != 4 {
		t.Fatalf("expected parity + 2 data + flash, got %d", len(disks))
	}
	if disks[0].Name != "disk1" && disks[0].Name != "parity" {
		t.Fatalf("unexpected disk order: %+v", disks)
	}
	var foundWarning bool
	for _, disk := range disks {
		if disk.Name == "disk2" && disk.Smart == "warning" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Fatal("disk2 SMART warning was not mapped")
	}

	shares, err := l.Shares(ctx)
	if err != nil {
		t.Fatalf("shares: %v", err)
	}
	if len(shares) != 2 {
		t.Fatalf("shares = %d, want 2", len(shares))
	}
	if shares[0].Name != "appdata" && shares[0].Name != "media" {
		t.Fatalf("unexpected share order: %+v", shares)
	}

	history, err := l.ParityHistory(ctx)
	if err != nil {
		t.Fatalf("parity history: %v", err)
	}
	if len(history) != 1 || history[0].Speed != 174.2 {
		t.Fatalf("unexpected parity history: %+v", history)
	}
}

func TestLiveSnapshotNeverContainsNullSlices(t *testing.T) {
	stateDir, bootDir, dataDir := fixtureHost(t)
	l, err := NewLive(stateDir, bootDir, filepath.Join(t.TempDir(), "absent.sock"), dataDir)
	if err != nil {
		t.Fatalf("NewLive: %v", err)
	}
	defer l.Close()

	snap, err := l.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	assertSlice := func(name string, length int) {
		t.Helper()
		if length < 0 {
			t.Fatalf("%s is nil", name)
		}
	}
	assertSlice("disks", len(snap.Disks))
	assertSlice("shares", len(snap.Shares))
	assertSlice("pools", len(snap.Pools))
	assertSlice("containers", len(snap.Containers))
	assertSlice("vms", len(snap.VMs))
	assertSlice("users", len(snap.Users))
	assertSlice("plugins", len(snap.Plugins))
	assertSlice("notifications", len(snap.Notifications))
	assertSlice("history", len(snap.History))
	assertSlice("network.interfaces", len(snap.Network.Interfaces))
	assertSlice("network.wireGuard.peers", len(snap.Network.WireGuard.Peers))
	assertSlice("network.services", len(snap.Network.Services))
	if snap.System.Load == nil || len(snap.System.Load) != 3 {
		t.Fatalf("load must be padded to three values: %#v", snap.System.Load)
	}
	for _, disk := range snap.Disks {
		if disk.Warnings == nil {
			t.Fatalf("disk %s has nil warnings", disk.Name)
		}
	}
}

func TestLiveSettingsPersistence(t *testing.T) {
	stateDir, bootDir, dataDir := fixtureHost(t)
	l, err := NewLive(stateDir, bootDir, filepath.Join(t.TempDir(), "absent.sock"), dataDir)
	if err != nil {
		t.Fatalf("NewLive: %v", err)
	}
	settings := model.Settings{Theme: "light", Accent: "cyan", Density: "compact", RefreshInterval: 5, TemperatureUnit: "celsius"}
	if err := l.SaveSettings(context.Background(), settings); err != nil {
		t.Fatalf("save: %v", err)
	}
	l.Close()

	reloaded, err := NewLive(stateDir, bootDir, filepath.Join(t.TempDir(), "absent.sock"), dataDir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	defer reloaded.Close()
	got, _ := reloaded.Settings(context.Background())
	if got.Theme != "light" || got.Accent != "cyan" || got.Density != "compact" {
		t.Fatalf("settings were not persisted: %+v", got)
	}
}

func TestNewLiveRequiresUnraidState(t *testing.T) {
	_, err := NewLive(t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "absent.sock"), t.TempDir())
	if err == nil {
		t.Fatal("expected an error when var.ini is absent")
	}
}
