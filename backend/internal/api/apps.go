package api

import (
	"net/http"
	"strconv"
)

func (s *Server) appsCatalog(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	query := r.URL.Query()
	limit, _ := strconv.Atoi(query.Get("limit"))
	offset, _ := strconv.Atoi(query.Get("offset"))
	page, err := s.provider.AppsCatalog(
		ctx,
		query.Get("query"),
		query.Get("kind"),
		query.Get("category"),
		query.Get("order"),
		limit,
		offset,
	)
	if err != nil {
		writeError(w, http.StatusBadGateway, "apps_catalog_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) appTemplate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	template, err := s.provider.AppTemplate(ctx, r.URL.Query().Get("url"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "app_template_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, template)
}

func (s *Server) appIcon(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.serverContext()
	defer cancel()
	raw, contentType, err := s.provider.AppIcon(ctx, r.URL.Query().Get("url"))
	if err != nil {
		writeError(w, http.StatusBadGateway, "app_icon_failed", err.Error())
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}
