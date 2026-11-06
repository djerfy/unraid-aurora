package api

import (
	"net/http"

	"aurora/internal/model"
)

func (s *Server) containerFolders(w http.ResponseWriter, r *http.Request) {
	s.readVisualFolders(w, r, "containers")
}

func (s *Server) saveContainerFolders(w http.ResponseWriter, r *http.Request) {
	s.writeVisualFolders(w, r, "containers")
}

func (s *Server) vmFolders(w http.ResponseWriter, r *http.Request) {
	s.readVisualFolders(w, r, "vms")
}

func (s *Server) saveVMFolders(w http.ResponseWriter, r *http.Request) {
	s.writeVisualFolders(w, r, "vms")
}

func (s *Server) readVisualFolders(w http.ResponseWriter, r *http.Request, kind string) {
	ctx, cancel := s.serverContext()
	defer cancel()
	layout, err := s.provider.VisualFolders(ctx, kind)
	if err != nil {
		writeError(w, http.StatusBadRequest, "folders", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, layout)
}

func (s *Server) writeVisualFolders(w http.ResponseWriter, r *http.Request, kind string) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	var layout model.VisualFolders
	if !s.decode(w, r, &layout) {
		return
	}
	ctx, cancel := s.serverContext()
	defer cancel()
	if err := s.provider.SaveVisualFolders(ctx, kind, layout); err != nil {
		writeError(w, http.StatusBadRequest, "folders", err.Error())
		return
	}
	saved, err := s.provider.VisualFolders(ctx, kind)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "folders", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, saved)
}
