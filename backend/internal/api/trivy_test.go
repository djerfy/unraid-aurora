package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestTrivyStatusAndVersions(t *testing.T) {
	env := loginTestEnv(t, false)
	resp := env.request(t, http.MethodGet, "/api/v1/trivy", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("trivy status = %d", resp.StatusCode)
	}
	var status struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if status.Version != "0.76.1" {
		t.Fatalf("unexpected status: %+v", status)
	}

	resp = env.request(t, http.MethodGet, "/api/v1/trivy/versions", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("trivy versions = %d", resp.StatusCode)
	}
}

func TestTrivyMutations(t *testing.T) {
	env := loginTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	for _, path := range []string{"/api/v1/trivy/install", "/api/v1/trivy/db/update"} {
		resp := env.request(t, http.MethodPost, path, map[string]any{}, headers)
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("%s = %d", path, resp.StatusCode)
		}
		resp.Body.Close()
	}
	resp := env.request(t, http.MethodPost, "/api/v1/docker/containers/abc/scan", map[string]any{}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("container scan = %d", resp.StatusCode)
	}

	resp = env.request(t, http.MethodPost, "/api/v1/docker/scan-all", map[string]any{}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("scan all = %d", resp.StatusCode)
	}
}

func TestSecurityOverview(t *testing.T) {
	env := loginTestEnv(t, false)
	resp := env.request(t, http.MethodGet, "/api/v1/docker/security", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("security overview = %d", resp.StatusCode)
	}
	var overview struct {
		Enabled bool `json:"enabled"`
		Images  []struct {
			Image           string `json:"image"`
			Vulnerabilities []struct {
				ID string `json:"id"`
			} `json:"vulnerabilities"`
		} `json:"images"`
		Totals struct {
			Critical int `json:"critical"`
		} `json:"totals"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&overview); err != nil {
		t.Fatal(err)
	}
	if !overview.Enabled || len(overview.Images) != 1 || overview.Images[0].Image != "linuxserver/plex:latest" {
		t.Fatalf("unexpected overview: %+v", overview)
	}
	if len(overview.Images[0].Vulnerabilities) != 1 || overview.Images[0].Vulnerabilities[0].ID != "CVE-2026-0001" {
		t.Fatalf("missing findings: %+v", overview.Images[0])
	}
	if overview.Totals.Critical != 1 {
		t.Fatalf("unexpected totals: %+v", overview.Totals)
	}
}

func TestContainerSecurityAndReadOnly(t *testing.T) {
	env := loginTestEnv(t, false)
	resp := env.request(t, http.MethodGet, "/api/v1/docker/containers/abc/security", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("container security = %d", resp.StatusCode)
	}

	readOnly := loginTestEnv(t, true)
	resp = readOnly.request(t, http.MethodPost, "/api/v1/trivy/install", map[string]any{}, map[string]string{"X-Aurora-Request": "aurora"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("read-only install = %d", resp.StatusCode)
	}
}
