// Command aurorad serves the Aurora control plane and its embedded web UI.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
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

	certificate, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
	if err != nil {
		log.Error("cannot load the TLS certificate", "cert", cfg.TLSCert, "key", cfg.TLSKey, "error", err,
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
		"stateDir", cfg.StateDir, "tls", cfg.TLSCert,
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
