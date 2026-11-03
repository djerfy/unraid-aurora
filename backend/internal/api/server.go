package api

import (
	"bufio"
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurora/internal/audit"
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
	audit    *audit.Log
}

func New(cfg config.Config, p provider.Provider, hub *metrics.Hub, authManager *auth.Manager, log *slog.Logger) *Server {
	return &Server{
		cfg: cfg, provider: p, hub: hub,
		auth:    authManager,
		log:     log,
		started: time.Now(),
		audit:   audit.New(cfg.ConfigDir),
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
	api.HandleFunc("GET /api/v1/disks/{device}/smart", s.diskSmart)
	api.HandleFunc("POST /api/v1/disks/{device}/smart-test", s.diskSmartTest)
	api.HandleFunc("GET /api/v1/shares", s.shares)
	api.HandleFunc("POST /api/v1/shares", s.createShare)
	api.HandleFunc("GET /api/v1/shares/{name}/config", s.shareConfig)
	api.HandleFunc("PUT /api/v1/shares/{name}", s.updateShare)
	api.HandleFunc("PUT /api/v1/shares/{name}/smb", s.updateShareSMB)
	api.HandleFunc("PUT /api/v1/shares/{name}/nfs", s.updateShareNFS)
	api.HandleFunc("GET /api/v1/pools", s.pools)
	api.HandleFunc("GET /api/v1/docker/containers", s.containers)
	api.HandleFunc("GET /api/v1/docker/containers/{id}/logs", s.containerLogs)
	api.HandleFunc("GET /api/v1/docker/containers/{id}/connections", s.containerConnections)
	api.HandleFunc("GET /api/v1/docker/containers/{id}/icon", s.containerIcon)
	api.HandleFunc("GET /api/v1/docker/containers/{id}/config", s.containerConfig)
	api.HandleFunc("PUT /api/v1/docker/containers/{id}/config", s.updateContainerConfig)
	api.HandleFunc("POST /api/v1/docker/containers", s.createContainer)
	api.HandleFunc("POST /api/v1/docker/containers/{id}/update", s.updateContainerImage)
	api.HandleFunc("PUT /api/v1/docker/containers/{id}/autostart", s.containerAutostart)
	api.HandleFunc("DELETE /api/v1/docker/containers/{id}", s.removeContainer)
	api.HandleFunc("GET /api/v1/docker/networks", s.dockerNetworks)
	api.HandleFunc("GET /api/v1/docker/templates", s.containerTemplates)
	api.HandleFunc("POST /api/v1/docker/check-updates", s.checkContainerUpdates)
	api.HandleFunc("GET /api/v1/vms", s.vms)
	api.HandleFunc("POST /api/v1/vms", s.createVM)
	api.HandleFunc("GET /api/v1/vms/{id}/icon", s.vmIcon)
	api.HandleFunc("GET /api/v1/vms/{id}/console", s.vmConsole)
	api.HandleFunc("PUT /api/v1/vms/{id}/autostart", s.vmAutostart)
	api.HandleFunc("GET /api/v1/vms/{id}/xml", s.vmXML)
	api.HandleFunc("PUT /api/v1/vms/{id}/xml", s.updateVMXML)
	api.HandleFunc("GET /api/v1/vms/{id}/snapshots", s.vmSnapshots)
	api.HandleFunc("POST /api/v1/vms/{id}/snapshots", s.createVMSnapshot)
	api.HandleFunc("POST /api/v1/vms/{id}/snapshots/{snapshot}/revert", s.restoreVMSnapshot)
	api.HandleFunc("DELETE /api/v1/vms/{id}/snapshots/{snapshot}", s.deleteVMSnapshot)
	api.HandleFunc("GET /api/v1/network", s.network)
	api.HandleFunc("GET /api/v1/users", s.users)
	api.HandleFunc("POST /api/v1/users", s.createUser)
	api.HandleFunc("PUT /api/v1/users/{name}", s.updateUser)
	api.HandleFunc("PUT /api/v1/users/{name}/password", s.changeUserPassword)
	api.HandleFunc("DELETE /api/v1/users/{name}", s.deleteUser)
	api.HandleFunc("GET /api/v1/users/{name}/icon", s.userIcon)
	api.HandleFunc("GET /api/v1/plugins", s.plugins)
	api.HandleFunc("POST /api/v1/plugins/check-updates", s.checkPluginUpdates)
	api.HandleFunc("POST /api/v1/plugins/{name}/update", s.updatePlugin)
	api.HandleFunc("POST /api/v1/plugins/install", s.installPlugin)
	api.HandleFunc("DELETE /api/v1/plugins/{name}", s.removePlugin)
	api.HandleFunc("GET /api/v1/notifications", s.notifications)
	api.HandleFunc("GET /api/v1/audit", s.auditLog)
	api.HandleFunc("GET /api/v1/user-scripts", s.userScripts)
	api.HandleFunc("GET /api/v1/user-scripts/{id}", s.userScript)
	api.HandleFunc("POST /api/v1/user-scripts/{id}/run", s.runUserScript)
	api.HandleFunc("GET /api/v1/zfs/pools", s.zfsPools)
	api.HandleFunc("POST /api/v1/zfs/pools/{name}/scrub", s.zfsScrub)
	api.HandleFunc("GET /api/v1/zfs/snapshots", s.zfsSnapshots)
	api.HandleFunc("POST /api/v1/zfs/snapshots", s.createZFSSnapshot)
	api.HandleFunc("DELETE /api/v1/zfs/snapshots", s.deleteZFSSnapshot)
	api.HandleFunc("POST /api/v1/notifications/test", s.testNotification)
	api.HandleFunc("GET /api/v1/parity/history", s.parityHistory)
	api.HandleFunc("GET /api/v1/ups", s.ups)
	api.HandleFunc("GET /api/v1/ups/settings", s.upsSettings)
	api.HandleFunc("PUT /api/v1/ups/settings", s.saveUPSSettings)
	api.HandleFunc("GET /api/v1/gpus", s.gpus)
	api.HandleFunc("GET /api/v1/mover", s.mover)
	api.HandleFunc("POST /api/v1/mover/start", s.startMover)
	api.HandleFunc("POST /api/v1/mover/stop", s.stopMover)
	api.HandleFunc("GET /api/v1/logs", s.logs)
	api.HandleFunc("GET /api/v1/settings", s.getSettings)
	api.HandleFunc("PUT /api/v1/settings", s.putSettings)
	api.HandleFunc("GET /api/v1/storage-layout", s.storageLayout)
	api.HandleFunc("PUT /api/v1/storage-layout", s.saveStorageLayout)
	api.HandleFunc("GET /api/v1/metrics/history", s.metricsHistory)
	api.HandleFunc("GET /api/v1/events", s.events)
	api.HandleFunc("GET /api/v1/tools/diagnostics", s.diagnostics)
	api.HandleFunc("GET /api/v1/tools/flash-backup", s.flashBackup)
	api.HandleFunc("GET /api/v1/config/export", s.configExport)
	api.HandleFunc("POST /api/v1/config/import", s.configImport)
	api.HandleFunc("GET /api/v1/balance/disks", s.balanceDisks)
	api.HandleFunc("GET /api/v1/balance/browse", s.balanceBrowse)
	api.HandleFunc("GET /api/v1/balance/status", s.balanceStatus)
	api.HandleFunc("GET /api/v1/balance/history", s.balanceHistory)
	api.HandleFunc("POST /api/v1/balance/start", s.startBalance)
	api.HandleFunc("POST /api/v1/balance/cancel", s.cancelBalance)
	api.HandleFunc("GET /api/v1/config-history", s.configHistory)
	api.HandleFunc("GET /api/v1/config-history/{id}", s.configHistoryDetail)
	api.HandleFunc("POST /api/v1/config-history/{id}/rollback", s.rollbackConfig)
	api.HandleFunc("GET /api/v1/terminal", s.terminal)
	api.HandleFunc("POST /api/v1/actions", s.action)

	root := http.NewServeMux()
	root.HandleFunc("GET /metrics", s.prometheus)
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
	handler = s.auditMutations(handler)
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
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: https://avatars.githubusercontent.com; style-src 'self' 'unsafe-inline'; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'; upgrade-insecure-requests")
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

func (s *Server) prometheus(w http.ResponseWriter, r *http.Request) {
	if s.cfg.MetricsToken == "" {
		http.NotFound(w, r)
		return
	}
	header := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if header == "" || subtle.ConstantTimeCompare([]byte(header), []byte(s.cfg.MetricsToken)) != 1 {
		writeError(w, http.StatusUnauthorized, "unauthorized", "metrics token is invalid")
		return
	}
	ctx, cancel := s.serverContext()
	defer cancel()
	metrics, err := s.provider.Metrics(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintf(w, "# HELP aurora_cpu_usage_percent Current CPU usage.\n")
	fmt.Fprintf(w, "# TYPE aurora_cpu_usage_percent gauge\naurora_cpu_usage_percent %.2f\n", metrics.CPU)
	fmt.Fprintf(w, "# HELP aurora_memory_usage_percent Current memory usage.\n")
	fmt.Fprintf(w, "# TYPE aurora_memory_usage_percent gauge\naurora_memory_usage_percent %.2f\n", metrics.Memory)
	fmt.Fprintf(w, "# HELP aurora_cpu_temperature_celsius CPU package temperature.\n")
	fmt.Fprintf(w, "# TYPE aurora_cpu_temperature_celsius gauge\naurora_cpu_temperature_celsius %.1f\n", metrics.Temp)
	fmt.Fprintf(w, "# HELP aurora_load_1m One-minute load average.\n")
	fmt.Fprintf(w, "# TYPE aurora_load_1m gauge\naurora_load_1m %.2f\n", metrics.Load)
	fmt.Fprintf(w, "# HELP aurora_network_receive_bytes_per_second Receive rate.\n")
	fmt.Fprintf(w, "# TYPE aurora_network_receive_bytes_per_second gauge\naurora_network_receive_bytes_per_second %.0f\n", metrics.NetRx)
	fmt.Fprintf(w, "# HELP aurora_network_transmit_bytes_per_second Transmit rate.\n")
	fmt.Fprintf(w, "# TYPE aurora_network_transmit_bytes_per_second gauge\naurora_network_transmit_bytes_per_second %.0f\n", metrics.NetTx)
	fmt.Fprintf(w, "# HELP aurora_disk_read_bytes_per_second Disk read rate.\n")
	fmt.Fprintf(w, "# TYPE aurora_disk_read_bytes_per_second gauge\naurora_disk_read_bytes_per_second %.0f\n", metrics.DiskRead)
	fmt.Fprintf(w, "# HELP aurora_disk_write_bytes_per_second Disk write rate.\n")
	fmt.Fprintf(w, "# TYPE aurora_disk_write_bytes_per_second gauge\naurora_disk_write_bytes_per_second %.0f\n", metrics.DiskWrite)
	if ups, err := s.provider.UPS(ctx); err == nil && ups.Present {
		onBattery := 0
		if ups.OnBattery {
			onBattery = 1
		}
		fmt.Fprintf(w, "# HELP aurora_ups_battery_percent UPS battery charge.\n# TYPE aurora_ups_battery_percent gauge\naurora_ups_battery_percent %.0f\n", ups.Battery)
		fmt.Fprintf(w, "# HELP aurora_ups_load_percent UPS load.\n# TYPE aurora_ups_load_percent gauge\naurora_ups_load_percent %.0f\n", ups.Load)
		fmt.Fprintf(w, "# HELP aurora_ups_on_battery Whether the UPS is on battery.\n# TYPE aurora_ups_on_battery gauge\naurora_ups_on_battery %d\n", onBattery)
	}
}

func (s *Server) auditMutations(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		user := "anonymous"
		if sess, ok := s.sessionUser(r); ok {
			user = sess.User
		}
		s.audit.Record(audit.Entry{
			Time: time.Now(), User: user, Method: r.Method, Path: r.URL.Path,
			Status: sw.status, Remote: auth.ClientIP(r),
		})
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
