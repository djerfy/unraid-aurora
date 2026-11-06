package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"aurora/internal/auth"
	"aurora/internal/config"
	"aurora/internal/metrics"
	"aurora/internal/model"
	"aurora/internal/provider"

	"github.com/coder/websocket"
)

// fakeProvider exposes the handful of methods the HTTP tests exercise. The
// embedded interface intentionally panics if a handler reaches an
// unimplemented method, which keeps the test surface honest.
type fakeProvider struct {
	provider.Provider
	settings model.Settings
	layout   model.StorageLayout
	folders  map[string]model.VisualFolders
}

func (f *fakeProvider) Name() string { return "live" }

func (f *fakeProvider) Snapshot(ctx context.Context) (model.Snapshot, error) {
	return model.Snapshot{
		System: model.System{Name: "Vault", Version: "7.2.4"},
		Array:  model.Array{State: "STARTED", Status: "protected"},
		Containers: []model.Container{
			{ID: "abc", Name: "plex", State: "running", Ports: []model.Port{}},
		},
		Notifications: []model.Notification{},
		Disks:         []model.Disk{},
		Shares:        []model.Share{},
		Pools:         []model.Pool{},
		VMs:           []model.VM{},
		Users:         []model.User{},
		Plugins:       []model.Plugin{},
		History:       []model.MetricPoint{},
		Metrics:       model.Metrics{CPU: 10, Cores: []float64{10}},
		Settings:      f.settings,
	}, nil
}

func (f *fakeProvider) ContainerConnections(ctx context.Context, id string) (model.ContainerConnections, error) {
	return model.ContainerConnections{
		Available: true,
		Connections: []model.ContainerConnection{{
			Protocol: "tcp", State: "ESTAB", RemoteAddress: "1.1.1.1", RemotePort: 443,
			Process: "plex", PID: 42, Country: "AU", City: "Sydney",
		}},
	}, nil
}

func (f *fakeProvider) ContainerNetworkMap(ctx context.Context, containerID string) (model.ContainerNetworkMap, error) {
	return model.ContainerNetworkMap{
		Available:  true,
		CapturedAt: time.Now(),
		Containers: []model.NetworkMapContainer{{ID: "abc", Name: "plex", IP: "172.18.0.2"}},
		Peers:      []model.NetworkMapPeer{{Address: "1.1.1.1", Scope: "internet", Connections: 1, Ports: []int{443}, Protocols: []string{"tcp"}}},
		Edges: []model.NetworkMapEdge{{
			ContainerID: "abc", ContainerName: "plex", PeerAddress: "1.1.1.1",
			Protocol: "tcp", LocalPort: 40000, RemotePort: 443, State: "ESTAB",
		}},
	}, nil
}

func (f *fakeProvider) ContainerPriorities(ctx context.Context) (map[string]int, error) {
	return map[string]int{"plex": 10}, nil
}

func (f *fakeProvider) SaveContainerPriorities(ctx context.Context, priorities map[string]int) (map[string]int, error) {
	return priorities, nil
}

func (f *fakeProvider) Metrics(ctx context.Context) (model.Metrics, error) {
	return model.Metrics{CPU: 10, Cores: []float64{10}}, nil
}

func (f *fakeProvider) History(ctx context.Context) ([]model.MetricPoint, error) {
	return []model.MetricPoint{}, nil
}

func (f *fakeProvider) Settings(ctx context.Context) (model.Settings, error) {
	return f.settings, nil
}

func (f *fakeProvider) SaveSettings(ctx context.Context, s model.Settings) error {
	f.settings = s
	return nil
}

func (f *fakeProvider) StorageLayout(ctx context.Context) (model.StorageLayout, error) {
	return f.layout, nil
}

func (f *fakeProvider) SaveStorageLayout(ctx context.Context, layout model.StorageLayout) error {
	f.layout = layout
	return nil
}

