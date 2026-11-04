package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func loginTestEnv(t *testing.T, readOnly bool) *testEnv {
	t.Helper()
	env := newTestEnv(t, readOnly)
	resp := env.request(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "root", "password": "secret"},
		map[string]string{"X-Aurora-Request": "aurora"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d", resp.StatusCode)
	}
	return env
}

func TestNetworkConfigEndpoints(t *testing.T) {
	env := loginTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}

	resp := env.request(t, http.MethodGet, "/api/v1/network/config", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("network config status = %d", resp.StatusCode)
	}
	var cfg struct {
		Interfaces []struct {
			IPAddr string `json:"ipAddr"`
		} `json:"interfaces"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Interfaces) != 1 || cfg.Interfaces[0].IPAddr != "192.168.1.10" {
		t.Fatalf("unexpected network config: %+v", cfg)
	}

	resp = env.request(t, http.MethodPut, "/api/v1/network/config", map[string]any{"unknown": true}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid body, got %d", resp.StatusCode)
	}
}

func TestNetworkConfigLockoutMapsToConflict(t *testing.T) {
	env := loginTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}
	body := map[string]any{
		"interfaces": []map[string]any{{
			"index": 0, "interface": "br0", "ipAddr": "10.0.0.1", "netmask": "255.255.255.0",
			"gateway": "10.0.0.254", "useDhcp": false, "bondNics": []string{},
			"bridgeNics": []string{"eth0"}, "bridging": true, "bridgeName": "br0", "mtu": 1500,
		}},
		"dns": []string{},
	}
	resp := env.request(t, http.MethodPut, "/api/v1/network/config", body, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error.Code != "network_lockout_risk" {
		t.Fatalf("unexpected error code %q", payload.Error.Code)
	}
}

func TestSystemStatusEndpoints(t *testing.T) {
	env := loginTestEnv(t, false)
	for _, path := range []string{"/api/v1/system/os", "/api/v1/unraid-api"} {
		resp := env.request(t, http.MethodGet, path, nil, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d", path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestNetworkInterfaceActions(t *testing.T) {
	env := loginTestEnv(t, false)
	headers := map[string]string{"X-Aurora-Request": "aurora"}

	resp := env.request(t, http.MethodPost, "/api/v1/network/interfaces/br0/up", map[string]any{}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("interface up status = %d", resp.StatusCode)
	}

	resp = env.request(t, http.MethodPost, "/api/v1/network/interfaces/br0/down", map[string]any{}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("interface down status = %d", resp.StatusCode)
	}

	resp = env.request(t, http.MethodPost, "/api/v1/network/interfaces/br0/reboot", map[string]any{}, headers)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid action status = %d", resp.StatusCode)
	}
}

func TestNetworkInterfaceActionsReadOnly(t *testing.T) {
	env := loginTestEnv(t, true)
	resp := env.request(t, http.MethodPost, "/api/v1/network/interfaces/br0/up", map[string]any{}, map[string]string{"X-Aurora-Request": "aurora"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 in read-only mode, got %d", resp.StatusCode)
	}
}
