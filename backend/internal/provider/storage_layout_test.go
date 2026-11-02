package provider

import (
	"context"
	"testing"

	"aurora/internal/model"
)

func TestStorageLayoutRoundTrip(t *testing.T) {
	live := &Live{dataDir: t.TempDir()}
	initial, err := live.StorageLayout(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if initial.Bays != defaultStorageBays || len(initial.Assignments) != 0 {
		t.Fatalf("unexpected default layout: %+v", initial)
	}

	want := model.StorageLayout{Bays: 16, Assignments: map[string]int{"parity": 0, "disk1": 1}}
	if err := live.SaveStorageLayout(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	got, err := live.StorageLayout(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Bays != 16 || got.Assignments["parity"] != 0 || got.Assignments["disk1"] != 1 {
		t.Fatalf("unexpected layout: %+v", got)
	}
}

func TestStorageLayoutRejectsDuplicateBays(t *testing.T) {
	live := &Live{dataDir: t.TempDir()}
	layout := model.StorageLayout{Bays: 8, Assignments: map[string]int{"disk1": 2, "disk2": 2}}
	if err := live.SaveStorageLayout(context.Background(), layout); err == nil {
		t.Fatal("duplicate bay was accepted")
	}
	layout = model.StorageLayout{Bays: 8, Assignments: map[string]int{"disk1": 8}}
	if err := live.SaveStorageLayout(context.Background(), layout); err == nil {
		t.Fatal("out-of-range bay was accepted")
	}
}