func (f *fakeProvider) NetworkConfig(ctx context.Context) (model.NetworkConfig, error) {
	return model.NetworkConfig{
		Format: "indexed",
		Interfaces: []model.NetworkInterfaceConfig{{
			Index: 0, Interface: "br0", IPAddr: "192.168.1.10", Netmask: "255.255.255.0",
			Gateway: "192.168.1.1", MTU: 1500,
		}},
	}, nil
}

func (f *fakeProvider) SaveNetworkConfig(ctx context.Context, cfg model.NetworkConfig, clientIP string, acknowledge bool) (model.NetworkConfig, error) {
	if len(cfg.Interfaces) > 0 && cfg.Interfaces[0].IPAddr == "10.0.0.1" {
		return cfg, provider.ErrNetworkLockout
	}
	return cfg, nil
}

func (f *fakeProvider) SetNetworkInterfaceState(ctx context.Context, name string, up bool) error {
	if name == "missing" {
		return errors.New("not configured")
	}
	return nil
}

func (f *fakeProvider) TrivyStatus(ctx context.Context) (model.TrivyStatus, error) {
	return model.TrivyStatus{Enabled: true, Installed: true, Version: "0.76.1", DBReady: true, DBUpdateHour: 3}, nil
}

func (f *fakeProvider) TrivyVersions(ctx context.Context) ([]model.TrivyVersion, error) {
	return []model.TrivyVersion{{Version: "0.76.1", Name: "v0.76.1"}}, nil
}

func (f *fakeProvider) StartTrivyInstall(version string) error { return nil }

func (f *fakeProvider) StartTrivyDBUpdate() error { return nil }

func (f *fakeProvider) AppsCatalog(ctx context.Context, query, kind, category, order string, limit, offset int) (model.AppCatalogPage, error) {
	return model.AppCatalogPage{
		Entries: []model.AppCatalogEntry{{
			ID: "https://example.test/template.xml", Name: "Example", Kind: "docker",
			Repository: "example/app:latest", Categories: []string{"Tools"}, TemplateURL: "https://example.test/template.xml",
		}},
		Total: 1, Categories: []string{"Tools"},
	}, nil
}

func (f *fakeProvider) AppTemplate(ctx context.Context, templateURL string) (model.ContainerTemplate, error) {
	return model.ContainerTemplate{Name: "example", Image: "example/app:latest", Network: "bridge", Ports: []model.Port{}, Volumes: []model.ContainerVolume{}, Env: []model.ContainerEnv{}}, nil
}

func (f *fakeProvider) AppIcon(ctx context.Context, iconURL string) ([]byte, string, error) {
	return []byte{0x89, 0x50, 0x4e, 0x47}, "image/png", nil
}

func (f *fakeProvider) SchedulerState(ctx context.Context) (model.SchedulerState, error) {
	return model.SchedulerState{Settings: model.SchedulerSettings{}}, nil
}

func (f *fakeProvider) PreviewScheduler(settings model.SchedulerSettings) (model.SchedulerPreview, error) {
	return model.SchedulerPreview{Files: []model.SchedulerPreviewFile{{Path: "/tmp/test.cron", Content: "test"}}}, nil
}

func (f *fakeProvider) ApplyScheduler(ctx context.Context, settings model.SchedulerSettings) error {
	return nil
}

func (f *fakeProvider) UpdateDisplaySettings(ctx context.Context, settings model.SystemDisplaySettings) error {
	return nil
}

func (f *fakeProvider) StartVMBackup(ctx context.Context, id, targetDir string) (model.VMBackup, error) {
	return model.VMBackup{ID: "b1", Kind: "backup", VMID: id, VMName: "plex", Dir: targetDir, State: "running", Phase: "preparing"}, nil
}

func (f *fakeProvider) VMBackupStatus(ctx context.Context, id string) (model.VMBackup, error) {
	return model.VMBackup{ID: "b1", Kind: "backup", VMID: id, VMName: "plex", State: "completed", Phase: "done"}, nil
}

func (f *fakeProvider) CancelVMBackup(ctx context.Context, id string) error { return nil }

