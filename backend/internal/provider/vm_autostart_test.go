package provider

import (
	"context"
	"testing"
)

func TestVMAutostartConfigRoundTrip(t *testing.T) {
	live := &Live{configRoot: t.TempDir()}
	config := vmAutostartConfig{
		Order: []string{"storage", "media"}, Managed: true,
		ManagedVMs: []string{"storage"}, AppliedFor: "array-start-1",
	}
	if err := live.writeVMAutostartConfig(config); err != nil {
		t.Fatal(err)
	}
	loaded, err := live.loadVMAutostartConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Managed || len(loaded.Order) != 2 || loaded.AppliedFor != "array-start-1" {
		t.Fatalf("unexpected config: %+v", loaded)
	}
	if len(loaded.ManagedVMs) != 1 || loaded.ManagedVMs[0] != "storage" {
		t.Fatalf("unexpected managed VMs: %+v", loaded.ManagedVMs)
	}
}

func TestMarkVMAutostartApplied(t *testing.T) {
	live := &Live{configRoot: t.TempDir()}
	if err := live.writeVMAutostartConfig(vmAutostartConfig{Order: []string{"media"}, Managed: true}); err != nil {
		t.Fatal(err)
	}
	live.markVMAutostartApplied("array-start-2")
	loaded, err := live.loadVMAutostartConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AppliedFor != "array-start-2" {
		t.Fatalf("applied marker not written: %+v", loaded)
	}
	// A second call for the same array start must be a no-op.
	live.markVMAutostartApplied("array-start-2")
	again, _ := live.loadVMAutostartConfig(context.Background())
	if again.AppliedFor != "array-start-2" {
		t.Fatalf("marker changed unexpectedly: %+v", again)
	}
}
