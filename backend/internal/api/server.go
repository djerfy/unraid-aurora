package api

import (
	"bufio"
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurora/internal/auth"
	"aurora/internal/config"
	"aurora/internal/metrics"
	"aurora/internal/provider"
	"aurora/internal/webui"
)

type Server struct {
	cfg      config.Config
	provider provider.Provider
	hub      *metrics.Hub
	auth     *auth.Manager
	log      *slog.Logger
	started  time.Time
}

func New(cfg config.Config, p provider.Provider, hub *metrics.Hub, authManager *auth.Manager, log *slog.Logger) *Server {
	return &Server{
		cfg: cfg, provider: p, hub: hub,
		auth:    authManager,
		log:     log,
		started: time.Now(),
	}
}

func (s *Server) Handler() http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("GET /api/v1/health", s.health)
	api.HandleFunc("GET /api/v1/auth/session", s.session)
	api.HandleFunc("POST /api/v1/auth/login", s.login)
	api.HandleFunc("POST /api/v1/auth/logout", s.logout)

	api.HandleFunc("GET /api/v1/bootstrap", s.bootstrap)
	api.HandleFunc("GET /api/v1/system", s.system)
	api.HandleFunc("GET /api/v1/array", s.array)
	api.HandleFunc("POST /api/v1/array/start", s.startArray)
	api.HandleFunc("POST /api/v1/array/stop", s.stopArray)
	api.HandleFunc("GET /api/v1/disks", s.disks)
	api.HandleFunc("GET /api/v1/shares", s.shares)
	api.HandleFunc("GET /api/v1/pools", s.pools)
	api.HandleFunc("GET /api/v1/docker/containers", s.containers)
	api.HandleFunc("GET /api/v1/docker/containers/{id}/logs", s.containerLogs)
	api.HandleFunc("GET /api/v1/docker/containers/{id}/icon", s.containerIcon)
	api.HandleFunc("GET /api/v1/docker/containers/{id}/config", s.containerConfig)
	api.HandleFunc("PUT /api/v1/docker/containers/{id}/config", s.updateContainerConfig)
	api.HandleFunc("POST /api/v1/docker/containers", s.createContainer)
	api.HandleFunc("DELETE /api/v1/docker/containers/{id}", s.removeContainer)
	api.HandleFunc("GET /api/v1/docker/networks", s.dockerNetworks)
	api.HandleFunc("POST /api/v1/docker/check-updates", s.checkContainerUpdates)
	api.HandleFunc("GET /api/v1/vms", s.vms)
	api.HandleFunc("GET /api/v1/vms/{id}/icon", s.vmIcon)
	api.HandleFunc("GET /api/v1/network", s.network)
	api.HandleFunc("GET /api/v1/users", s.users)
	api.HandleFunc("GET /api/v1/users/{name}/icon", s.userIcon)
	api.HandleFunc("GET /api/v1/plugins", s.plugins)
	api.HandleFunc("GET /api/v1/notifications", s.notifications)
	api.HandleFunc("GET /api/v1/parity/history", s.parityHistory)
	api.HandleFunc("GET /api/v1/ups", s.ups)
	api.HandleFunc("GET /api/v1/gpus", s.gpus)
	api.HandleFunc("GET /api/v1/mover", s.mover)
	api.HandleFunc("POST /api/v1/mover/start", s.startMover)
	api.HandleFunc("POST /api/v1/mover/stop", s.stopMover)
	api.HandleFunc("GET /api/v1/logs", s.logs)
	api.HandleFunc("GET /api/v1/settings", s.getSettings)
	api.HandleFunc("PUT /api/v1/settings", s.putSettings)
	api.HandleFunc("GET /api/v1/metrics/history", s.metricsHistory)
	api.HandleFunc("GET /api/v1/events", s.events)
	api.HandleFunc("GET /api/v1/terminal", s.terminal)
	api.HandleFunc("POST /api/v1/actions", s.action)

	root := http.NewServeMux()
	root.Handle("/api/", api)
	root.HandleFunc("GET /api", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"name": "aurora-api", "version": s.cfg.Version, "endpoints": "/api/v1",
		})
	})

	if ui, err := webui.Handler(); err == nil {
		root.Handle("/", ui)
	} else {
		s.log.Warn("web UI is unavailable", "error", err)
		root.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "UI build is missing: run `make build`", http.StatusServiceUnavailable)
		})
	}

	var handler http.Handler = root
	handler = s.authenticate(handler)
	handler = s.csrf(handler)
	handler = s.securityHeaders(handler)
	handler = s.accessLog(handler)
	handler = s.recoverPanic(handler)
	return handler
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublic(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if _, ok := s.auth.FromRequest(r); !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isPublic(path string) bool {
	switch path {
	case "/api/v1/health", "/api/v1/auth/session", "/api/v1/auth/login":
		return true
	}
	return !strings.HasPrefix(path, "/api/")
}

func (s *Server) csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("X-Aurora-Request") != "aurora" {
			writeError(w, http.StatusForbidden, "csrf", "missing request header")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || !sameHost(u.Host, r.Host) {
				writeError(w, http.StatusForbidden, "origin", "cross-origin request rejected")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func sameHost(a, b string) bool {
	// Ignore the port so a reverse proxy on another port still works.
	hostA, _, _ := strings.Cut(a, ":")
	hostB, _, _ := strings.Cut(b, ":")
	return strings.EqualFold(hostA, hostB) || (hostA == "127.0.0.1" && hostB == "localhost") || (hostA == "localhost" && hostB == "127.0.0.1")
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'; upgrade-insecure-requests")
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			s.log.Info("request",
				"method", r.Method, "path", r.URL.Path, "status", sw.status,
				"duration", time.Since(start).Round(time.Millisecond).String(),
				"ip", auth.ClientIP(r),
			)
		}
	})
}

func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panic recovered", "path", r.URL.Path, "panic", rec)
				writeError(w, http.StatusInternalServerError, "internal", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) sessionUser(r *http.Request) (auth.Session, bool) {
	return s.auth.FromRequest(r)
}

func (s *Server) secureCookie(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (s *Server) serverContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}
