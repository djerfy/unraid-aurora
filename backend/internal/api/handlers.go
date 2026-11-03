package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"aurora/internal/auth"
	"aurora/internal/model"
	"aurora/internal/notify"
	"aurora/internal/provider"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The connection is already committed; nothing else to do but report it.
		_ = err
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

func (s *Server) decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	s.log.Error("request failed", "error", err)
	writeError(w, http.StatusInternalServerError, "internal", "unable to read host data")
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"version":  s.cfg.Version,
		"provider": s.provider.Name(),
		"uptime":   int64(time.Since(s.started).Seconds()),
		"time":     time.Now(),
	})
}

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.sessionUser(r)
	if ok {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "user": sess.User})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false, "user": ""})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !s.decode(w, r, &body) {
		return
	}
	token, sess, err := s.auth.Login(strings.TrimSpace(body.Username), body.Password, auth.ClientIP(r))
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrRateLimited):
			writeError(w, http.StatusTooManyRequests, "rate_limited", "too many attempts, try again in 15 minutes")
		default:
			s.log.Warn("login failed", "user", body.Username, "ip", auth.ClientIP(r))
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
		}
		return
	}
	http.SetCookie(w, s.auth.Cookie(token, s.secureCookie(r)))
	s.log.Info("login succeeded", "user", sess.User, "ip", auth.ClientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "user": sess.User})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, s.auth.ClearCookie())
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) bootstrap(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	snap, err := s.provider.Snapshot(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	if latest := s.hub.Latest(); latest.T > 0 {
		snap.Metrics = latest
	}
	snap.History = s.hub.History()
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) system(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.System(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) array(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.Array(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) startArray(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var body model.ArrayStartOptions
	if !s.decode(w, r, &body) {
		return
	}
	if err := provider.ValidateArrayStartOptions(body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_array_options", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := s.provider.StartArray(ctx, body); err != nil {
		writeError(w, http.StatusInternalServerError, "array_start_failed", err.Error())
		return
	}
	s.log.Info("array start requested")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) stopArray(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := s.provider.StopArray(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "array_stop_failed", err.Error())
		return
	}
	s.log.Info("array stop requested")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) disks(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.Disks(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) diskSmart(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	report, err := s.provider.SmartReport(ctx, r.PathValue("device"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "smart_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) diskSmartTest(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	device := r.PathValue("device")
	if err := s.provider.RunSmartTest(ctx, device); err != nil {
		writeError(w, http.StatusInternalServerError, "smart_test_failed", err.Error())
		return
	}
	s.log.Info("smart self-test started", "device", device)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) diagnostics(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	data, err := s.provider.Diagnostics(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	filename := fmt.Sprintf("aurora-diagnostics-%s.zip", time.Now().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) shares(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.Shares(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) pools(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.Pools(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) createShare(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var cfg model.ShareConfig
	if !s.decode(w, r, &cfg) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := s.provider.CreateShare(ctx, cfg); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "create_share_failed", err.Error())
		return
	}
	s.log.Info("share created", "share", cfg.Name)
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (s *Server) shareConfig(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	cfg, err := s.provider.ShareConfig(ctx, r.PathValue("name"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) updateShare(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var cfg model.ShareConfig
	if !s.decode(w, r, &cfg) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	name := r.PathValue("name")
	if err := s.provider.UpdateShareProperties(ctx, name, cfg); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "update_share_failed", err.Error())
		return
	}
	s.log.Info("share settings updated", "share", name)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) updateShareSMB(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var cfg model.ShareConfig
	if !s.decode(w, r, &cfg) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	name := r.PathValue("name")
	if err := s.provider.UpdateShareSMB(ctx, name, cfg); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "update_share_smb_failed", err.Error())
		return
	}
	s.log.Info("share SMB settings updated", "share", name)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) updateShareNFS(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var cfg model.ShareConfig
	if !s.decode(w, r, &cfg) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	name := r.PathValue("name")
	if err := s.provider.UpdateShareNFS(ctx, name, cfg); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "update_share_nfs_failed", err.Error())
		return
	}
	s.log.Info("share NFS settings updated", "share", name)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) containers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.Containers(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) containerLogs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	lines := queryInt(r, "lines", 120, 1, 1000)
	v, err := s.provider.ContainerLogs(ctx, r.PathValue("id"), lines)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) containerConnections(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.ContainerConnections(ctx, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "container_connections", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) containerIcon(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	data, contentType, err := s.provider.ContainerIcon(ctx, r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	serveIcon(w, data, contentType)
}

func (s *Server) vmIcon(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	data, contentType, err := s.provider.VMIcon(ctx, r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	serveIcon(w, data, contentType)
}

func serveIcon(w http.ResponseWriter, data []byte, contentType string) {
	if contentType == "" {
		contentType = "image/png"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) containerConfig(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	cfg, err := s.provider.ContainerConfig(ctx, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) updateContainerConfig(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var cfg model.ContainerConfig
	if !s.decode(w, r, &cfg) {
		return
	}
	if err := provider.ValidateContainerConfig(cfg); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_config", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := s.provider.UpdateContainerConfig(ctx, r.PathValue("id"), cfg); err != nil {
		s.log.Error("container update failed", "id", r.PathValue("id"), "error", err)
		writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}
	s.log.Info("container configuration updated", "id", r.PathValue("id"), "name", cfg.Name)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) createContainer(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var opts model.ContainerCreateOptions
	if !s.decode(w, r, &opts) {
		return
	}
	if err := provider.ValidateContainerCreateOptions(opts); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_container", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	id, err := s.provider.CreateContainer(ctx, opts)
	if err != nil {
		s.log.Error("container creation failed", "name", opts.Name, "error", err)
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	s.log.Info("container created", "name", opts.Name, "image", opts.Image, "id", id)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// updateContainerImage streams the pull/recreate stages as newline-delimited
// JSON so the UI can display progress like the official webGUI.
func (s *Server) updateContainerImage(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming", "streaming is not supported")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(w)
	emit := func(event model.ContainerUpdateEvent) {
		if err := encoder.Encode(event); err == nil {
			flusher.Flush()
		}
	}
	id := r.PathValue("id")
	err := s.provider.UpdateContainerImage(ctx, id, emit)
	if err != nil {
		s.log.Error("container image update failed", "id", id, "error", err)
		emit(model.ContainerUpdateEvent{Step: "error", Message: err.Error(), Error: true, Done: true})
		return
	}
	s.log.Info("container image updated", "id", id)
}

func (s *Server) removeContainer(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := s.provider.RemoveContainer(ctx, r.PathValue("id")); err != nil {
		s.log.Error("container removal failed", "id", r.PathValue("id"), "error", err)
		writeError(w, http.StatusInternalServerError, "remove_failed", err.Error())
		return
	}
	s.log.Info("container removed", "id", r.PathValue("id"))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) containerTemplates(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	templates, err := s.provider.ContainerTemplates(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, templates)
}

func (s *Server) containerAutostart(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if !s.decode(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	id := r.PathValue("id")
	if err := s.provider.SetContainerAutostart(ctx, id, body.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "autostart_failed", err.Error())
		return
	}
	s.log.Info("container autostart updated", "id", id, "enabled", body.Enabled)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) dockerNetworks(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	networks, err := s.provider.DockerNetworks(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, networks)
}

func (s *Server) checkContainerUpdates(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	checks, err := s.provider.CheckContainerUpdates(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	updates := 0
	for _, check := range checks {
		if check.Update {
			updates++
		}
	}
	s.log.Info("container image check completed", "images", len(checks), "updates", updates)
	writeJSON(w, http.StatusOK, checks)
}

func (s *Server) vmAutostart(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if !s.decode(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	id := r.PathValue("id")
	if err := s.provider.SetVMAutostart(ctx, id, body.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "autostart_failed", err.Error())
		return
	}
	s.log.Info("vm autostart updated", "id", id, "enabled", body.Enabled)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) vmXML(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	xml, err := s.provider.VMXML(ctx, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"xml": xml})
}

func (s *Server) createVM(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var opts model.VMCreateOptions
	if !s.decode(w, r, &opts) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	vm, err := s.provider.CreateVM(ctx, opts)
	if err != nil {
		writeError(w, http.StatusBadRequest, "vm_create_failed", err.Error())
		return
	}
	s.log.Info("vm created", "name", opts.Name)
	writeJSON(w, http.StatusCreated, vm)
}

func (s *Server) updateVMXML(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var body struct {
		XML string `json:"xml"`
	}
	if !s.decode(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.XML) == "" {
		writeError(w, http.StatusBadRequest, "invalid_xml", "XML cannot be empty")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.provider.UpdateVMXML(ctx, r.PathValue("id"), body.XML); err != nil {
		writeError(w, http.StatusBadRequest, "vm_xml_failed", err.Error())
		return
	}
	s.log.Info("vm xml updated", "id", r.PathValue("id"))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) vmSnapshots(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.VMSnapshots(ctx, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "vm_snapshots_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) createVMSnapshot(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if !s.decode(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := s.provider.CreateVMSnapshot(ctx, r.PathValue("id"), body.Name, body.Description); err != nil {
		writeError(w, http.StatusBadRequest, "vm_snapshot_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (s *Server) restoreVMSnapshot(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := s.provider.RestoreVMSnapshot(ctx, r.PathValue("id"), r.PathValue("snapshot")); err != nil {
		writeError(w, http.StatusBadRequest, "vm_snapshot_restore_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) deleteVMSnapshot(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := s.provider.DeleteVMSnapshot(ctx, r.PathValue("id"), r.PathValue("snapshot")); err != nil {
		writeError(w, http.StatusBadRequest, "vm_snapshot_delete_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) vms(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.VMs(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) network(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.Network(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) users(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.Users(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var options model.UserCreateOptions
	if !s.decode(w, r, &options) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.provider.CreateUser(ctx, options); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "create_user_failed", err.Error())
		return
	}
	s.log.Info("user created", "name", options.Name)
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var body struct {
		Description string `json:"description"`
	}
	if !s.decode(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := r.PathValue("name")
	if err := s.provider.UpdateUser(ctx, name, body.Description); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "update_user_failed", err.Error())
		return
	}
	s.log.Info("user updated", "name", name)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) changeUserPassword(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if !s.decode(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := r.PathValue("name")
	if err := s.provider.ChangeUserPassword(ctx, name, body.Password); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "change_password_failed", err.Error())
		return
	}
	s.log.Info("user password changed", "name", name)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := r.PathValue("name")
	if err := s.provider.DeleteUser(ctx, name); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "delete_user_failed", err.Error())
		return
	}
	s.log.Info("user deleted", "name", name)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) userIcon(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	data, contentType, err := s.provider.UserIcon(ctx, r.PathValue("name"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	serveIcon(w, data, contentType)
}

func (s *Server) plugins(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.Plugins(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) checkPluginUpdates(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	plugins, err := s.provider.CheckPluginUpdates(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	updates := 0
	for _, plugin := range plugins {
		if plugin.UpdateAvailable {
			updates++
		}
	}
	s.log.Info("plugin update check completed", "plugins", len(plugins), "updates", updates)
	writeJSON(w, http.StatusOK, plugins)
}

// updatePlugin streams the plugin manager output as newline-delimited JSON.
func (s *Server) updatePlugin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming", "streaming is not supported")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(w)
	emit := func(event model.PluginEvent) {
		if err := encoder.Encode(event); err == nil {
			flusher.Flush()
		}
	}
	name := r.PathValue("name")
	if err := s.provider.UpdatePlugin(ctx, name, emit); err != nil {
		s.log.Error("plugin update failed", "plugin", name, "error", err)
	}
}

func (s *Server) installPlugin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var body struct {
		URL string `json:"url"`
	}
	if !s.decode(w, r, &body) {
		return
	}
	streamPluginCommand(w, r, s, func(ctx context.Context, emit func(model.PluginEvent)) error {
		return s.provider.InstallPlugin(ctx, body.URL, emit)
	})
}

func (s *Server) removePlugin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	name := r.PathValue("name")
	streamPluginCommand(w, r, s, func(ctx context.Context, emit func(model.PluginEvent)) error {
		return s.provider.RemovePlugin(ctx, name, emit)
	})
}

func streamPluginCommand(w http.ResponseWriter, r *http.Request, s *Server, run func(context.Context, func(model.PluginEvent)) error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming", "streaming is not supported")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	emit := func(event model.PluginEvent) {
		if err := encoder.Encode(event); err == nil {
			flusher.Flush()
		}
	}
	if err := run(ctx, emit); err != nil {
		s.log.Error("plugin command failed", "error", err)
	}
}

func (s *Server) zfsPools(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.ZFSPools(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) zfsScrub(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.provider.ScrubZFSPool(ctx, r.PathValue("name")); err != nil {
		writeError(w, http.StatusBadRequest, "zfs_scrub_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) zfsSnapshots(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.ZFSSnapshots(ctx, r.URL.Query().Get("dataset"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "zfs_snapshots_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) createZFSSnapshot(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var body struct {
		Dataset string `json:"dataset"`
		Name    string `json:"name"`
	}
	if !s.decode(w, r, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.provider.CreateZFSSnapshot(ctx, body.Dataset, body.Name); err != nil {
		writeError(w, http.StatusBadRequest, "zfs_snapshot_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}

func (s *Server) deleteZFSSnapshot(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.provider.DeleteZFSSnapshot(ctx, r.URL.Query().Get("dataset"), r.URL.Query().Get("name")); err != nil {
		writeError(w, http.StatusBadRequest, "zfs_snapshot_delete_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) userScripts(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.UserScripts(ctx)
	if err != nil {
		writeError(w, http.StatusNotFound, "user_scripts_unavailable", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) userScript(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.UserScript(ctx, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "user_script_not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"content": v})
}

func (s *Server) runUserScript(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	output, err := s.provider.RunUserScript(ctx, r.PathValue("id"))
	result := map[string]any{"output": output}
	if err != nil {
		result["error"] = err.Error()
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) auditLog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.audit.List(queryInt(r, "limit", 200, 1, 500)))
}

func (s *Server) testNotification(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	settings, err := s.provider.Settings(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	var draft model.Settings
	if err := json.NewDecoder(r.Body).Decode(&draft); err == nil && (draft.DiscordWebhook != "" || draft.NtfyURL != "" || draft.WebhookURL != "") {
		settings = draft
	}
	if err := notify.SendTest(ctx, settings); err != nil {
		writeError(w, http.StatusBadRequest, "notification_test_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) notifications(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.Notifications(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) parityHistory(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.ParityHistory(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) ups(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.UPS(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) upsSettings(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.UPSSettings(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) saveUPSSettings(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var settings model.UPSSettings
	if !s.decode(w, r, &settings) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.provider.SaveUPSSettings(ctx, settings); err != nil {
		writeError(w, http.StatusBadRequest, "ups_settings_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) gpus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	values, err := s.provider.GPUs(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) mover(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	status, err := s.provider.Mover(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) startMover(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.provider.StartMover(ctx); err != nil {
		writeError(w, http.StatusConflict, "mover_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) stopMover(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	if err := s.provider.StopMover(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "mover_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) logs(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	switch source {
	case "", "syslog", "docker", "parity", "nginx", "libvirt", "maillog":
	default:
		writeError(w, http.StatusBadRequest, "invalid_source", "source must be syslog, docker, parity, nginx, libvirt or maillog")
		return
	}
	if source == "" {
		source = "syslog"
	}
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.Logs(ctx, source, queryInt(r, "lines", 200, 10, 1000))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeError(w, http.StatusNotFound, "log_unavailable", err.Error())
			return
		}
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) balanceDisks(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	disks, err := s.provider.BalanceDisks(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, disks)
}

func (s *Server) balanceBrowse(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	entries, err := s.provider.BalanceBrowse(ctx, r.URL.Query().Get("disk"), r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "browse_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) balanceStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	job, err := s.provider.BalanceStatus(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) balanceHistory(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	history, err := s.provider.BalanceHistory(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, history)
}

func (s *Server) startBalance(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var options model.BalanceOptions
	if !s.decode(w, r, &options) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	job, err := s.provider.StartBalance(ctx, options)
	if err != nil {
		writeError(w, http.StatusConflict, "balance_failed", err.Error())
		return
	}
	s.log.Info("balance job started", "source", options.Source, "destinations", options.Destinations, "dryRun", options.DryRun)
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) cancelBalance(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.provider.CancelBalance(ctx); err != nil {
		writeError(w, http.StatusConflict, "cancel_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) configHistory(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	history, err := s.provider.ConfigHistory(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, history)
}

func (s *Server) configHistoryDetail(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	entry, err := s.provider.ConfigHistoryDetail(ctx, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) rollbackConfig(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	id := r.PathValue("id")
	if err := s.provider.RollbackConfig(ctx, id); err != nil {
		s.log.Error("config rollback failed", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "rollback_failed", err.Error())
		return
	}
	s.log.Info("config rollback applied", "id", id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.Settings(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var body model.Settings
	if !s.decode(w, r, &body) {
		return
	}
	if err := validateSettings(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_settings", err.Error())
		return
	}
	ctx, cancel := s.serverContext()
	defer cancel()
	if err := s.provider.SaveSettings(ctx, body); err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) storageLayout(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	layout, err := s.provider.StorageLayout(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, layout)
}

func (s *Server) saveStorageLayout(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	// The client timestamp is informational; accepting the model directly would
	// make an empty "updated":"" fail JSON decoding before validation.
	var body struct {
		Version int                 `json:"version"`
		Views   []model.StorageView `json:"views"`
		Updated any                 `json:"updated"`
	}
	if !s.decode(w, r, &body) {
		return
	}
	ctx, cancel := s.serverContext()
	defer cancel()
	layout := model.StorageLayout{Version: body.Version, Views: body.Views}
	if err := s.provider.SaveStorageLayout(ctx, layout); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_layout", err.Error())
		return
	}
	layout, err := s.provider.StorageLayout(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, layout)
}

func (s *Server) metricsHistory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.hub.History())
}

func (s *Server) action(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var body model.Action
	if !s.decode(w, r, &body) {
		return
	}
	if err := validateAction(body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_action", err.Error())
		return
	}
	ctx, cancel := s.serverContext()
	defer cancel()
	if err := s.provider.Perform(ctx, body); err != nil {
		s.log.Error("action failed", "kind", body.Kind, "target", body.Target, "error", err)
		writeError(w, http.StatusInternalServerError, "action_failed", err.Error())
		return
	}
	s.log.Info("action applied", "kind", body.Kind, "target", body.Target)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "action": body})
}

// events streams live metrics over Server-Sent Events.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming", "streaming is not supported")
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")

	ch, cancel := s.hub.Subscribe()
	defer cancel()

	fmt.Fprint(w, "retry: 3000\n\n")
	sendMeta, _ := json.Marshal(map[string]any{"mode": s.provider.Name(), "version": s.cfg.Version})
	fmt.Fprintf(w, "event: hello\ndata: %s\n\n", sendMeta)
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case m := <-ch:
			raw, err := json.Marshal(m)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: metrics\ndata: %s\n\n", raw)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func queryInt(r *http.Request, key string, fallback, min, max int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func validateSettings(s *model.Settings) error {
	if s.Theme == "" {
		s.Theme = "dark"
	}
	switch s.Theme {
	case "dark", "light", "system":
	default:
		return fmt.Errorf("theme must be dark, light or system")
	}
	if s.Accent == "" {
		s.Accent = "ember"
	}
	switch s.Accent {
	case "ember", "violet", "cyan", "emerald", "amber":
	default:
		return fmt.Errorf("unknown accent %q", s.Accent)
	}
	switch s.Density {
	case "", "comfortable", "compact":
	default:
		return fmt.Errorf("density must be comfortable or compact")
	}
	if s.RefreshInterval == 0 {
		s.RefreshInterval = 5
	}
	if s.RefreshInterval < 2 || s.RefreshInterval > 60 {
		return fmt.Errorf("refresh interval must be between 2 and 60 seconds")
	}
	switch s.TemperatureUnit {
	case "", "celsius", "fahrenheit":
	default:
		return fmt.Errorf("temperature unit must be celsius or fahrenheit")
	}
	return nil
}

func validateAction(a model.Action) error {
	allowed := map[string]bool{
		provider.ActionArrayStart: true, provider.ActionArrayStop: true, provider.ActionArrayPause: true,
		provider.ActionDiskSpinUp: true, provider.ActionDiskSpinDown: true,
		provider.ActionContainerStart: true, provider.ActionContainerStop: true,
		provider.ActionContainerRestart: true, provider.ActionContainerUpdate: true,
		provider.ActionVMStart: true, provider.ActionVMStop: true, provider.ActionVMPause: true, provider.ActionVMResume: true,
		provider.ActionParityStart: true, provider.ActionParityStop: true, provider.ActionParityPause: true, provider.ActionParityResume: true,
		provider.ActionNotifyAck: true, provider.ActionNotifyAckAll: true, provider.ActionNotifyArchive: true,
	}
	if !allowed[a.Kind] {
		return fmt.Errorf("unsupported action %q", a.Kind)
	}
	needsTarget := a.Kind != provider.ActionArrayStart && a.Kind != provider.ActionArrayStop && a.Kind != provider.ActionArrayPause &&
		a.Kind != provider.ActionParityStart && a.Kind != provider.ActionParityStop && a.Kind != provider.ActionParityPause &&
		a.Kind != provider.ActionParityResume && a.Kind != provider.ActionNotifyAckAll
	if needsTarget && strings.TrimSpace(a.Target) == "" {
		return fmt.Errorf("target is required for %s", a.Kind)
	}
	if len(a.Target) > 128 {
		return fmt.Errorf("target is too long")
	}
	for _, r := range a.Target {
		if unicode.IsControl(r) {
			return fmt.Errorf("target contains control characters")
		}
	}
	return nil
}