func (f *fakeProvider) StartVMRestore(ctx context.Context, id, backupDir string, confirm bool) (model.VMBackup, error) {
	return model.VMBackup{ID: "r1", Kind: "restore", VMID: id, VMName: "plex", Dir: backupDir, State: "running", Phase: "verifying"}, nil
}

func (f *fakeProvider) VMAutostartOrder(ctx context.Context) (model.VMAutostartOrder, error) {
	return model.VMAutostartOrder{Order: []string{"plex"}}, nil
}

func (f *fakeProvider) SetVMAutostartOrder(ctx context.Context, order []string, managed bool) (model.VMAutostartOrder, error) {
	return model.VMAutostartOrder{Order: order, Managed: managed}, nil
}

func (f *fakeProvider) ApplyVMAutostartOrder(ctx context.Context) (model.VMAutostartResult, error) {
	return model.VMAutostartResult{Started: []string{"plex"}, Skipped: []string{}, Failed: []model.VMAutostartFailure{}}, nil
}

func (f *fakeProvider) VisualFolders(ctx context.Context, kind string) (model.VisualFolders, error) {
	if f.folders == nil {
		return model.VisualFolders{Version: 1, Folders: []model.VisualFolder{}, Assignments: map[string]string{}}, nil
	}
	return f.folders[kind], nil
}

func (f *fakeProvider) SaveVisualFolders(ctx context.Context, kind string, layout model.VisualFolders) error {
	if f.folders == nil {
		f.folders = make(map[string]model.VisualFolders)
	}
	f.folders[kind] = layout
	return nil
}

func (f *fakeProvider) VMLogs(ctx context.Context, id string, lines int) ([]model.LogLine, error) {
	return []model.LogLine{{
		Time:    time.Unix(0, 0),
		Level:   "info",
		Source:  "libvirt",
		Message: "starting up",
	}}, nil
}

func (f *fakeProvider) VMAdvanced(ctx context.Context, id string) (model.VMAdvancedConfig, error) {
	return model.VMAdvancedConfig{VCPUs: 2, MemoryMB: 2048, Networks: []model.VMNetworkConfig{{Bridge: "br0", Model: "virtio"}}}, nil
}

func (f *fakeProvider) UpdateVMAdvanced(ctx context.Context, id string, cfg model.VMAdvancedConfig) error {
	return nil
}

func (f *fakeProvider) VMHostDevices(ctx context.Context) ([]model.VMHostDevice, error) {
	return []model.VMHostDevice{{Type: "pci", Address: "0000:01:00.0", Name: "Test GPU"}}, nil
}

func (f *fakeProvider) CloneVM(ctx context.Context, id string, opts model.VMCloneOptions) (model.VM, error) {
	return model.VM{ID: "vm-clone", Name: opts.Name}, nil
}

func (f *fakeProvider) ResizeVMDisk(ctx context.Context, id string, opts model.VMDiskResizeOptions) error {
	return nil
}

func (f *fakeProvider) ContainerSecurity(ctx context.Context, id string) (model.ContainerSecurityDetail, error) {
	return model.ContainerSecurityDetail{ContainerSecurity: model.ContainerSecurity{Enabled: true, Available: true, Critical: 1, Total: 1}}, nil
}

func (f *fakeProvider) SecurityOverview(ctx context.Context) (model.SecurityOverview, error) {
	return model.SecurityOverview{
		Enabled: true,
		Images: []model.SecurityImage{{
			Image: "linuxserver/plex:latest", ImageID: "sha256:abc",
			Containers: []model.SecurityContainerRef{{ID: "abc", Name: "plex", State: "running"}},
			Security:   model.ContainerSecurity{Enabled: true, Available: true, Critical: 1, Total: 1},
			Vulnerabilities: []model.ContainerVulnerability{{
				ID: "CVE-2026-0001", Package: "openssl", Severity: "CRITICAL",
			}},
		}},
		Totals: model.SecurityTotals{Images: 1, ScannedImages: 1, Containers: 1, ScannedContainers: 1, Critical: 1, Total: 1},
	}, nil
}

