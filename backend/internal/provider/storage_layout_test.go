package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"aurora/internal/model"
)

func TestStorageLayoutRoundTrip(t *testing.T) {
	live := &Live{dataDir: t.TempDir()}
	initial, err := live.StorageLayout(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(initial.Views) != 1 || initial.Columns != 2 || initial.Views[0].Orientation != "vertical" || initial.Views[0].Bays != defaultStorageBays {
		t.Fatalf("unexpected default layout: %+v", initial)
	}

	want := model.StorageLayout{Columns: 2, Views: []model.StorageView{
		{ID: "tower", Name: "Tower", Orientation: "vertical", Column: 0, Bays: 16, Assignments: map[string]int{"parity": 0, "disk1": 1}},
		{ID: "rack", Name: "Rack", Orientation: "horizontal", Column: 1, Bays: 8, Assignments: map[string]int{"cache": 0}},
		{ID: "nas", Name: "NAS", Orientation: "nas", Column: 1, Bays: 6, Assignments: map[string]int{"disk2": 0}},
	}}
	if err := live.SaveStorageLayout(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	got, err := live.StorageLayout(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Views) != 3 || got.Columns != 2 || got.Views[0].Bays != 16 || got.Views[1].Orientation != "horizontal" || got.Views[2].Bays != 6 {
		t.Fatalf("unexpected layout: %+v", got)
	}
	if got.Views[0].Assignments["parity"] != 0 || got.Views[1].Assignments["cache"] != 0 {
		t.Fatalf("unexpected assignments: %+v", got.Views)
	}
	if got.Views[1].Column != 1 || got.Views[2].Column != 1 {
		t.Fatalf("unexpected view columns: %+v", got.Views)
	}
}

func TestStorageLayoutMigratesLegacyFile(t *testing.T) {
	dir := t.TempDir()
	raw := []byte(`{"bays":12,"assignments":{"parity":0,"disk1":1}}`)
	if err := os.WriteFile(filepath.Join(dir, "storage-layout.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	live := &Live{dataDir: dir}
	layout, err := live.StorageLayout(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(layout.Views) != 1 || layout.Columns != 2 || layout.Views[0].Bays != 12 || layout.Views[0].Assignments["disk1"] != 1 {
		t.Fatalf("legacy layout was not migrated: %+v", layout)
	}
}

func TestStorageLayoutRejectsInvalidAssignments(t *testing.T) {
	live := &Live{dataDir: t.TempDir()}
	cases := []model.StorageLayout{
		{Columns: 2, Views: []model.StorageView{{ID: "a", Name: "A", Orientation: "vertical", Bays: 8, Assignments: map[string]int{"disk1": 2, "disk2": 2}}}},
		{Columns: 2, Views: []model.StorageView{{ID: "a", Name: "A", Orientation: "vertical", Bays: 8, Assignments: map[string]int{"disk1": 8}}}},
		{Columns: 2, Views: []model.StorageView{
			{ID: "a", Name: "A", Orientation: "vertical", Bays: 8, Assignments: map[string]int{"disk1": 0}},
			{ID: "b", Name: "B", Orientation: "horizontal", Bays: 8, Assignments: map[string]int{"disk1": 0}},
		}},
		{Columns: 2, Views: []model.StorageView{{ID: "nas", Name: "NAS", Orientation: "nas", Bays: 12}}},
		{Columns: 2, Views: []model.StorageView{{ID: "bad", Name: "Bad", Orientation: "vertical", Column: 2}}},
		{Columns: 4, Views: []model.StorageView{{ID: "a", Name: "A", Orientation: "vertical"}}},
	}
	for index, layout := range cases {
		if err := live.SaveStorageLayout(context.Background(), layout); err == nil {
			t.Fatalf("invalid layout %d was accepted", index)
		}
	}
}
