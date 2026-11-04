package api

import (
	"net/http"
	"strings"
	"time"
)

func (s *Server) apiTokens(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.sessionUser(r); !ok {
		writeError(w, http.StatusForbidden, "session_required", "API token management requires a signed-in session")
		return
	}
	writeJSON(w, http.StatusOK, s.tokens.list())
}

func (s *Server) createAPIToken(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	if _, ok := s.sessionUser(r); !ok {
		writeError(w, http.StatusForbidden, "session_required", "API token management requires a signed-in session")
		return
	}
	var body struct {
		Name          string   `json:"name"`
		Role          string   `json:"role"`
		ReadOnly      bool     `json:"readOnly"`
		ExpiresInDays int      `json:"expiresInDays"`
		Scopes        []string `json:"scopes"`
	}
	if !s.decode(w, r, &body) {
		return
	}
	role := body.Role
	if strings.TrimSpace(role) == "" {
		// Keep older clients that only send readOnly working.
		if body.ReadOnly {
			role = TokenRoleViewer
		} else {
			role = TokenRoleAdmin
		}
	}
	var expiresAt *time.Time
	if body.ExpiresInDays < 0 || body.ExpiresInDays > 3650 {
		writeError(w, http.StatusBadRequest, "invalid_expiry", "expiry must be between 0 and 3650 days")
		return
	}
	if body.ExpiresInDays > 0 {
		value := time.Now().AddDate(0, 0, body.ExpiresInDays)
		expiresAt = &value
	}
	token, secret, err := s.tokens.create(body.Name, role, expiresAt, body.Scopes)
	if err != nil {
		writeError(w, http.StatusBadRequest, "token_create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": token, "secret": secret})
}

func (s *Server) revokeAPIToken(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		writeError(w, http.StatusForbidden, "read_only", "server is in read-only mode")
		return
	}
	if _, ok := s.sessionUser(r); !ok {
		writeError(w, http.StatusForbidden, "session_required", "API token management requires a signed-in session")
		return
	}
	if err := s.tokens.revoke(r.PathValue("id")); err != nil {
		writeError(w, http.StatusNotFound, "token_not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
