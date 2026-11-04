package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aurora/internal/model"
	"aurora/internal/provider"
)

func (s *Server) osStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	status, err := s.provider.OSStatus(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "os_status_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) osCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	status, err := s.provider.CheckOSUpdate(ctx)
	if err != nil {
		s.log.Warn("Unraid OS update check failed", "error", err)
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) osUpdate(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	streamPluginCommand(w, r, s, func(ctx context.Context, emit func(model.PluginEvent)) error {
		return s.provider.InstallOSUpdate(ctx, emit)
	})
}

func (s *Server) osRestore(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	version, err := s.provider.RestorePreviousOS(ctx)
	if err != nil {
		writeError(w, http.StatusBadRequest, "os_restore_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "version": version, "rebootRequired": true})
}

func (s *Server) unraidAPIStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	status, err := s.provider.UnraidAPIStatus(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "unraid_api_status_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) unraidAPIAction(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	action := r.PathValue("action")
	switch action {
	case "start", "stop", "restart":
	default:
		writeError(w, http.StatusBadRequest, "unsupported_action", "action must be start, stop or restart")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	output, err := s.provider.UnraidAPIAction(ctx, action)
	if err != nil {
		writeError(w, http.StatusBadRequest, "unraid_api_action_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "output": output})
}

func (s *Server) unraidAPILogs(w http.ResponseWriter, r *http.Request) {
	lines, _ := strconv.Atoi(r.URL.Query().Get("lines"))
	ctx, cancel := s.serverContext()
	defer cancel()
	entries, err := s.provider.UnraidAPILogs(ctx, lines)
	if err != nil {
		writeError(w, http.StatusBadRequest, "unraid_api_logs_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) networkConfig(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	cfg, err := s.provider.NetworkConfig(ctx)
	if err != nil {
		writeError(w, http.StatusBadRequest, "network_config_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) saveNetworkConfig(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var body model.NetworkConfigUpdate
	if !s.decode(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cfg, err := s.provider.SaveNetworkConfig(ctx, body.NetworkConfig, requestClientIP(r), body.Acknowledge)
	if err != nil {
		if errors.Is(err, provider.ErrNetworkLockout) {
			writeError(w, http.StatusConflict, "network_lockout_risk", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "network_config_invalid", err.Error())
		return
	}
	s.log.Info("network configuration saved", "backup", cfg.Backup, "interfaces", len(cfg.Interfaces))
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) applyNetworkConfig(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var body struct {
		Acknowledge bool `json:"acknowledge"`
	}
	if !s.decode(w, r, &body) {
		return
	}
	if !body.Acknowledge {
		writeError(w, http.StatusBadRequest, "acknowledge_required", "restarting networking requires explicit confirmation")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.provider.ApplyNetworkConfig(ctx); err != nil {
		writeError(w, http.StatusBadRequest, "network_apply_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "applying": true})
}

func (s *Server) networkInterfaceAction(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	action := r.PathValue("action")
	if action != "up" && action != "down" {
		writeError(w, http.StatusBadRequest, "unsupported_action", "action must be up or down")
		return
	}
	name := r.PathValue("name")
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err := s.provider.SetNetworkInterfaceState(ctx, name, action == "up"); err != nil {
		writeError(w, http.StatusBadRequest, "network_interface_action_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name, "state": action})
}

func (s *Server) revertNetworkConfig(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	ctx, cancel := s.serverContext()
	defer cancel()
	backup, err := s.provider.RevertNetworkConfig(ctx)
	if err != nil {
		writeError(w, http.StatusBadRequest, "network_revert_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "backup": backup})
}

func requestClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	host = strings.Trim(host, "[]")
	if parsed := net.ParseIP(host); parsed != nil && parsed.IsLoopback() {
		if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); forwarded != "" {
			return forwarded
		}
	}
	return host
}
