package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"aurora/internal/auth"
	"aurora/internal/model"
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

func (s *Server) logs(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	switch source {
	case "", "syslog", "docker", "parity":
	default:
		writeError(w, http.StatusBadRequest, "invalid_source", "source must be syslog, docker or parity")
		return
	}
	if source == "" {
		source = "syslog"
	}
	ctx, cancel := s.serverContext()
	defer cancel()
	v, err := s.provider.Logs(ctx, source, queryInt(r, "lines", 200, 10, 1000))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
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
