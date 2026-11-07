package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"aurora/internal/model"
)

const inspectFixture = `{
  "Id": "old-container",
  "Name": "/plex",
  "State": {"Running": true},
  "Config": {
    "Image": "plexinc/pms-docker:latest",
    "Cmd": ["serve"],
    "Env": ["PUID=99", "TZ=Europe/Paris"],
    "ExposedPorts": {"32400/tcp": {}},
    "Labels": {"net.unraid.docker.icon": "https://example.com/plex.png"}
  },
  "HostConfig": {
    "NetworkMode": "bridge",
    "Binds": ["/mnt/user/appdata/plex:/config", "/mnt/user/media:/data:ro"],
    "PortBindings": {"32400/tcp": [{"HostIp": "", "HostPort": "32400"}]},
    "RestartPolicy": {"Name": "unless-stopped", "MaximumRetryCount": 0},
    "Privileged": true
  },
  "NetworkSettings": {
    "Networks": {"bridge": {"Aliases": null}}
  }
}`

type fakeDocker struct {
	mu         sync.Mutex
	calls      []string
	createBody map[string]any
	failCreate bool
}

func (f *fakeDocker) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, req.Method+" "+req.URL.Path)

	respond := func(status int, body string) *http.Response {
		return &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}
	}
	path := req.URL.Path
	switch {
	case req.Method == http.MethodGet && strings.HasSuffix(path, "/json"):
		return respond(http.StatusOK, inspectFixture), nil
	case req.Method == http.MethodPost && strings.HasSuffix(path, "/stop"):
		return respond(http.StatusNoContent, ""), nil
	case req.Method == http.MethodPost && strings.HasSuffix(path, "/rename"):
		return respond(http.StatusNoContent, ""), nil
	case req.Method == http.MethodPost && strings.HasSuffix(path, "/create"):
		raw, _ := io.ReadAll(req.Body)
		_ = json.Unmarshal(raw, &f.createBody)
		if f.failCreate {
			return respond(http.StatusInternalServerError, `{"message":"boom"}`), nil
		}
		return respond(http.StatusCreated, `{"Id":"new-container"}`), nil
	case req.Method == http.MethodPost && strings.HasSuffix(path, "/start"):
		return respond(http.StatusNoContent, ""), nil
	case req.Method == http.MethodDelete:
		return respond(http.StatusNoContent, ""), nil
	default:
		return respond(http.StatusNotFound, `{"message":"not found"}`), nil
	}
}

func (f *fakeDocker) hasCall(value string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, call := range f.calls {
		if call == value {
			return true
		}
	}
	return false
}

func newFakeLive() (*Live, *fakeDocker) {
	fake := &fakeDocker{}
	return &Live{client: &http.Client{Transport: fake}}, fake
}

func TestContainerConfigReadsDockerInspect(t *testing.T) {
	live, _ := newFakeLive()
	cfg, err := live.ContainerConfig(context.Background(), "old-container")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if cfg.Name != "plex" || cfg.Image != "plexinc/pms-docker:latest" || cfg.NetworkMode != "bridge" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.RestartPolicy != "unless-stopped" {
		t.Fatalf("restart policy = %q", cfg.RestartPolicy)
	}
	if len(cfg.Ports) != 1 || cfg.Ports[0].Host != 32400 || cfg.Ports[0].Container != 32400 {
		t.Fatalf("ports = %+v", cfg.Ports)
	}
	if len(cfg.Volumes) != 2 || cfg.Volumes[0].Destination != "/config" {
		t.Fatalf("volumes = %+v", cfg.Volumes)
	}
	foundRO := false
	for _, volume := range cfg.Volumes {
		if volume.Destination == "/data" {
			foundRO = volume.ReadOnly
		}
	}
	if !foundRO {
		t.Fatal("/data should be read-only")
	}
	if len(cfg.Env) != 2 || cfg.Env[1].Key != "TZ" {
		t.Fatalf("env = %+v", cfg.Env)
	}
}

