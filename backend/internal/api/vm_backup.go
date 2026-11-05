package api

import (
	"net/http"
)

func (s *Server) startVMBackup(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var body struct {
		Dir string `json:"dir"`
	}
	if !s.decode(w, r, &body) {
		return
	}
	ctx, cancel := s.serverContext()
	defer cancel()
	job, err := s.provider.StartVMBackup(ctx, r.PathValue("id"), body.Dir)
	if err != nil {
		writeError(w, http.StatusBadRequest, "vm_backup", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) vmBackupStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	job, err := s.provider.VMBackupStatus(ctx, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "vm_backup", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) cancelVMBackup(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	ctx, cancel := s.serverContext()
	defer cancel()
	if err := s.provider.CancelVMBackup(ctx, r.PathValue("id")); err != nil {
		writeError(w, http.StatusBadRequest, "vm_backup", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) restoreVMBackup(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var body struct {
		Dir     string `json:"dir"`
		Confirm bool   `json:"confirm"`
	}
	if !s.decode(w, r, &body) {
		return
	}
	ctx, cancel := s.serverContext()
	defer cancel()
	job, err := s.provider.StartVMRestore(ctx, r.PathValue("id"), body.Dir, body.Confirm)
	if err != nil {
		writeError(w, http.StatusBadRequest, "vm_restore", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) vmAutostartOrder(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	order, err := s.provider.VMAutostartOrder(ctx)
	if err != nil {
		writeError(w, http.StatusBadRequest, "vm_autostart", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (s *Server) setVMAutostartOrder(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var body struct {
		Order   []string `json:"order"`
		Managed bool     `json:"managed"`
	}
	if !s.decode(w, r, &body) {
		return
	}
	ctx, cancel := s.serverContext()
	defer cancel()
	order, err := s.provider.SetVMAutostartOrder(ctx, body.Order, body.Managed)
	if err != nil {
		writeError(w, http.StatusBadRequest, "vm_autostart", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (s *Server) applyVMAutostartOrder(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	ctx, cancel := s.serverContext()
	defer cancel()
	result, err := s.provider.ApplyVMAutostartOrder(ctx)
	if err != nil {
		writeError(w, http.StatusBadRequest, "vm_autostart", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}
