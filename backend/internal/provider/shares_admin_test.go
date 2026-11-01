package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"aurora/internal/model"
)

func writeTestINI(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestShareConfigReadsStateFiles(t *testing.T) {
	state := t.TempDir()
	writeTestINI(t, filepath.Join(state, "shares.ini"), `["media"]
comment="Movies and series"
floor="10G"
allocator="highwater"
splitLevel="2"
include="disk1,disk2"
exclude="disk3"
useCache="yes"
cachePool="cache"
cachePool2=""
cow="auto"
exclusive="no"
`)
	writeTestINI(t, filepath.Join(state, "sec.ini"), `["media"]
export="e"
security="secure"
caseSensitive="auto"
volsizelimit=""
`)
	writeTestINI(t, filepath.Join(state, "sec_nfs.ini"), `["media"]
export="e"
security="private"
hostList="192.168.1.0/24(rw)"
`)

	live := &Live{stateDir: state}
	cfg, err := live.ShareConfig(context.Background(), "media")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if cfg.Comment != "Movies and series" || cfg.Floor != "10G" || cfg.Allocator != "highwater" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if cfg.UseCache != "yes" || cfg.CachePool != "cache" || cfg.SplitLevel != "2" {
		t.Fatalf("cache cfg = %+v", cfg)
	}
	if len(cfg.Include) != 2 || len(cfg.Exclude) != 1 {
		t.Fatalf("disk lists = %+v / %+v", cfg.Include, cfg.Exclude)
	}
	if cfg.SMBExport != "e" || cfg.SMBSecurity != "secure" {
		t.Fatalf("smb = %+v", cfg)
	}
	if cfg.NFSExport != "e" || cfg.NFSSecurity != "private" || cfg.NFSHostList != "192.168.1.0/24(rw)" {
		t.Fatalf("nfs = %+v", cfg)
	}
}

func TestValidateShareConfig(t *testing.T) {
	valid := model.ShareConfig{
		Name: "media", Allocator: "highwater", SplitLevel: "2", UseCache: "yes",
		COW: "auto", CachePool: "cache", Include: []string{"disk1"}, Exclude: []string{"disk2"},
	}
	if err := validateShareConfig(valid, "media"); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	for name, mutate := range map[string]func(*model.ShareConfig){
		"rename":       func(cfg *model.ShareConfig) { cfg.Name = "other" },
		"allocator":    func(cfg *model.ShareConfig) { cfg.Allocator = "random" },
		"split level":  func(cfg *model.ShareConfig) { cfg.SplitLevel = "9" },
		"cache mode":   func(cfg *model.ShareConfig) { cfg.UseCache = "maybe" },
		"floor":        func(cfg *model.ShareConfig) { cfg.Floor = "a lot" },
		"disk name":    func(cfg *model.ShareConfig) { cfg.Include = []string{"disk 1"} },
		"control char": func(cfg *model.ShareConfig) { cfg.Comment = "bad\nvalue" },
	} {
		cfg := valid
		mutate(&cfg)
		if err := validateShareConfig(cfg, "media"); err == nil {
			t.Fatalf("%s should be rejected", name)
		}
	}
}

func TestValidateShareNetworkSettings(t *testing.T) {
	smb := model.ShareConfig{SMBExport: "e", SMBSecurity: "private", SMBCaseSensitive: "auto"}
	if err := validateShareSMB(smb); err != nil {
		t.Fatalf("valid smb rejected: %v", err)
	}
	smb.SMBExport = "invalid"
	if err := validateShareSMB(smb); err == nil {
		t.Fatal("invalid smb export should be rejected")
	}
	nfs := model.ShareConfig{NFSExport: "e", NFSSecurity: "private", NFSHostList: "*(rw)"}
	if err := validateShareNFS(nfs); err != nil {
		t.Fatalf("valid nfs rejected: %v", err)
	}
	nfs.NFSSecurity = "weird"
	if err := validateShareNFS(nfs); err == nil {
		t.Fatal("invalid nfs security should be rejected")
	}
}
