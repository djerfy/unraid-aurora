package mcpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"aurora/internal/model"
	"aurora/internal/provider"
)

type fakeProvider struct {
	provider.Provider
	performed []model.Action
}

func (f *fakeProvider) System(context.Context) (model.System, error) {
	return model.System{Name: "Tower", Version: "7.2.4"}, nil
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

func (f *fakeProvider) Perform(_ context.Context, action model.Action) error {
	f.performed = append(f.performed, action)
	return nil
}

func connect(t *testing.T, opts Options) *mcp.ClientSession {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := New(opts)
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "aurora-test"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestReadToolsResourcesAndPrompts(t *testing.T) {
	session := connect(t, Options{Provider: &fakeProvider{}, Version: "test"})
	ctx := context.Background()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool, len(tools.Tools))
	for _, tool := range tools.Tools {
		names[tool.Name] = true
	}
	for _, expected := range []string{"get_array", "list_vms", "vm_logs", "get_metrics_history", "start_vm"} {
		if !names[expected] {
			t.Fatalf("missing tool %q", expected)
		}
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_array"})
	if err != nil || result.IsError {
		t.Fatalf("get_array failed: %v %+v", err, result)
	}
	if text := result.Content[0].(*mcp.TextContent).Text; !strings.Contains(text, "STARTED") {
		t.Fatalf("unexpected array payload: %s", text)
	}

	resource, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "aurora://vms"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resource.Contents) != 1 || !strings.Contains(resource.Contents[0].Text, "plex") {
		t.Fatalf("unexpected resource: %+v", resource.Contents)
	}

	prompt, err := session.GetPrompt(ctx, &mcp.GetPromptParams{Name: "vm-triage", Arguments: map[string]string{"vm": "plex"}})
	if err != nil || len(prompt.Messages) != 1 {
		t.Fatalf("unexpected prompt: %v %+v", err, prompt)
	}
	if text := prompt.Messages[0].Content.(*mcp.TextContent).Text; !strings.Contains(text, "plex") {
		t.Fatalf("prompt does not mention the VM: %s", text)
	}
}

func TestActionsRequirePermission(t *testing.T) {
	fake := &fakeProvider{}
	readOnly := connect(t, Options{Provider: fake, Version: "test", ReadOnly: true})
	result, err := readOnly.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "start_vm", Arguments: map[string]any{"target": "plex"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected the action to be rejected in read-only mode")
	}
	if len(fake.performed) != 0 {
		t.Fatalf("provider was called in read-only mode: %+v", fake.performed)
	}
}

func TestScopedTokenLimitsTools(t *testing.T) {
	fake := &fakeProvider{}
	session := connect(t, Options{Provider: fake, Version: "test", Scopes: []string{"docker"}, AllowActions: true})
	ctx := context.Background()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_containers"})
	if err != nil || result.IsError {
		t.Fatalf("docker scope should list containers: %v %+v", err, result)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "get_array"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("docker scope must not read the array")
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "start_vm", Arguments: map[string]any{"target": "plex"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("docker scope must not start a VM")
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "start_container", Arguments: map[string]any{"target": "abc"}})
	if err != nil || result.IsError {
		t.Fatalf("docker scope should start a container: %v %+v", err, result)
	}
}

func TestActionsRunWhenAllowed(t *testing.T) {
	fake := &fakeProvider{}
	session := connect(t, Options{Provider: fake, Version: "test", AllowActions: true})
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "start_vm", Arguments: map[string]any{"target": "plex"},
	})
	if err != nil || result.IsError {
		t.Fatalf("start_vm failed: %v %+v", err, result)
	}
	if len(fake.performed) != 1 || fake.performed[0].Kind != provider.ActionVMStart || fake.performed[0].Target != "plex" {
		t.Fatalf("unexpected actions: %+v", fake.performed)
	}
}

type jsonOnlyTransport struct{}

func (jsonOnlyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header.Set("Accept", "application/json")
	return http.DefaultTransport.RoundTrip(clone)
}

func TestHTTPJSONOnlyClient(t *testing.T) {
	server := httptest.NewServer(HTTPHandler(func(*http.Request) Options {
		return Options{Provider: &fakeProvider{}, Version: "test"}
	}))
	defer server.Close()

	transport := &mcp.StreamableClientTransport{
		Endpoint:             server.URL,
		HTTPClient:           &http.Client{Transport: jsonOnlyTransport{}},
		DisableStandaloneSSE: true,
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "json-only"}, nil)
	session, err := client.Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatalf("a JSON-only client could not connect: %v", err)
	}
	defer session.Close()

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_array"})
	if err != nil || result.IsError {
		t.Fatalf("get_array failed: %v %+v", err, result)
	}
}