func (f *fakeProvider) ScanContainer(ctx context.Context, id string) error { return nil }

func (f *fakeProvider) ScanAllContainers(ctx context.Context) (int, error) { return 2, nil }

func (f *fakeProvider) OSStatus(ctx context.Context) (model.OSStatus, error) {
	return model.OSStatus{Version: "7.2.4", Branch: "stable"}, nil
}

func (f *fakeProvider) UnraidAPIStatus(ctx context.Context) (model.UnraidAPIStatus, error) {
	return model.UnraidAPIStatus{Installed: true, Running: true, State: "online", Version: "4.36.1"}, nil
}

func (f *fakeProvider) Perform(ctx context.Context, action model.Action) error {
	if action.Kind == provider.ActionArrayStart {
		return nil
	}
	return errors.New("unsupported in fake")
}

func fakeAuth() *auth.Manager {
	return auth.New(time.Hour,
		auth.WithShadowLookup(func(username string) (string, error) {
			if username == "root" {
				return "$6$fake$hash", nil
			}
			return "", errors.New("missing")
		}),
		auth.WithPasswordVerifier(func(password, hash string) error {
			if password == "secret" {
				return nil
			}
			return errors.New("mismatch")
		}),
	)
}

type testEnv struct {
	server   *httptest.Server
	client   *http.Client
	provider *fakeProvider
}

