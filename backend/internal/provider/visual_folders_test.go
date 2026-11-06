package provider

import (
	"context"
	"testing"

	"aurora/internal/model"
)

func TestVisualFoldersRoundTrip(t *testing.T) {
	live := &Live{configRoot: t.TempDir()}
	layout := model.VisualFolders{
		Folders: []model.VisualFolder{
			{ID: "second", Name: "Second", Order: 5},
			{ID: "first", Name: "First", Order: 1},
		},
		Assignments: map[string]string{"plex": "first", "ghost": "missing", "": "first"},
	}
	if err := live.SaveVisualFolders(context.Background(), "containers", layout); err != nil {
		t.Fatal(err)
	}
	got, err := live.VisualFolders(context.Background(), "containers")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Folders) != 2 || got.Folders[0].ID != "first" || got.Folders[0].Order != 0 || got.Folders[1].Order != 1 {
		t.Fatalf("unexpected folder order: %+v", got.Folders)
	}
	if len(got.Assignments) != 1 || got.Assignments["plex"] != "first" {
		t.Fatalf("unexpected assignments: %+v", got.Assignments)
	}
	if got.Version != 1 {
		t.Fatalf("unexpected version: %d", got.Version)
	}
}

func TestVisualFoldersDefaultsEmpty(t *testing.T) {
	live := &Live{configRoot: t.TempDir()}
	got, err := live.VisualFolders(context.Background(), "vms")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Folders) != 0 || len(got.Assignments) != 0 || got.Version != 1 {
		t.Fatalf("unexpected default layout: %+v", got)
	}
}

func TestVisualFoldersRejectDuplicates(t *testing.T) {
	live := &Live{configRoot: t.TempDir()}
	duplicateID := model.VisualFolders{Folders: []model.VisualFolder{
		{ID: "a", Name: "One"}, {ID: "a", Name: "Two"},
	}}
	if err := live.SaveVisualFolders(context.Background(), "containers", duplicateID); err == nil {
		t.Fatal("duplicate IDs must be rejected")
	}
	duplicateName := model.VisualFolders{Folders: []model.VisualFolder{
		{ID: "a", Name: "Media"}, {ID: "b", Name: "media"},
	}}
	if err := live.SaveVisualFolders(context.Background(), "containers", duplicateName); err == nil {
		t.Fatal("duplicate names must be rejected")
	}
}

func TestVisualFoldersUnknownKind(t *testing.T) {
	live := &Live{configRoot: t.TempDir()}
	if _, err := live.VisualFolders(context.Background(), "other"); err == nil {
		t.Fatal("unknown kind must be rejected")
	}
}
