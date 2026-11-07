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
	"aurora/internal/mcpserver"
	"aurora/internal/metrics"
	"aurora/internal/model"
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
	tokens   *tokenStore
}

func New(cfg config.Config, p provider.Provider, hub *metrics.Hub, authManager *auth.Manager, log *slog.Logger) *Server {
	return &Server{
		cfg: cfg, provider: p, hub: hub,
		auth:    authManager,
		log:     log,
		started: time.Now(),
		audit:   audit.New(cfg.ConfigDir),
		tokens:  newTokenStore(cfg.ConfigDir),
	}
}

func (s *Server) Handler() http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("GET /api/docs", s.apiDocs)
	api.HandleFunc("GET /api/openapi.json", s.openAPISpec)
	api.HandleFunc("GET /api/v1/health", s.health)
	api.HandleFunc("GET /api/v1/auth/session", s.session)
	api.HandleFunc("POST /api/v1/auth/login", s.login)
	api.HandleFunc("POST /api/v1/auth/logout", s.logout)
	api.HandleFunc("POST /api/v1/auth/revoke-all", s.revokeAllSessions)
	api.HandleFunc("GET /api/v1/auth/sessions", s.authSessions)
	api.HandleFunc("DELETE /api/v1/auth/sessions/{nonce}", s.revokeSession)

	api.HandleFunc("GET /api/v1/bootstrap", s.bootstrap)
	api.HandleFunc("GET /api/v1/system", s.system)
	api.HandleFunc("GET /api/v1/apps", s.appsCatalog)
	api.HandleFunc("GET /api/v1/apps/template", s.appTemplate)
	api.HandleFunc("GET /api/v1/apps/icon", s.appIcon)
	api.HandleFunc("GET /api/v1/system/scheduler", s.schedulerState)
	api.HandleFunc("POST /api/v1/system/scheduler/preview", s.schedulerPreview)
	api.HandleFunc("PUT /api/v1/system/scheduler", s.schedulerApply)
	api.HandleFunc("PUT /api/v1/system/display", s.displaySettingsApply)
	api.HandleFunc("GET /api/v1/array", s.array)
	api.HandleFunc("POST /api/v1/array/start", s.startArray)
	api.HandleFunc("POST /api/v1/array/stop", s.stopArray)
	api.HandleFunc("GET /api/v1/disks", s.disks)
	api.HandleFunc("GET /api/v1/disks/{device}/smart", s.diskSmart)
	api.HandleFunc("POST /api/v1/disks/{device}/smart-test", s.diskSmartTest)
	api.HandleFunc("GET /api/v1/shares", s.shares)
	api.HandleFunc("POST /api/v1/shares", s.createShare)
	api.HandleFunc("POST /api/v1/shares/usage/refresh", s.refreshShareUsage)
	api.HandleFunc("GET /api/v1/shares/{name}/config", s.shareConfig)
	api.HandleFunc("PUT /api/v1/shares/{name}", s.updateShare)
	api.HandleFunc("PUT /api/v1/shares/{name}/smb", s.updateShareSMB)
	api.HandleFunc("PUT /api/v1/shares/{name}/nfs", s.updateShareNFS)
	api.HandleFunc("GET /api/v1/pools", s.pools)
	api.HandleFunc("GET /api/v1/docker/containers", s.containers)
	api.HandleFunc("GET /api/v1/docker/containers/{id}/logs", s.containerLogs)
	api.HandleFunc("GET /api/v1/docker/containers/{id}/connections", s.containerConnections)
	api.HandleFunc("GET /api/v1/docker/network-map", s.containerNetworkMap)
	api.HandleFunc("GET /api/v1/docker/containers/{id}/icon", s.containerIcon)
	api.HandleFunc("GET /api/v1/docker/containers/{id}/config", s.containerConfig)
	api.HandleFunc("GET /api/v1/docker/security", s.securityOverview)
	api.HandleFunc("GET /api/v1/docker/containers/{id}/security", s.containerSecurity)
	api.HandleFunc("POST /api/v1/docker/containers/{id}/scan", s.scanContainer)
	api.HandleFunc("POST /api/v1/docker/scan-all", s.scanAllContainers)
	api.HandleFunc("PUT /api/v1/docker/containers/{id}/config", s.updateContainerConfig)
	api.HandleFunc("POST /api/v1/docker/containers", s.createContainer)
	api.HandleFunc("POST /api/v1/docker/containers/bulk", s.bulkContainerAction)
	api.HandleFunc("PUT /api/v1/docker/priorities", s.saveContainerPriorities)
	api.HandleFunc("GET /api/v1/docker/images", s.containerImages)
	api.HandleFunc("DELETE /api/v1/docker/images/{id}", s.removeContainerImage)
	api.HandleFunc("POST /api/v1/docker/images/prune", s.pruneContainerImages)
	api.HandleFunc("GET /api/v1/docker/settings", s.dockerSettings)
	api.HandleFunc("PUT /api/v1/docker/settings", s.saveDockerSettings)
	api.HandleFunc("POST /api/v1/docker/containers/{id}/update", s.updateContainerImage)
	api.HandleFunc("PUT /api/v1/docker/containers/{id}/autostart", s.containerAutostart)
	api.HandleFunc("DELETE /api/v1/docker/containers/{id}", s.removeContainer)
	api.HandleFunc("GET /api/v1/docker/networks", s.dockerNetworks)
	api.HandleFunc("GET /api/v1/docker/templates", s.containerTemplates)
	api.HandleFunc("GET /api/v1/docker/folders", s.containerFolders)
	api.HandleFunc("PUT /api/v1/docker/folders", s.saveContainerFolders)
	api.HandleFunc("POST /api/v1/docker/check-updates", s.checkContainerUpdates)
	api.HandleFunc("GET /api/v1/vms", s.vms)
	api.HandleFunc("POST /api/v1/vms", s.createVM)
	api.HandleFunc("GET /api/v1/vms/host-devices", s.vmHostDevices)
	api.HandleFunc("GET /api/v1/vms/settings", s.vmSettings)
	api.HandleFunc("PUT /api/v1/vms/settings", s.saveVMSettings)
	api.HandleFunc("GET /api/v1/vms/{id}/icon", s.vmIcon)
	api.HandleFunc("GET /api/v1/vms/{id}/advanced", s.vmAdvanced)
	api.HandleFunc("PUT /api/v1/vms/{id}/advanced", s.updateVMAdvanced)
	api.HandleFunc("POST /api/v1/vms/{id}/clone", s.cloneVM)
	api.HandleFunc("POST /api/v1/vms/{id}/resize", s.resizeVMDisk)
	api.HandleFunc("POST /api/v1/vms/{id}/usb/attach", s.attachVMUSB)
	api.HandleFunc("POST /api/v1/vms/{id}/usb/detach", s.detachVMUSB)
	api.HandleFunc("GET /api/v1/vms/{id}/console", s.vmConsole)
	api.HandleFunc("GET /api/v1/vms/{id}/logs", s.vmLogs)
	api.HandleFunc("POST /api/v1/vms/{id}/backup", s.startVMBackup)
	api.HandleFunc("GET /api/v1/vms/{id}/backup", s.vmBackupStatus)
	api.HandleFunc("POST /api/v1/vms/{id}/backup/cancel", s.cancelVMBackup)
	api.HandleFunc("POST /api/v1/vms/{id}/restore", s.restoreVMBackup)
	api.HandleFunc("GET /api/v1/vms/folders", s.vmFolders)
	api.HandleFunc("PUT /api/v1/vms/folders", s.saveVMFolders)
	api.HandleFunc("GET /api/v1/vms/autostart-order", s.vmAutostartOrder)
	api.HandleFunc("PUT /api/v1/vms/autostart-order", s.setVMAutostartOrder)
	api.HandleFunc("POST /api/v1/vms/autostart-order/apply", s.applyVMAutostartOrder)
	api.HandleFunc("PUT /api/v1/vms/{id}/autostart", s.vmAutostart)
	api.HandleFunc("GET /api/v1/vms/{id}/xml", s.vmXML)
	api.HandleFunc("PUT /api/v1/vms/{id}/xml", s.updateVMXML)
	api.HandleFunc("GET /api/v1/vms/{id}/snapshots", s.vmSnapshots)
	api.HandleFunc("POST /api/v1/vms/{id}/snapshots", s.createVMSnapshot)
	api.HandleFunc("POST /api/v1/vms/{id}/snapshots/{snapshot}/revert", s.restoreVMSnapshot)
	api.HandleFunc("DELETE /api/v1/vms/{id}/snapshots/{snapshot}", s.deleteVMSnapshot)
	api.HandleFunc("GET /api/v1/network", s.network)
	api.HandleFunc("GET /api/v1/network/config", s.networkConfig)
	api.HandleFunc("PUT /api/v1/network/config", s.saveNetworkConfig)
	api.HandleFunc("POST /api/v1/network/apply", s.applyNetworkConfig)
	api.HandleFunc("POST /api/v1/network/revert", s.revertNetworkConfig)
	api.HandleFunc("POST /api/v1/network/interfaces/{name}/{action}", s.networkInterfaceAction)
	api.HandleFunc("GET /api/v1/system/os", s.osStatus)
	api.HandleFunc("POST /api/v1/system/os/check", s.osCheck)
	api.HandleFunc("POST /api/v1/system/os/update", s.osUpdate)
	api.HandleFunc("POST /api/v1/system/os/restore", s.osRestore)
	api.HandleFunc("GET /api/v1/trivy", s.trivyStatus)
	api.HandleFunc("GET /api/v1/trivy/versions", s.trivyVersions)
	api.HandleFunc("POST /api/v1/trivy/install", s.trivyInstall)
	api.HandleFunc("POST /api/v1/trivy/db/update", s.trivyDBUpdate)
	api.HandleFunc("GET /api/v1/unraid-api", s.unraidAPIStatus)
	api.HandleFunc("GET /api/v1/unraid-api/logs", s.unraidAPILogs)
	api.HandleFunc("POST /api/v1/unraid-api/{action}", s.unraidAPIAction)
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
	api.HandleFunc("GET /api/v1/tasks", s.tasks)
	api.HandleFunc("GET /api/v1/tokens", s.apiTokens)
	api.HandleFunc("POST /api/v1/tokens", s.createAPIToken)
	api.HandleFunc("DELETE /api/v1/tokens/{id}", s.revokeAPIToken)
	api.HandleFunc("GET /api/v1/user-scripts", s.userScripts)
	api.HandleFunc("GET /api/v1/user-scripts/{id}", s.userScript)
	api.HandleFunc("PUT /api/v1/user-scripts/{id}", s.updateUserScript)
	api.HandleFunc("POST /api/v1/user-scripts/{id}/run", s.runUserScript)
	api.HandleFunc("GET /api/v1/zfs/pools", s.zfsPools)
	api.HandleFunc("POST /api/v1/zfs/pools/{name}/scrub", s.zfsScrub)
	api.HandleFunc("GET /api/v1/zfs/snapshots", s.zfsSnapshots)
	api.HandleFunc("POST /api/v1/zfs/snapshots", s.createZFSSnapshot)
	api.HandleFunc("DELETE /api/v1/zfs/snapshots", s.deleteZFSSnapshot)
	api.HandleFunc("POST /api/v1/zfs/snapshots/rollback", s.rollbackZFSSnapshot)
	api.HandleFunc("POST /api/v1/zfs/snapshots/clone", s.cloneZFSSnapshot)
	api.HandleFunc("POST /api/v1/zfs/datasets", s.createZFSDataset)
	api.HandleFunc("PUT /api/v1/zfs/datasets", s.updateZFSDataset)
	api.HandleFunc("DELETE /api/v1/zfs/datasets", s.deleteZFSDataset)
	api.HandleFunc("POST /api/v1/notifications/test", s.testNotification)
	api.HandleFunc("GET /api/v1/parity/history", s.parityHistory)
	api.HandleFunc("GET /api/v1/ups", s.ups)
	api.HandleFunc("GET /api/v1/ups/settings", s.upsSettings)
	api.HandleFunc("PUT /api/v1/ups/settings", s.saveUPSSettings)
	api.HandleFunc("GET /api/v1/ups/nut", s.nutSettings)
	api.HandleFunc("PUT /api/v1/ups/nut", s.saveNUTSettings)
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

	mcpHandler := mcpserver.HTTPHandler(func(r *http.Request) mcpserver.Options {
		principal, _ := r.Context().Value(tokenContextKey{}).(tokenPrincipal)
		role := principal.Role
		if role == "" {
			// Cookie sessions belong to the root operator.
			role = TokenRoleAdmin
		}
		readOnly := s.cfg.ReadOnly || principal.ReadOnly
		user := s.auditUser(r)
		remote := auth.ClientIP(r)
		return mcpserver.Options{
			Provider:     s.provider,
			Version:      s.cfg.Version,
			ReadOnly:     readOnly,
			AllowActions: !readOnly && (role == TokenRoleAdmin || role == TokenRoleOperator),
			Role:         role,
			Scopes:       principal.Scopes,
			OnAction: func(tool, target string, err error) {
				status := http.StatusOK
				if err != nil {
					status = http.StatusInternalServerError
				}
				s.audit.Record(audit.Entry{
					Time: time.Now(), User: user, Method: "MCP",
					Path: "/api/v1/mcp/" + tool + "?" + target, Status: status, Remote: remote,
				})
			},
		}
	})
	root.HandleFunc("/api/v1/mcp", func(w http.ResponseWriter, r *http.Request) {
		settings, err := s.provider.Settings(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "settings", err.Error())
			return
		}
		if !settings.MCPEnabled {
			writeError(w, http.StatusForbidden, "mcp_disabled", "the MCP server is disabled in Settings")
			return
		}
		mcpHandler.ServeHTTP(w, r)
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
	handler = s.compress(handler)
	return handler
}

