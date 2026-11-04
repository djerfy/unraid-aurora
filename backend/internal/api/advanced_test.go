package api

import (
	"net/http"
	"testing"
)

func TestAppsAndSchedulerEndpoints(t *testing.T) {
	env := loginTestEnv(t, false)
	for _, path := range []string{"/api/v1/apps", "/api/v1/apps/template?url=https%3A%2F%2Fexample.test%2Ftemplate.xml", "/api/v1/apps/icon?url=https%3A%2F%2Fexample.test%2Ficon.png", "/api/v1/system/scheduler"} {
		resp := env.request(t, http.MethodGet, path, nil, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s = %d", path, resp.StatusCode)
		}
		resp.Body.Close()
	}

	headers := map[string]string{"X-Aurora-Request": "aurora"}
	resp := env.request(t, http.MethodPost, "/api/v1/system/scheduler/preview", map[string]any{}, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("scheduler preview = %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = env.request(t, http.MethodPut, "/api/v1/system/scheduler", map[string]any{"acknowledge": false}, headers)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("scheduler without ack = %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = env.request(t, http.MethodPut, "/api/v1/system/scheduler", map[string]any{"acknowledge": true}, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("scheduler apply = %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = env.request(t, http.MethodPut, "/api/v1/system/display", map[string]any{"acknowledge": true, "dateFormat": "%c", "timeFormat": "%R"}, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("display apply = %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestVMAdvancedEndpoints(t *testing.T) {
	env := loginTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	for _, path := range []string{"/api/v1/vms/host-devices", "/api/v1/vms/test/advanced"} {
		resp := env.request(t, http.MethodGet, path, nil, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s = %d", path, resp.StatusCode)
		}
		resp.Body.Close()
	}

	resp := env.request(t, http.MethodPut, "/api/v1/vms/test/advanced", map[string]any{
		"vcpus": 2, "memoryMB": 2048, "networks": []map[string]any{{"bridge": "br0", "model": "virtio"}},
	}, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("advanced update = %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = env.request(t, http.MethodPost, "/api/v1/vms/test/clone", map[string]any{"name": "clone", "start": false}, headers)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("clone = %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = env.request(t, http.MethodPost, "/api/v1/vms/test/resize", map[string]any{"sizeGB": 100}, headers)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("resize = %d", resp.StatusCode)
	}
	resp.Body.Close()

	readOnly := loginTestEnv(t, true)
	resp = readOnly.request(t, http.MethodPost, "/api/v1/vms/test/clone", map[string]any{"name": "clone"}, headers)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("read-only clone = %d", resp.StatusCode)
	}
	resp.Body.Close()
}
