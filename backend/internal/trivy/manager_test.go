package trivy

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurora/internal/model"
)

func TestParseReportCountsAndSorts(t *testing.T) {
	raw := []byte(`{
	  "Results": [
	    {
	      "Target": "app/lib",
	      "Type": "node-pkg",
	      "Vulnerabilities": [
	        {"VulnerabilityID":"CVE-2026-0002","PkgName":"lib","InstalledVersion":"1.0","FixedVersion":"1.1","Severity":"MEDIUM","Title":"Medium issue","PrimaryURL":"https://example.test/2"},
	        {"VulnerabilityID":"CVE-2026-0001","PkgName":"lib","InstalledVersion":"1.0","FixedVersion":"1.1","Severity":"CRITICAL","Title":"Critical issue","PrimaryURL":"https://example.test/1"},
	        {"VulnerabilityID":"CVE-2026-0003","PkgName":"lib","InstalledVersion":"1.0","Severity":"HIGH","Title":"High issue"}
	      ]
	    },
	    {
	      "Target": "os",
	      "Type": "debian",
	      "Vulnerabilities": [
	        {"VulnerabilityID":"CVE-2026-0004","PkgName":"pkg","InstalledVersion":"2.0","Severity":"LOW","Title":"Low issue"},
	        {"VulnerabilityID":"CVE-2026-0005","PkgName":"pkg","InstalledVersion":"2.0","Severity":"UNKNOWN","Title":"Unknown issue"}
	      ]
	    }
	  ]
	}`)
	manager := New(t.TempDir())
	detail, err := manager.parseReport(raw, "repo:tag", "sha256:abc", "0.76.1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Critical != 1 || detail.High != 1 || detail.Medium != 1 || detail.Low != 1 || detail.Unknown != 1 {
		t.Fatalf("unexpected counts: %+v", detail.ContainerSecurity)
	}
	if detail.Total != 5 || len(detail.Vulnerabilities) != 5 {
		t.Fatalf("unexpected totals: %+v", detail)
	}
	if detail.Vulnerabilities[0].Severity != "CRITICAL" || detail.Vulnerabilities[0].ID != "CVE-2026-0001" {
		t.Fatalf("vulnerabilities are not sorted by severity: %+v", detail.Vulnerabilities)
	}
	if detail.Vulnerabilities[0].Target != "app/lib" || detail.Vulnerabilities[0].Type != "node-pkg" {
		t.Fatalf("missing target metadata: %+v", detail.Vulnerabilities[0])
	}
}

func TestSaveScanAndRead(t *testing.T) {
	manager := New(t.TempDir())
	now := time.Now()
	detail := model.ContainerSecurityDetail{
		ContainerSecurity: model.ContainerSecurity{
			Enabled: true, Available: true, Critical: 2, High: 1, Total: 3,
			ScannedAt: &now, TrivyVersion: "0.76.1",
		},
		Vulnerabilities: []model.ContainerVulnerability{{
			ID: "CVE-1", Package: "openssl", Severity: "CRITICAL", Title: "issue",
		}},
	}
	manager.saveScan(scanKey("repo:tag", "sha256:abc"), "repo:tag", "sha256:abc", detail)

	summary := manager.Security("repo:tag", "sha256:abc", true)
	if !summary.Available || summary.Critical != 2 || summary.High != 1 || summary.Total != 3 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	loaded := manager.Detail("repo:tag", "sha256:abc", true)
	if len(loaded.Vulnerabilities) != 1 || loaded.Vulnerabilities[0].ID != "CVE-1" {
		t.Fatalf("unexpected detail: %+v", loaded)
	}
	if manager.Security("repo:tag", "sha256:abc", false).Enabled {
		t.Fatal("disabled scanning should return an empty, disabled summary")
	}
}