type tokenPrincipal struct {
	Name     string
	Role     string
	ReadOnly bool
	Scopes   []string
}

// tokenRoleAllows applies the token role policy. Viewers are read-only,
// operators can run day-to-day workload and array actions, and admins retain
// full control over host configuration.
func tokenRoleAllows(role string, scopes []string, method, path string) bool {
	// MCP authenticates every valid token; the tool scope checks happen inside
	// the MCP server, which returns a precise error for scoped tokens.
	if path == "/api/v1/mcp" {
		return true
	}
	if !tokenScopeAllows(scopes, path) {
		return false
	}
	if handled, allowed := sensitiveReadAllows(role, path); handled {
		return allowed
	}
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	switch role {
	case TokenRoleAdmin:
		return true
	case TokenRoleOperator:
		for _, prefix := range []string{
			"/api/v1/actions",
			"/api/v1/array/",
			"/api/v1/docker/",
			"/api/v1/vms/",
			"/api/v1/mover",
			"/api/v1/balance/",
			"/api/v1/disks/",
		} {
			if strings.HasPrefix(path, prefix) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// sensitiveReadAllows keeps secrets and privileged consoles away from tokens
// that are only meant to read dashboards. It applies to every method so a PUT
// on the same resource follows the same role floor.
func sensitiveReadAllows(role, path string) (bool, bool) {
	adminOnly := func() (bool, bool) { return true, role == TokenRoleAdmin }
	operator := func() (bool, bool) { return true, role == TokenRoleAdmin || role == TokenRoleOperator }
	switch {
	case path == "/api/v1/terminal":
		return adminOnly()
	case strings.HasPrefix(path, "/api/v1/user-scripts/"):
		return adminOnly()
	case strings.HasPrefix(path, "/api/v1/config-history"):
		return adminOnly()
	case path == "/api/v1/settings", path == "/api/v1/audit", path == "/api/v1/ups/settings", path == "/api/v1/logs", path == "/api/v1/unraid-api/logs":
		return adminOnly()
	case strings.HasPrefix(path, "/api/v1/config/export"), path == "/api/v1/tools/flash-backup", path == "/api/v1/tools/diagnostics":
		return adminOnly()
	case strings.HasPrefix(path, "/api/v1/users"):
		if strings.HasSuffix(path, "/icon") {
			return false, false
		}
		return adminOnly()
	case strings.HasPrefix(path, "/api/v1/vms/") && (strings.HasSuffix(path, "/console") || strings.HasSuffix(path, "/xml") || strings.HasSuffix(path, "/advanced")):
		return operator()
	case strings.HasPrefix(path, "/api/v1/docker/containers/") && strings.HasSuffix(path, "/config"):
		return operator()
	}
	return false, false
}

func redactSettings(settings model.Settings) model.Settings {
	settings.DiscordWebhook = ""
	settings.NtfyURL = ""
	settings.WebhookURL = ""
	settings.WebhookHeaders = nil
	settings.WebhookBody = ""
	return settings
}

func settingsForRequest(r *http.Request, settings model.Settings) model.Settings {
	if principal, ok := r.Context().Value(tokenContextKey{}).(tokenPrincipal); ok && principal.Role != TokenRoleAdmin {
		return redactSettings(settings)
	}
	return settings
}

type tokenContextKey struct{}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublic(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if raw := bearerToken(r); raw != "" {
			token, ok := s.tokens.validate(raw)
			if !ok {
				writeError(w, http.StatusUnauthorized, "unauthorized", "API token is invalid")
				return
			}
			if !tokenRoleAllows(token.Role, token.Scopes, r.Method, r.URL.Path) {
				if token.Role == TokenRoleViewer {
					writeError(w, http.StatusForbidden, "read_only_token", "this API token is read-only")
				} else {
					writeError(w, http.StatusForbidden, "token_forbidden", "this API token is not allowed to perform this action")
				}
				return
			}
			ctx := context.WithValue(r.Context(), tokenContextKey{}, tokenPrincipal{Name: token.Name, Role: token.Role, ReadOnly: token.ReadOnly, Scopes: append([]string(nil), token.Scopes...)})
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		if _, ok := s.auth.FromRequest(r); !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}

func (s *Server) auditUser(r *http.Request) string {
	if principal, ok := r.Context().Value(tokenContextKey{}).(tokenPrincipal); ok {
		return "token:" + principal.Name
	}
	if sess, ok := s.sessionUser(r); ok {
		return sess.User
	}
	return "anonymous"
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
		if bearerToken(r) != "" {
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
	authorized := false
	header := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if s.cfg.MetricsToken != "" && header != "" && subtle.ConstantTimeCompare([]byte(header), []byte(s.cfg.MetricsToken)) == 1 {
		authorized = true
	}
	if !authorized {
		if _, ok := s.tokens.validate(header); ok {
			authorized = true
		}
	}
	if !authorized {
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
		// MCP actions write their own, tool-level audit entries through the
		// OnAction callback; avoid a duplicate transport-level line.
		if r.URL.Path == "/api/v1/mcp" {
			next.ServeHTTP(w, r)
			return
		}
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		user := s.auditUser(r)
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
