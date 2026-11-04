package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenAPIDocumentIsValid(t *testing.T) {
	var spec struct {
		OpenAPI string `json:"openapi"`
		Paths   map[string]map[string]any
	}
	if err := json.Unmarshal(openAPIDocument, &spec); err != nil {
		t.Fatalf("embedded OpenAPI document is invalid JSON: %v", err)
	}
	if !strings.HasPrefix(spec.OpenAPI, "3.") {
		t.Fatalf("unexpected OpenAPI version: %q", spec.OpenAPI)
	}
	for _, path := range []string{"/api/v1/bootstrap", "/api/v1/docker/containers/bulk", "/api/v1/auth/revoke-all"} {
		if len(spec.Paths[path]) == 0 {
			t.Fatalf("OpenAPI document is missing %s", path)
		}
	}
}

func TestOpenAPIDocumentMatchesDocsCopy(t *testing.T) {
	path := filepath.Join("..", "..", "..", "docs", "openapi.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read docs copy: %v", err)
	}
	if string(raw) != string(openAPIDocument) {
		t.Fatal("docs/openapi.json is stale; run `python3 scripts/openapi.py`")
	}
}

func TestAPIDocsPageIsSelfHosted(t *testing.T) {
	for _, forbidden := range []string{"cdn.redoc.ly", "fonts.googleapis.com", "unpkg.com", "jsdelivr"} {
		if strings.Contains(apiDocsHTML, forbidden) {
			t.Fatalf("docs page references an external asset: %s", forbidden)
		}
	}
	if !strings.Contains(apiDocsHTML, "/redoc/init.js") || !strings.Contains(apiDocsHTML, "type=\"module\"") {
		t.Fatal("docs page must load the local Redoc module")
	}
}

func TestAPIDocsHandlers(t *testing.T) {
	server := &Server{}
	docs := httptest.NewRecorder()
	server.apiDocs(docs, httptest.NewRequest(http.MethodGet, "/api/docs", nil))
	if docs.Code != http.StatusOK || !strings.Contains(docs.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("unexpected docs response: %d %s", docs.Code, docs.Header().Get("Content-Type"))
	}
	spec := httptest.NewRecorder()
	server.openAPISpec(spec, httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil))
	if spec.Code != http.StatusOK || !strings.Contains(spec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("unexpected spec response: %d %s", spec.Code, spec.Header().Get("Content-Type"))
	}
	if spec.Body.Len() == 0 {
		t.Fatal("spec response is empty")
	}
}

func TestAPIDocsRoutesRequireSession(t *testing.T) {
	env := newTestEnv(t, false)
	resp := env.request(t, http.MethodGet, "/api/docs", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a session, got %d", resp.StatusCode)
	}
	logged := loginTestEnv(t, false)
	resp = logged.request(t, http.MethodGet, "/api/docs", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("unexpected docs route: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	resp = logged.request(t, http.MethodGet, "/api/openapi.json", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("unexpected spec route: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
}
