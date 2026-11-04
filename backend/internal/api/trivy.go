package api

import (
	"context"
	"net/http"
	"time"
)

func (s *Server) trivyStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	status, err := s.provider.TrivyStatus(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "trivy_status_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) trivyVersions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	versions, err := s.provider.TrivyVersions(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "trivy_versions_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, versions)
}

func (s *Server) trivyInstall(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var body struct {
		Version string `json:"version"`
	}
	if !s.decode(w, r, &body) {
		return
	}
	if err := s.provider.StartTrivyInstall(body.Version); err != nil {
		writeError(w, http.StatusConflict, "trivy_install_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "started": true})
}

func (s *Server) trivyDBUpdate(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	if err := s.provider.StartTrivyDBUpdate(); err != nil {
		writeError(w, http.StatusConflict, "trivy_db_update_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "started": true})
}

func (s *Server) securityOverview(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	overview, err := s.provider.SecurityOverview(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "security_overview_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (s *Server) containerSecurity(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	detail, err := s.provider.ContainerSecurity(ctx, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "container_security_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) scanAllContainers(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	started, err := s.provider.ScanAllContainers(ctx)
	if err != nil {
		writeError(w, http.StatusConflict, "container_scan_all_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "started": started})
}

func (s *Server) scanContainer(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.provider.ScanContainer(ctx, r.PathValue("id")); err != nil {
		writeError(w, http.StatusBadRequest, "container_scan_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "started": true})
}