func TestUpdateContainerConfigRecreates(t *testing.T) {
	live, fake := newFakeLive()
	cfg := model.ContainerConfig{
		RestartPolicy: "always",
		Ports:         []model.Port{{Host: 8080, Container: 80, Protocol: "tcp"}},
		Volumes: []model.ContainerVolume{
			{Source: "/mnt/user/appdata/plex", Destination: "/config"},
			{Source: "/mnt/user/media", Destination: "/data", ReadOnly: true},
			{Source: "/mnt/user/recordings", Destination: "/recordings"},
		},
		Env:        []model.ContainerEnv{{Key: "PUID", Value: "99"}, {Key: "NEW_VAR", Value: "yes"}},
		Privileged: true,
	}
	if err := live.UpdateContainerConfig(context.Background(), "old-container", cfg); err != nil {
		t.Fatalf("update: %v", err)
	}
	for _, call := range []string{
		"POST /containers/old-container/stop",
		"POST /containers/old-container/rename",
		"POST /containers/create",
		"POST /containers/new-container/start",
		"DELETE /containers/old-container",
	} {
		if !fake.hasCall(call) {
			t.Fatalf("missing docker call %q in %v", call, fake.calls)
		}
	}
	if fake.createBody == nil {
		t.Fatal("create body was not captured")
	}
	host := fake.createBody["HostConfig"].(map[string]any)
	if host["RestartPolicy"].(map[string]any)["Name"] != "always" {
		t.Fatalf("restart policy not updated: %+v", host["RestartPolicy"])
	}
	if host["Privileged"] != true {
		t.Fatal("unrelated HostConfig options must be preserved")
	}
	binds := host["Binds"].([]any)
	if len(binds) != 3 || binds[2] != "/mnt/user/recordings:/recordings" {
		t.Fatalf("binds = %+v", binds)
	}
	env := fake.createBody["Env"].([]any)
	if len(env) != 2 || env[1] != "NEW_VAR=yes" {
		t.Fatalf("env = %+v", env)
	}
	ports := host["PortBindings"].(map[string]any)
	if _, ok := ports["80/tcp"]; !ok {
		t.Fatalf("ports = %+v", ports)
	}
}

func TestUpdateContainerConfigRollsBack(t *testing.T) {
	live, fake := newFakeLive()
	fake.failCreate = true
	cfg := model.ContainerConfig{
		RestartPolicy: "unless-stopped",
		Ports:         []model.Port{{Host: 32400, Container: 32400, Protocol: "tcp"}},
		Volumes:       []model.ContainerVolume{{Source: "/tmp", Destination: "/config"}},
		Env:           []model.ContainerEnv{},
	}
	if err := live.UpdateContainerConfig(context.Background(), "old-container", cfg); err == nil {
		t.Fatal("expected create failure")
	}
	if !fake.hasCall("POST /containers/old-container/start") {
		t.Fatalf("old container should be restarted after rollback: %v", fake.calls)
	}
}

func TestValidateContainerConfig(t *testing.T) {
	valid := model.ContainerConfig{
		RestartPolicy: "unless-stopped",
		Ports:         []model.Port{{Host: 8080, Container: 80, Protocol: "tcp"}},
		Volumes:       []model.ContainerVolume{{Source: "/mnt/user/appdata", Destination: "/config"}},
		Env:           []model.ContainerEnv{{Key: "PUID", Value: "99"}},
	}
	if err := ValidateContainerConfig(valid); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	invalid := []model.ContainerConfig{
		{RestartPolicy: "sometimes"},
		{RestartPolicy: "no", Ports: []model.Port{{Host: 0, Container: 80, Protocol: "tcp"}}},
		{RestartPolicy: "no", Ports: []model.Port{{Host: 8080, Container: 80, Protocol: "icmp"}}},
		{RestartPolicy: "no", Ports: []model.Port{{Host: 8080, Container: 80, Protocol: "tcp"}, {Host: 8080, Container: 81, Protocol: "tcp"}}},
		{RestartPolicy: "no", Volumes: []model.ContainerVolume{{Source: "/data", Destination: "config"}}},
		{RestartPolicy: "no", Volumes: []model.ContainerVolume{{Source: "/data", Destination: "/config"}, {Source: "/other", Destination: "/config"}}},
		{RestartPolicy: "no", Env: []model.ContainerEnv{{Key: "BAD-KEY", Value: "x"}}},
	}
	for i, cfg := range invalid {
		if err := ValidateContainerConfig(cfg); err == nil {
			t.Fatalf("case %d should fail", i)
		}
	}
}

func TestIconHelpers(t *testing.T) {
	raw := []byte(`<Container><Icon>https://example.com/icon.png</Icon></Container>`)
	if got := iconElementText(raw); got != "https://example.com/icon.png" {
		t.Fatalf("icon element = %q", got)
	}
	domain := []byte(`<domain><metadata><vmtemplate icon="ubuntu.png"/></metadata></domain>`)
	if got := iconFromDomainXML(domain); got != "ubuntu.png" {
		t.Fatalf("domain icon = %q", got)
	}
	if got := defaultVMIcon("Windows 11 Pro"); got != "windows11.png" {
		t.Fatalf("windows icon = %q", got)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "icon.png")
	if err := os.WriteFile(path, []byte("not-a-real-png"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, mime, err := readFirstIcon(path)
	if err != nil || string(data) != "not-a-real-png" || mime != "image/png" {
		t.Fatalf("readFirstIcon = %q %q %v", data, mime, err)
	}
}
