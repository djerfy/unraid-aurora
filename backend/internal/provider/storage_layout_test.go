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
	if initial.Version != 2 || len(initial.Views) != 1 || initial.Views[0].Kind != "tower" {
		t.Fatalf("unexpected default layout: %+v", initial)
	}
	if len(initial.Views[0].Sections) != 1 || initial.Views[0].Sections[0].Rows == 0 {
		t.Fatalf("default section missing: %+v", initial.Views[0])
	}

	want := model.StorageLayout{Version: 2, Views: []model.StorageView{
		{
			ID: "tower", Name: "Tower", Kind: "tower", Size: "m", X: 0, Y: 0, W: 6, H: 20,
			Sections: []model.StorageSection{
				{ID: "data", Name: "Data", Color: "#4aa3ff", Media: "hdd", Rows: 2, Cols: 2, Assignments: map[string]int{"parity": 0, "disk1": 1}},
				{ID: "cache", Name: "Cache", Color: "#3ddc97", Media: "ssd", Rows: 1, Cols: 2, Assignments: map[string]int{"cache": 0}},
			},
		},
		{
			ID: "rack", Name: "Rack", Kind: "server", Size: "l", X: 0, Y: 22, W: 12, H: 18,
			Sections: []model.StorageSection{
				{ID: "data", Name: "Data", Color: "#4aa3ff", Media: "any", Rows: 2, Cols: 4, Assignments: map[string]int{"disk2": 0}},
			},
		},
	}}
	if err := live.SaveStorageLayout(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	got, err := live.StorageLayout(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 2 || len(got.Views) != 2 || got.Views[1].Kind != "server" {
		t.Fatalf("unexpected layout: %+v", got)
	}
	if got.Views[0].Sections[0].Assignments["parity"] != 0 || got.Views[0].Sections[1].Assignments["cache"] != 0 {
		t.Fatalf("unexpected assignments: %+v", got.Views)
	}
}

func TestStorageLayoutMigratesLegacyFile(t *testing.T) {
	dir := t.TempDir()
	raw := []byte(`{"columns":2,"views":[{"id":"tower","name":"Tower","orientation":"vertical","column":0,"bays":8,"assignments":{"parity":0,"disk1":1}},{"id":"rack","name":"Rack","orientation":"horizontal","column":1,"bays":12,"assignments":{"disk2":0}}]}`)
	if err := os.WriteFile(filepath.Join(dir, "storage-layout.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	live := &Live{dataDir: dir}
	layout, err := live.StorageLayout(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if layout.Version != 2 || len(layout.Views) != 2 {
		t.Fatalf("legacy layout was not migrated: %+v", layout)
	}
	if layout.Views[0].Kind != "tower" || layout.Views[1].Kind != "server" {
		t.Fatalf("legacy kinds not mapped: %+v", layout.Views)
	}
	if layout.Views[0].Sections[0].Assignments["disk1"] != 1 {
		t.Fatalf("legacy assignments not preserved: %+v", layout.Views[0])
	}
}

func TestStorageLayoutMigratesSingleViewFile(t *testing.T) {
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
	if len(layout.Views) != 1 || layout.Views[0].Sections[0].Assignments["disk1"] != 1 {
		t.Fatalf("single-view layout was not migrated: %+v", layout)
	}
}

func TestStorageLayoutValidation(t *testing.T) {
	live := &Live{dataDir: t.TempDir()}
	base := func() model.StorageView {
		return model.StorageView{
			ID: "tower", Name: "Tower", Kind: "tower", Size: "m", X: 0, Y: 0, W: 6, H: 20,
			Sections: []model.StorageSection{{
				ID: "data", Name: "Data", Color: "#fff", Media: "any", Rows: 2, Cols: 2,
				Assignments: map[string]int{"disk1": 0},
			}},
		}
	}
	cases := map[string]model.StorageLayout{
		"overlap": {Version: 2, Views: []model.StorageView{
			base(),
			{ID: "rack", Name: "Rack", Kind: "server", Size: "m", X: 0, Y: 0, W: 9, H: 14, Sections: base().Sections},
		}},
		"duplicate slot": {Version: 2, Views: []model.StorageView{func() model.StorageView {
			view := base()
			view.Sections[0].Assignments = map[string]int{"disk1": 0, "disk2": 0}
			return view
		}()}},
		"slot outside section": {Version: 2, Views: []model.StorageView{func() model.StorageView {
			view := base()
			view.Sections[0].Assignments = map[string]int{"disk1": 4}
			return view
		}()}},
		"duplicate disk across sections": {Version: 2, Views: []model.StorageView{func() model.StorageView {
			view := base()
			view.Sections = append(view.Sections, model.StorageSection{
				ID: "cache", Name: "Cache", Color: "#fff", Media: "any", Rows: 1, Cols: 2,
				Assignments: map[string]int{"disk1": 0},
			})
			return view
		}()}},
		"outside board": {Version: 2, Views: []model.StorageView{func() model.StorageView {
			view := base()
			view.X = 8
			return view
		}()}},
	}
	for name, layout := range cases {
		if err := live.SaveStorageLayout(context.Background(), layout); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
}

func TestStorageLayoutKeepsCustomSize(t *testing.T) {
	live := &Live{dataDir: t.TempDir()}
	layout := model.StorageLayout{Version: 2, Views: []model.StorageView{{
		ID: "tower", Name: "Tower", Kind: "tower", Size: "custom", X: 1, Y: 2, W: 7, H: 23,
		Sections: []model.StorageSection{{
			ID: "data", Name: "Data", Color: "#fff", Media: "any", Rows: 2, Cols: 2,
			Assignments: map[string]int{"disk1": 0},
		}},
	}}}
	if err := live.SaveStorageLayout(context.Background(), layout); err != nil {
		t.Fatal(err)
	}
	got, err := live.StorageLayout(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	view := got.Views[0]
	if view.Size != "custom" || view.X != 1 || view.Y != 2 || view.W != 7 || view.H != 23 {
		t.Fatalf("custom geometry was not preserved: %+v", view)
	}
}

func TestStorageLayoutRepairsInvalidGeometry(t *testing.T) {
	dir := t.TempDir()
	raw := []byte(`{"version":2,"views":[{"id":"tower","name":"Tower","kind":"tower","size":"custom","x":0,"y":0,"w":0,"h":0,"sections":[{"id":"data","name":"Data","color":"#fff","media":"any","rows":2,"cols":2,"assignments":{}}]}]}`)
	if err := os.WriteFile(filepath.Join(dir, "storage-layout.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	live := &Live{dataDir: dir}
	got, err := live.StorageLayout(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	view := got.Views[0]
	if view.W != 6 || view.H != 20 {
		t.Fatalf("invalid geometry was not repaired: %+v", view)
	}
}
