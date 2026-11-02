package config

import (
	"path/filepath"
	"testing"
)

func TestConfigDirDefaultsToBootFlash(t *testing.T) {
	boot := t.TempDir()
	t.Setenv("AURORA_BOOT_DIR", boot)
	t.Setenv("AURORA_CONFIG_DIR", "")
	cfg, err := Load("test")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(boot, "config", "aurora")
	if cfg.ConfigDir != want {
		t.Fatalf("ConfigDir = %q, want %q", cfg.ConfigDir, want)
	}
}

func TestConfigDirOverride(t *testing.T) {
	custom := t.TempDir()
	t.Setenv("AURORA_CONFIG_DIR", custom)
	cfg, err := Load("test")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConfigDir != custom {
		t.Fatalf("ConfigDir = %q, want %q", cfg.ConfigDir, custom)
	}
}
