package api

import (
	"net/http"

	"aurora/internal/model"
)

func (s *Server) schedulerState(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	state, err := s.provider.SchedulerState(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "scheduler_state_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) schedulerPreview(w http.ResponseWriter, r *http.Request) {
	var settings model.SchedulerSettings
	if !s.decode(w, r, &settings) {
		return
	}
	preview, err := s.provider.PreviewScheduler(settings)
	if err != nil {
		writeError(w, http.StatusBadRequest, "scheduler_preview_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (s *Server) schedulerApply(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var body model.SchedulerApply
	if !s.decode(w, r, &body) {
		return
	}
	if !body.Acknowledge {
		writeError(w, http.StatusBadRequest, "scheduler_not_acknowledged", "explicit confirmation is required")
		return
	}
	ctx, cancel := s.serverContext()
	defer cancel()
	if err := s.provider.ApplyScheduler(ctx, body.SchedulerSettings); err != nil {
		writeError(w, http.StatusBadGateway, "scheduler_apply_failed", err.Error())
		return
	}
	state, err := s.provider.SchedulerState(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "warning": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "state": state})
}

func (s *Server) displaySettingsApply(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var body model.DisplayApply
	if !s.decode(w, r, &body) {
		return
	}
	if !body.Acknowledge {
		writeError(w, http.StatusBadRequest, "display_not_acknowledged", "explicit confirmation is required")
		return
	}
	ctx, cancel := s.serverContext()
	defer cancel()
	if err := s.provider.UpdateDisplaySettings(ctx, body.SystemDisplaySettings); err != nil {
		writeError(w, http.StatusBadGateway, "display_apply_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
