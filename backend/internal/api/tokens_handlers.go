package api

import (
	"net/http"
	"strings"
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
		Name     string `json:"name"`
		Role     string `json:"role"`
		ReadOnly bool   `json:"readOnly"`
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
	token, secret, err := s.tokens.create(body.Name, role)
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
