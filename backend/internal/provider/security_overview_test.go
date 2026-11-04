package provider

import (
	"testing"
	"time"

	"aurora/internal/model"
)

func TestBuildSecurityOverviewMergesSharedImages(t *testing.T) {
	scannedAt := time.Now().Add(-time.Hour)
	detail := model.ContainerSecurityDetail{
		ContainerSecurity: model.ContainerSecurity{
			Enabled: true, Available: true, ScannedAt: &scannedAt,
			Critical: 2, High: 1, Total: 3,
			Image: "repo:tag", ImageID: "sha256:shared",
		},
		Vulnerabilities: []model.ContainerVulnerability{
			{ID: "CVE-2", Package: "openssl", Severity: "CRITICAL"},
			{ID: "CVE-1", Package: "curl", Severity: "HIGH"},
			{ID: "CVE-2", Package: "openssl", Severity: "CRITICAL"},
		},
	}
	containers := []model.Container{
		{ID: "a", Name: "alpha", Image: "repo:tag", ImageID: "sha256:shared", State: "running"},
		{ID: "b", Name: "beta", Image: "repo:tag", ImageID: "sha256:shared", State: "exited"},
		{ID: "c", Name: "gamma", Image: "other:tag", ImageID: "sha256:other", State: "running"},
	}
	overview := buildSecurityOverview(true, containers, map[string]model.ContainerSecurityDetail{
		"a": detail,
		"b": detail,
	})

	if len(overview.Images) != 2 {
		t.Fatalf("expected two unique images, got %+v", overview.Images)
	}
	if len(overview.Images[0].Containers) != 2 {
		t.Fatalf("shared image should keep both containers: %+v", overview.Images[0])
	}
	if overview.Totals.Images != 2 || overview.Totals.Containers != 3 {
		t.Fatalf("unexpected image/container totals: %+v", overview.Totals)
	}
	if overview.Totals.ScannedImages != 1 || overview.Totals.ScannedContainers != 2 {
		t.Fatalf("shared image must not duplicate scan totals: %+v", overview.Totals)
	}
	if overview.Totals.Critical != 2 || overview.Totals.High != 1 || overview.Totals.Total != 3 {
		t.Fatalf("unexpected severity totals: %+v", overview.Totals)
	}
	if overview.Totals.LastScanAt == nil || !overview.Totals.LastScanAt.Equal(scannedAt) {
		t.Fatalf("unexpected last scan time: %+v", overview.Totals.LastScanAt)
	}
}

func TestBuildSecurityOverviewKeepsUnscannedImages(t *testing.T) {
	overview := buildSecurityOverview(true, []model.Container{
		{ID: "a", Name: "alpha", Image: "repo:tag", ImageID: "sha256:abc", State: "running"},
	}, nil)

	if len(overview.Images) != 1 {
		t.Fatalf("unscanned image should stay visible: %+v", overview)
	}
	if overview.Images[0].Security.Available || !overview.Images[0].Security.Enabled {
		t.Fatalf("unexpected unscanned state: %+v", overview.Images[0].Security)
	}
	if overview.Images[0].Vulnerabilities == nil {
		t.Fatal("an empty vulnerability list must be serialised as an array, not null")
	}
	if overview.Totals.ScannedImages != 0 || overview.Totals.Containers != 1 {
		t.Fatalf("unexpected unscanned totals: %+v", overview.Totals)
	}
}
