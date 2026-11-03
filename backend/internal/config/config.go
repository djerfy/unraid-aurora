package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	Addr         string
	StateDir     string
	BootDir      string
	DockerSocket string
	DataDir      string
	ConfigDir    string
	TLSCert      string // explicit override; empty derives from the server name
	TLSCertDir   string
	TLSKey       string
	ServerName   string // optional override for certificate lookup
	SessionTTL   time.Duration
	Version      string
	MetricsToken string
	ReadOnly     bool
}

func Load(version string) (Config, error) {
	cfg := Config{
		Addr:         env("AURORA_ADDR", ":8484"),
		StateDir:     env("AURORA_STATE_DIR", "/var/local/emhttp"),
		BootDir:      env("AURORA_BOOT_DIR", "/boot"),
		DockerSocket: env("AURORA_DOCKER_SOCKET", "/var/run/docker.sock"),
		DataDir:      env("AURORA_DATA_DIR", "/var/lib/aurora"),
		ConfigDir:    env("AURORA_CONFIG_DIR", ""),
		TLSCert:      env("AURORA_TLS_CERT", ""),
		TLSCertDir:   env("AURORA_TLS_CERT_DIR", "/boot/config/ssl/certs"),
		ServerName:   env("AURORA_SERVER_NAME", ""),
		SessionTTL:   24 * time.Hour,
		Version:      version,
		MetricsToken: env("AURORA_METRICS_TOKEN", ""),
		ReadOnly:     envBool("AURORA_READ_ONLY", false),
	}

	if ttl := env("AURORA_SESSION_TTL", ""); ttl != "" {
		d, err := time.ParseDuration(ttl)
		if err != nil {
			return cfg, fmt.Errorf("invalid AURORA_SESSION_TTL: %w", err)
		}
		if d <= 0 {
			return cfg, fmt.Errorf("AURORA_SESSION_TTL must be positive")
		}
		cfg.SessionTTL = d
	}
	if cfg.StateDir == "" {
		return cfg, fmt.Errorf("AURORA_STATE_DIR must not be empty")
	}
	if cfg.ConfigDir == "" {
		cfg.ConfigDir = filepath.Join(cfg.BootDir, "config", "aurora")
	}
	if cfg.ConfigDir == "" {
		return cfg, fmt.Errorf("AURORA_CONFIG_DIR must not be empty")
	}
	if cfg.TLSCertDir == "" {
		return cfg, fmt.Errorf("AURORA_TLS_CERT_DIR must not be empty")
	}
	cfg.TLSKey = env("AURORA_TLS_KEY", "")
	return cfg, nil
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
