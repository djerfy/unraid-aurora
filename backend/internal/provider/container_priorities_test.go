package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"aurora/internal/model"
)

func TestContainerPrioritiesRoundTrip(t *testing.T) {
	live := &Live{configRoot: t.TempDir()}
	live.dockerAutostartPath = filepath.Join(t.TempDir(), "autostart")
	if err := os.WriteFile(live.dockerAutostartPath, []byte("web\ndb\ncache\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	saved, err := live.SaveContainerPriorities(context.Background(), map[string]int{
		"db": 10, "cache": 20, "web": 90, "default": 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := saved["default"]; ok {
		t.Fatalf("priority 50 must not be persisted: %+v", saved)
	}

	got, err := live.ContainerPriorities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got["db"] != 10 || got["cache"] != 20 || got["web"] != 90 {
		t.Fatalf("unexpected priorities: %+v", got)
	}

	raw, err := os.ReadFile(live.dockerAutostartPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "db\ncache\nweb\n" {
		t.Fatalf("autostart order = %q", raw)
	}
}

func TestContainerPrioritiesValidation(t *testing.T) {
	live := &Live{configRoot: t.TempDir()}
	live.dockerAutostartPath = filepath.Join(t.TempDir(), "autostart")
	if _, err := live.SaveContainerPriorities(context.Background(), map[string]int{"bad/name": 10}); err == nil {
		t.Fatal("invalid names must be rejected")
	}
	if _, err := live.SaveContainerPriorities(context.Background(), map[string]int{"plex": 101}); err == nil {
		t.Fatal("out-of-range priorities must be rejected")
	}
}

func TestApplyContainerPriorities(t *testing.T) {
	containers := []model.Container{{Name: "plex"}, {Name: "db"}}
	applyContainerPriorities(containers, map[string]int{"db": 10})
	if containers[0].Priority != 50 || containers[1].Priority != 10 {
		t.Fatalf("unexpected priorities: %+v", containers)
	}
}

func TestSortAutostartEntries(t *testing.T) {
	sorted := sortAutostartEntries(
		[]string{"web", "db", "cache", "other"},
		map[string]int{"db": 10, "cache": 10},
	)
	want := []string{"db", "cache", "web", "other"}
	for index := range want {
		if sorted[index] != want[index] {
			t.Fatalf("sorted = %v, want %v", sorted, want)
		}
	}
}