func TestSaveScanNormalizesEmptyVulnerabilities(t *testing.T) {
	manager := New(t.TempDir())
	key := scanKey("repo:tag", "sha256:abc")
	manager.saveScan(key, "repo:tag", "sha256:abc", model.ContainerSecurityDetail{
		ContainerSecurity: model.ContainerSecurity{Enabled: true, Available: true},
	})
	raw, err := os.ReadFile(manager.detailPath(key))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"vulnerabilities": null`) {
		t.Fatalf("empty findings must be stored as an array: %s", raw)
	}
	if manager.Detail("repo:tag", "sha256:abc", true).Vulnerabilities == nil {
		t.Fatal("detail should expose an empty finding list, not null")
	}
}

func TestStatusMissingBinary(t *testing.T) {
	manager := New(t.TempDir())
	status := manager.Status()
	if status.Installed || status.DBReady {
		t.Fatalf("empty directory should not report an installation: %+v", status)
	}
	if manager.EnqueueScan("repo:tag", "sha256:abc", false) {
		t.Fatal("a scan must not be queued without Trivy and its database")
	}
}

func TestStartInstallRejectsInvalidVersion(t *testing.T) {
	manager := New(t.TempDir())
	if err := manager.StartInstall("../../etc/passwd"); err == nil {
		t.Fatal("invalid version was accepted")
	}
}

func TestScanKeyIgnoresTagWhenImageIDExists(t *testing.T) {
	first := scanKey("repo:tag", "sha256:abc")
	second := scanKey("repo:other", "sha256:abc")
	if first != second {
		t.Fatal("image ID must be the stable scan key")
	}
	if first == scanKey("repo:tag", "sha256:def") {
		t.Fatal("different image IDs must produce different keys")
	}
}

func TestAssetNameMatchesReleaseConvention(t *testing.T) {
	asset, err := assetName("0.76.1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(asset, "trivy_0.76.1_Linux-") || !strings.HasSuffix(asset, ".tar.gz") {
		t.Fatalf("unexpected asset name %q", asset)
	}
}

func TestStatePersistence(t *testing.T) {
	dir := t.TempDir()
	manager := New(dir)
	manager.mu.Lock()
	manager.state.Version = "0.76.1"
	manager.state.DBUpdatedAt = time.Now()
	manager.mu.Unlock()
	if err := manager.persistState(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state diskState
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	if state.Version != "0.76.1" {
		t.Fatalf("unexpected persisted state: %+v", state)
	}
	reloaded := New(dir)
	if reloaded.Status().Version != "0.76.1" {
		t.Fatalf("state was not reloaded: %+v", reloaded.Status())
	}
}

func TestDownloadAndInstallVerifiesChecksumAndExtractsBinary(t *testing.T) {
	asset, err := assetName("9.9.9")
	if err != nil {
		t.Fatal(err)
	}
	binaryContent := []byte("#!/bin/sh\necho fake-trivy\n")
	var archive bytes.Buffer
	gzipWriter := gzip.NewWriter(&archive)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{Name: "trivy", Mode: 0o755, Size: int64(len(binaryContent))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(binaryContent); err != nil {
		t.Fatal(err)
	}
	tarWriter.Close()
	gzipWriter.Close()
	sum := sha256.Sum256(archive.Bytes())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v9.9.9/" + asset:
			_, _ = w.Write(archive.Bytes())
		case "/v9.9.9/trivy_9.9.9_checksums.txt":
			_, _ = fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), asset)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	previous := releasesDownloadBase
	releasesDownloadBase = server.URL
	t.Cleanup(func() { releasesDownloadBase = previous })
	runtimeDir := t.TempDir()
	t.Setenv("AURORA_TRIVY_RUNTIME_DIR", runtimeDir)

	manager := New(t.TempDir())
	if err := manager.downloadAndInstall(context.Background(), "9.9.9"); err != nil {
		t.Fatal(err)
	}
	installed, err := os.ReadFile(manager.binary)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(installed, binaryContent) {
		t.Fatalf("unexpected binary content: %q", installed)
	}
	status := manager.Status()
	if status.Version != "9.9.9" || !status.RuntimeReady {
		t.Fatalf("unexpected status: %+v", status)
	}
	if _, err := os.Stat(filepath.Join(runtimeDir, "trivy")); err != nil {
		t.Fatalf("runtime binary was not materialized: %v", err)
	}
}

func TestScanReadyRequiresDatabaseAndIdleManager(t *testing.T) {
	manager := New(t.TempDir())
	manager.runtimeReady = true
	if manager.scanReady() {
		t.Fatal("scan must not be ready before the database is downloaded")
	}
	if err := os.MkdirAll(filepath.Dir(manager.dbPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manager.dbPath(), []byte("fake db"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manager.dbMetadataPath(), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if manager.scanReady() {
		t.Fatal("scan must not be ready while the Java database is missing")
	}
	if err := os.MkdirAll(filepath.Dir(manager.javaDBPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manager.javaDBPath(), []byte("fake java db"), 0o644); err != nil {
		t.Fatal(err)
	}
	if manager.scanReady() {
		t.Fatal("scan must not be ready while the Java metadata is missing")
	}
	if err := os.WriteFile(manager.javaMetadataPath(), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !manager.scanReady() {
		t.Fatal("scan should be ready once both databases and metadata exist")
	}
	manager.updatingDB = true
	if manager.scanReady() {
		t.Fatal("scan must not start while the database is being updated")
	}
	manager.updatingDB = false
	manager.installing = true
	if manager.scanReady() {
		t.Fatal("scan must not start while Trivy is being installed")
	}
}

func TestScansReportsQueuedAndRunning(t *testing.T) {
	base := time.Unix(1000, 0)
	manager := &Manager{scanning: map[string]scanActivity{
		"running": {image: "linuxserver/jackett:latest", started: base, running: true},
		"queued":  {image: "bitnami/mongodb:latest", started: base.Add(time.Minute)},
	}}
	scans := manager.Scans()
	if len(scans) != 2 {
		t.Fatalf("scans = %+v", scans)
	}
	if scans[0].Image != "linuxserver/jackett:latest" || !scans[0].Running || scans[0].Status != "running" {
		t.Fatalf("oldest scan should come first: %+v", scans)
	}
	if scans[1].Image != "bitnami/mongodb:latest" || scans[1].Running || scans[1].Status != "queued" {
		t.Fatalf("queued scan should be reported as queued: %+v", scans)
	}
}

func TestRecentScansIncludesFinished(t *testing.T) {
	now := time.Now()
	manager := &Manager{scanning: map[string]scanActivity{}, recentScans: []scanRecord{
		{image: "redis:7", started: now.Add(-time.Minute), finished: now, status: "completed"},
		{image: "old:1", started: now.Add(-2 * time.Hour), finished: now.Add(-time.Hour), status: "completed"},
		{image: "broken:1", started: now.Add(-2 * time.Minute), finished: now.Add(-time.Minute), status: "failed", err: "boom"},
	}}
	scans := manager.RecentScans()
	if len(scans) != 3 {
		t.Fatalf("finished scans = %+v", scans)
	}
	if scans[1].Image != "broken:1" || scans[1].Status != "failed" || scans[1].Error != "boom" {
		t.Fatalf("failed scan missing: %+v", scans)
	}
	if scans[2].Image != "redis:7" || scans[2].Status != "completed" || scans[2].FinishedAt == nil {
		t.Fatalf("completed scan missing: %+v", scans)
	}
	if active := manager.Scans(); len(active) != 0 {
		t.Fatalf("finished scans must not be active: %+v", active)
	}
}

func TestScanCachesFollowFileChanges(t *testing.T) {
	manager := New(t.TempDir())
	key := scanKey("repo:tag", "sha256:abc")
	manager.saveScan(key, "repo:tag", "sha256:abc", model.ContainerSecurityDetail{
		ContainerSecurity: model.ContainerSecurity{Enabled: true, Available: true},
		Vulnerabilities:   []model.ContainerVulnerability{{ID: "CVE-1", Package: "openssl"}},
	})
	if got := manager.Detail("repo:tag", "sha256:abc", true).Vulnerabilities; len(got) != 1 {
		t.Fatalf("unexpected cached findings: %+v", got)
	}

	// Simulate an external rewrite (for example a crash-safe replacement):
	// the size changes, so the next read must bypass the cache.
	replacement := []byte(`{"vulnerabilities":[{"id":"CVE-2","package":"curl","severity":"HIGH"}]}`)
	if err := os.WriteFile(manager.detailPath(key), replacement, 0o644); err != nil {
		t.Fatal(err)
	}
	got := manager.Detail("repo:tag", "sha256:abc", true).Vulnerabilities
	if len(got) != 1 || got[0].ID != "CVE-2" {
		t.Fatalf("detail cache was not invalidated: %+v", got)
	}
}
