package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareOSVersions(t *testing.T) {
	cases := []struct {
		left, right string
		want        int
	}{
		{"7.2.0", "7.2.0", 0},
		{"7.2.0", "7.1.4", 1},
		{"7.1.4", "7.2.0", -1},
		{"7.2.0", "7.2.0-rc.2", 1},
		{"7.2.0-rc.2", "7.2.0-rc.1", 1},
		{"7.10.0", "7.9.9", 1},
	}
	for _, testCase := range cases {
		if got := compareOSVersions(testCase.left, testCase.right); got != testCase.want {
			t.Fatalf("compareOSVersions(%q, %q) = %d, want %d", testCase.left, testCase.right, got, testCase.want)
		}
	}
}

func TestChangesVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "changes.txt")
	content := "# Unraid OS\n#\n# Version 7.3.4 2026-10-22\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	version, date := changesVersion(path)
	if version != "7.3.4" || date != "2026-10-22" {
		t.Fatalf("unexpected changes details: %q %q", version, date)
	}
}

func TestRestorePreviousOS(t *testing.T) {
	boot := t.TempDir()
	previous := filepath.Join(boot, "previous")
	if err := os.MkdirAll(previous, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(boot, "bzroot"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(boot, "bzroot-gui"), []byte("new-gui"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(previous, "bzroot"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(previous, "bzroot-gui"), []byte("old-gui"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(previous, "changes.txt"), []byte("# Version 6.12.3 2023-01-01\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	live := &Live{bootDir: boot, stateDir: t.TempDir()}
	version, err := live.RestorePreviousOS(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if version != "6.12.3" {
		t.Fatalf("unexpected version %q", version)
	}
	raw, err := os.ReadFile(filepath.Join(boot, "bzroot"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "old" {
		t.Fatalf("expected previous bzroot, got %q", raw)
	}
	if _, err := os.Stat(filepath.Join(previous, "bzroot")); !os.IsNotExist(err) {
		t.Fatal("previous bzroot should have been moved")
	}
	staging, _ := filepath.Glob(filepath.Join(boot, "deletemedowngrade.*"))
	if len(staging) != 0 {
		t.Fatalf("staging directory was not cleaned up: %v", staging)
	}
}

func TestCheckOSUpdateUsesReleaseService(t *testing.T) {
	sha := strings.Repeat("a", 64)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/os":
			if r.URL.Query().Get("branch") != "stable" {
				t.Errorf("unexpected branch %q", r.URL.Query().Get("branch"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"version": "7.4.0", "name": "Unraid 7.4.0", "date": "2026-11-01",
				"isNewer": true, "isEligible": true, "sha256": sha,
				"changelog": "https://example.test/changelog", "changelogPretty": "https://example.test/notes",
			})
		case r.URL.Path == "/sha256/"+sha:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"version": "7.4.0", "name": "Unraid 7.4.0", "date": "2026-11-01",
				"plugin_url": server.URL + "/unRAIDServer.plg", "sha256": sha,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	previousBase := osReleasesBaseURL
	osReleasesBaseURL = server.URL
	t.Cleanup(func() { osReleasesBaseURL = previousBase })

	boot := t.TempDir()
	live := &Live{bootDir: boot, stateDir: t.TempDir(), configRoot: filepath.Join(t.TempDir(), "aurora")}
	status, err := live.CheckOSUpdate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.UpdateAvailable || status.Latest != "7.4.0" || status.CheckError != "" {
		t.Fatalf("unexpected status: %+v", status)
	}
	cache, err := live.loadOSCache()
	if err != nil {
		t.Fatal(err)
	}
	if cache.PluginURL != server.URL+"/unRAIDServer.plg" {
		t.Fatalf("unexpected plugin URL %q", cache.PluginURL)
	}
	cachedStatus, err := live.OSStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cachedStatus.Latest != "7.4.0" || cachedStatus.CheckedAt == nil {
		t.Fatalf("expected cached update status, got %+v", cachedStatus)
	}
	if err := validateReleaseURL(server.URL + "/unRAIDServer.plg"); err != nil {
		t.Fatalf("release URL should be accepted: %v", err)
	}
	if err := validateReleaseURL("https://evil.example/unRAIDServer.plg"); err == nil {
		t.Fatal("external release URL must be rejected")
	}
}

func TestUnraidAPIStatusParsing(t *testing.T) {
	cases := map[string]struct {
		running bool
		state   string
	}{
		"online":    {true, "online"},
		"errored":   {false, "errored"},
		"stopped":   {false, "stopped"},
		"unknown":   {false, "unknown"},
		"launching": {false, "launching"},
	}
	for output, expected := range cases {
		running, state := interpretUnraidAPIStatus(output)
		if running != expected.running || state != expected.state {
			t.Fatalf("interpret(%q) = %v %q, want %v %q", output, running, state, expected.running, expected.state)
		}
	}
}
