package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"aurora/internal/model"
)

type bearerTransport struct {
	secret string
}

func (b bearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header.Set("Authorization", "Bearer "+b.secret)
	return http.DefaultTransport.RoundTrip(clone)
}

func (f *fakeProvider) System(context.Context) (model.System, error) {
	return model.System{Name: "Vault", Version: "7.2.4"}, nil
}

func (f *fakeProvider) Array(context.Context) (model.Array, error) {
	return model.Array{State: "STARTED", Total: 100, Used: 40}, nil
}

func (f *fakeProvider) VMs(context.Context) ([]model.VM, error) {
	return []model.VM{{ID: "vm-plex", Name: "plex", State: "running"}}, nil
}

func (f *fakeProvider) Containers(context.Context) ([]model.Container, error) {
	return []model.Container{{ID: "abc", Name: "plex", State: "running"}}, nil
}

func TestMCPTokenScopes(t *testing.T) {
	if !tokenRoleAllows(TokenRoleViewer, nil, http.MethodPost, "/api/v1/mcp") {
		t.Fatal("an unscoped viewer token must be allowed to open an MCP session")
	}
	if !tokenRoleAllows(TokenRoleViewer, []string{"docker"}, http.MethodPost, "/api/v1/mcp") {
		t.Fatal("a scoped viewer token must be allowed to open an MCP session")
	}
}

func TestMCPHTTPScopedViewer(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	resp.Body.Close()

	resp = env.request(t, http.MethodPost, "/api/v1/tokens",
		map[string]any{"name": "mcp-scoped", "role": "viewer", "scopes": []string{"docker"}}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("token creation failed: %d", resp.StatusCode)
	}
	var created struct {
		Secret string `json:"secret"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	transport := &mcp.StreamableClientTransport{
		Endpoint:             env.server.URL + "/api/v1/mcp",
		HTTPClient:           &http.Client{Transport: bearerTransport{secret: created.Secret}},
		DisableStandaloneSSE: true,
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "aurora-test"}, nil)
	session, err := client.Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatalf("a scoped read-only token must be able to connect: %v", err)
	}
	defer session.Close()

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_containers"})
	if err != nil || result.IsError {
		t.Fatalf("docker-scoped token could not list containers: %v %+v", err, result)
	}
	result, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_array"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("docker-scoped token must not read the array")
	}
}

func TestMCPHTTPReadOnlySession(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	resp.Body.Close()

	resp = env.request(t, http.MethodPost, "/api/v1/tokens",
		map[string]any{"name": "mcp-test", "role": "viewer"}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("token creation failed: %d", resp.StatusCode)
	}
	var created struct {
		Secret string `json:"secret"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Secret == "" {
		t.Fatal("no token secret returned")
	}

	transport := &mcp.StreamableClientTransport{
		Endpoint:             env.server.URL + "/api/v1/mcp",
		HTTPClient:           &http.Client{Transport: bearerTransport{secret: created.Secret}},
		DisableStandaloneSSE: true,
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "aurora-test"}, nil)
	session, err := client.Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tool := range tools.Tools {
		if tool.Name == "get_array" {
			found = true
		}
	}
	if !found {
		t.Fatal("get_array is missing from the MCP tool list")
	}

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_array"})
	if err != nil || result.IsError {
		t.Fatalf("get_array failed: %v %+v", err, result)
	}
	result, err = session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "start_vm", Arguments: map[string]any{"target": "plex"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("a viewer token must not be able to start a VM")
	}
}
