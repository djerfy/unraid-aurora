// Command aurorad serves the Aurora control plane and its embedded web UI.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"aurora/internal/api"
	"aurora/internal/auth"
	"aurora/internal/config"
	"aurora/internal/metrics"
	"aurora/internal/provider"
)

var version = "0.1.0-dev"

func main() {
	var (
		addr    = flag.String("addr", "", "listen address (overrides AURORA_ADDR)")
		showVer = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()
	if *showVer {
		os.Stdout.WriteString("aurora " + version + "\n")
		return
	}

	level := slog.LevelInfo
	if strings.EqualFold(os.Getenv("AURORA_LOG_LEVEL"), "debug") {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	cfg, err := config.Load(version)
	if err != nil {
		log.Error("invalid configuration", "error", err)
		os.Exit(2)
	}
	if *addr != "" {
		cfg.Addr = *addr
	}

	certPath, keyPath, err := resolveTLS(cfg)
	if err != nil {
		log.Error("cannot locate the TLS certificate", "error", err,
			"certDir", cfg.TLSCertDir, "hint", "generate the certificate in Unraid or set AURORA_TLS_CERT/AURORA_TLS_KEY")
		os.Exit(1)
	}
	certificate, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		log.Error("cannot load the TLS certificate", "cert", certPath, "key", keyPath, "error", err,
			"hint", "generate the certificate in Unraid or set AURORA_TLS_CERT/AURORA_TLS_KEY")
		os.Exit(1)
	}

	p, err := provider.NewLive(cfg.StateDir, cfg.BootDir, cfg.DockerSocket, cfg.DataDir)
	if err != nil {
		log.Error("cannot read the Unraid host", "error", err,
			"hint", "Aurora must run on Unraid with access to /var/local/emhttp, /etc/shadow and the Docker socket")
		os.Exit(1)
	}
	defer p.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	hub := metrics.NewHub(p)
	go hub.Run(ctx, 2*time.Second)

	authManager := auth.New(cfg.SessionTTL)
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.New(cfg, p, hub, authManager, log).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// SSE connections are long lived; write deadlines are managed per handler.
		WriteTimeout:   0,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{certificate},
			MinVersion:   tls.VersionTLS12,
		},
	}

	log.Info("aurora is starting",
		"addr", cfg.Addr, "version", version,
		"auth", "native root/shadow", "readOnly", cfg.ReadOnly,
		"stateDir", cfg.StateDir, "tls", certPath,
	)

	go func() {
		if err := srv.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server stopped", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
	}
}

// resolveTLS finds the Unraid certificate bundle. The standard location is
// /boot/config/ssl/certs/<server>_unraid_bundle.pem, where <server> is the
// Unraid server name from var.ini (or the host name as a fallback).
func resolveTLS(cfg config.Config) (certPath, keyPath string, err error) {
	certPath = cfg.TLSCert
	if certPath == "" {
		name := cfg.ServerName
		if name == "" {
			name = serverNameFromState(cfg.StateDir)
		}
		if name == "" {
			name, _ = os.Hostname()
		}
		name = filepath.Base(strings.TrimSpace(name))
		if name == "" || name == "." || name == ".." {
			return "", "", fmt.Errorf("cannot determine the Unraid server name")
		}
		certPath = filepath.Join(cfg.TLSCertDir, name+"_unraid_bundle.pem")
		if _, statErr := os.Stat(certPath); statErr != nil {
			// Old or renamed servers can leave a single bundle behind. Use it
			// only when the choice is unambiguous.
			matches, _ := filepath.Glob(filepath.Join(cfg.TLSCertDir, "*_unraid_bundle.pem"))
			if len(matches) == 1 {
				certPath = matches[0]
			} else {
				return "", "", fmt.Errorf("expected %s (server name %q): %w", certPath, name, statErr)
			}
		}
	}
	keyPath = cfg.TLSKey
	if keyPath == "" {
		keyPath = certPath
	}
	return certPath, keyPath, nil
}

func serverNameFromState(stateDir string) string {
	raw, err := os.ReadFile(filepath.Join(stateDir, "var.ini"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "NAME" {
			continue
		}
		return strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return ""
}