func newTestEnv(t *testing.T, readOnly bool) *testEnv {
	t.Helper()
	cfg := config.Config{
		Version:    "test",
		SessionTTL: time.Hour,
		ReadOnly:   readOnly,
	}
	p := &fakeProvider{}
	hub := metrics.NewHub(p, "")
	srv := New(cfg, p, hub, fakeAuth(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	server := httptest.NewServer(srv.Handler())
	t.Cleanup(server.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &testEnv{server: server, client: &http.Client{Jar: jar}, provider: p}
}

func (e *testEnv) request(t *testing.T, method, path string, body any, headers map[string]string) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, e.server.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestHealth(t *testing.T) {
	env := newTestEnv(t, false)
	resp := env.request(t, http.MethodGet, "/api/v1/health", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d", resp.StatusCode)
	}
	var health map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatal(err)
	}
	if health["status"] != "ok" || health["provider"] != "live" {
		t.Fatalf("unexpected health payload: %v", health)
	}
}

func TestBootstrapRequiresAuthentication(t *testing.T) {
	env := newTestEnv(t, false)
	resp := env.request(t, http.MethodGet, "/api/v1/bootstrap", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthenticationFlow(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}

	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "wrong"}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad password, got %d", resp.StatusCode)
	}

	resp = env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for valid login, got %d", resp.StatusCode)
	}

	resp = env.request(t, http.MethodGet, "/api/v1/auth/session", nil, nil)
	defer resp.Body.Close()
	var session struct {
		Authenticated bool   `json:"authenticated"`
		User          string `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	if !session.Authenticated || session.User != "root" {
		t.Fatalf("unexpected session: %+v", session)
	}

	resp = env.request(t, http.MethodGet, "/api/v1/bootstrap", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with session cookie, got %d", resp.StatusCode)
	}
	var snap model.Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		t.Fatal(err)
	}
	if len(snap.Containers) != 1 || snap.Containers[0].Name != "plex" {
		t.Fatalf("unexpected snapshot: %+v", snap.Containers)
	}

	resp = env.request(t, http.MethodPost, "/api/v1/auth/logout", map[string]string{}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logout status = %d", resp.StatusCode)
	}
	resp = env.request(t, http.MethodGet, "/api/v1/bootstrap", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", resp.StatusCode)
	}
}

func TestLoginRejectsNonRoot(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "admin", "password": "secret"}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestActionsRequireCSRFHeader(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	resp.Body.Close()

	resp = env.request(t, http.MethodPost, "/api/v1/actions",
		model.Action{Kind: provider.ActionArrayStart}, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 without header, got %d", resp.StatusCode)
	}

	resp = env.request(t, http.MethodPost, "/api/v1/actions",
		model.Action{Kind: provider.ActionArrayStart}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with header, got %d", resp.StatusCode)
	}
}

func TestReadOnlyBlocksMutations(t *testing.T) {
	env := newTestEnv(t, true)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	resp.Body.Close()

	resp = env.request(t, http.MethodPost, "/api/v1/actions",
		model.Action{Kind: provider.ActionArrayStart}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 in read-only mode, got %d", resp.StatusCode)
	}
}

func TestSettingsValidation(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	resp.Body.Close()

	settings := model.Settings{Theme: "neon", Accent: "ember", RefreshInterval: 5}
	resp = env.request(t, http.MethodPut, "/api/v1/settings", settings, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for invalid theme, got %d", resp.StatusCode)
	}

	settings.Theme = "dark"
	resp = env.request(t, http.MethodPut, "/api/v1/settings", settings, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for valid settings, got %d", resp.StatusCode)
	}

	settings.Accent = "blue"
	resp = env.request(t, http.MethodPut, "/api/v1/settings", settings, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for the blue accent, got %d", resp.StatusCode)
	}
}

func TestUnknownActionIsRejected(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	resp.Body.Close()

	resp = env.request(t, http.MethodPost, "/api/v1/actions",
		model.Action{Kind: "array.explode"}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestTerminalWebSocket(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d", resp.StatusCode)
	}
	serverURL, err := url.Parse(env.server.URL)
	if err != nil {
		t.Fatal(err)
	}
	cookies := env.client.Jar.Cookies(serverURL)
	if len(cookies) == 0 {
		t.Fatal("session cookie missing")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(env.server.URL, "http") + "/api/v1/terminal"
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Cookie": []string{cookies[0].String()}},
	})
	if err != nil {
		t.Fatalf("dial terminal: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	if err := conn.Write(ctx, websocket.MessageBinary, []byte("echo TERMINAL_OK\n")); err != nil {
		t.Fatalf("write terminal: %v", err)
	}
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read terminal: %v", err)
		}
		if strings.Contains(string(data), "TERMINAL_OK") {
			return
		}
	}
}

func TestSaveStorageLayoutIgnoresClientTimestamp(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	resp.Body.Close()

	body := map[string]any{
		"version": 2,
		"updated": "",
		"views": []map[string]any{{
			"id": "tower", "name": "Tower", "kind": "tower", "size": "m",
			"x": 0, "y": 0, "w": 6, "h": 20,
			"sections": []map[string]any{{
				"id": "data", "name": "Data", "color": "#fff", "media": "any",
				"rows": 2, "cols": 2, "assignments": map[string]int{"disk1": 0},
			}},
		}},
	}
	resp = env.request(t, http.MethodPut, "/api/v1/storage-layout", body, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var layout model.StorageLayout
	if err := json.NewDecoder(resp.Body).Decode(&layout); err != nil {
		t.Fatal(err)
	}
	if layout.Version != 2 || len(layout.Views) != 1 || layout.Views[0].Kind != "tower" {
		t.Fatalf("unexpected layout: %+v", layout)
	}
}

func TestContainerConnectionsEndpoint(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	resp.Body.Close()

	resp = env.request(t, http.MethodGet, "/api/v1/docker/containers/abc/connections", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out model.ContainerConnections
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.Available || len(out.Connections) != 1 || out.Connections[0].RemoteAddress != "1.1.1.1" {
		t.Fatalf("unexpected connections: %+v", out)
	}
}

func TestVMLogsEndpoint(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	resp.Body.Close()

	resp = env.request(t, http.MethodGet, "/api/v1/vms/vm-plex/logs?lines=20", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var lines []model.LogLine
	if err := json.NewDecoder(resp.Body).Decode(&lines); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0].Message != "starting up" || lines[0].Source != "libvirt" {
		t.Fatalf("unexpected log lines: %+v", lines)
	}
}

func TestVisualFolderEndpoints(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	resp.Body.Close()

	resp = env.request(t, http.MethodGet, "/api/v1/docker/folders", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body := map[string]any{
		"version":     1,
		"folders":     []map[string]any{{"id": "media", "name": "Media", "order": 0}},
		"assignments": map[string]string{"plex": "media"},
	}
	resp = env.request(t, http.MethodPut, "/api/v1/docker/folders", body, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var saved model.VisualFolders
	if err := json.NewDecoder(resp.Body).Decode(&saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.Folders) != 1 || saved.Assignments["plex"] != "media" {
		t.Fatalf("unexpected folders: %+v", saved)
	}

	resp = env.request(t, http.MethodGet, "/api/v1/vms/folders", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestMCPEndpointDisabledByDefault(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	resp.Body.Close()

	resp = env.request(t, http.MethodGet, "/api/v1/mcp", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 while MCP is disabled, got %d", resp.StatusCode)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "mcp_disabled" {
		t.Fatalf("unexpected error code: %+v", body)
	}
}

func TestContainerPrioritiesEndpoint(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	resp.Body.Close()

	resp = env.request(t, http.MethodPut, "/api/v1/docker/priorities",
		map[string]any{"priorities": map[string]int{"plex": 10}}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out struct {
		OK         bool           `json:"ok"`
		Priorities map[string]int `json:"priorities"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.OK || out.Priorities["plex"] != 10 {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestSortContainerTargets(t *testing.T) {
	targets := []string{"web", "db", "cache", "other"}
	sortContainerTargets(targets, map[string]int{"db": 10, "cache": 20})
	want := []string{"db", "cache", "web", "other"}
	for index := range want {
		if targets[index] != want[index] {
			t.Fatalf("sorted = %v, want %v", targets, want)
		}
	}
}

func TestContainerNetworkMapEndpoint(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	resp.Body.Close()

	resp = env.request(t, http.MethodGet, "/api/v1/docker/network-map", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out model.ContainerNetworkMap
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.Available || len(out.Containers) != 1 || len(out.Peers) != 1 || len(out.Edges) != 1 {
		t.Fatalf("unexpected network map: %+v", out)
	}
	if out.Edges[0].PeerAddress != "1.1.1.1" {
		t.Fatalf("unexpected edge: %+v", out.Edges[0])
	}
}

func TestVMBackupEndpoints(t *testing.T) {
	env := newTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"}, headers)
	resp.Body.Close()

	resp = env.request(t, http.MethodPost, "/api/v1/vms/vm-plex/backup",
		map[string]string{"dir": "/mnt/user/backups"}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var job model.VMBackup
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		t.Fatal(err)
	}
	if job.Kind != "backup" || job.Dir != "/mnt/user/backups" {
		t.Fatalf("unexpected job: %+v", job)
	}

	resp = env.request(t, http.MethodGet, "/api/v1/vms/vm-plex/backup", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = env.request(t, http.MethodPost, "/api/v1/vms/vm-plex/restore",
		map[string]any{"dir": "/mnt/user/backups/aurora-plex-1", "confirm": true}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = env.request(t, http.MethodGet, "/api/v1/vms/autostart-order", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var order model.VMAutostartOrder
	if err := json.NewDecoder(resp.Body).Decode(&order); err != nil {
		t.Fatal(err)
	}
	if len(order.Order) != 1 || order.Order[0] != "plex" {
		t.Fatalf("unexpected order: %+v", order)
	}

	resp = env.request(t, http.MethodPut, "/api/v1/vms/autostart-order",
		map[string]any{"order": []string{"plex"}, "managed": true}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = env.request(t, http.MethodPost, "/api/v1/vms/autostart-order/apply", nil, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSecurityHeadersAllowGitHubAvatars(t *testing.T) {
	env := newTestEnv(t, false)
	resp := env.request(t, http.MethodGet, "/api/v1/health", nil, nil)
	defer resp.Body.Close()
	csp := resp.Header.Get("Content-Security-Policy")
	if !strings.Contains(csp, "https://avatars.githubusercontent.com") {
		t.Fatalf("CSP does not allow the GitHub avatar host: %q", csp)
	}
}
