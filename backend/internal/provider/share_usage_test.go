package provider

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShareUsageCacheAndCapacity(t *testing.T) {
	state := t.TempDir()
	config := t.TempDir()
	if err := os.WriteFile(filepath.Join(state, "disks.ini"), []byte(`["disk1"]
type="Data"
fsSize="1000000"
fsUsed="400000"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "shares.ini"), []byte(`["media"]
size="0"
free="0"
useCache="no"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]shareUsageEntry{
		"media": {Used: 12345, Updated: time.Now()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "share-usage.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	live := &Live{stateDir: state, configRoot: config, shareUsage: map[string]shareUsageEntry{}, shareUsageRefreshing: map[string]bool{}, shareUsageSem: make(chan struct{}, 1)}
	live.loadShareUsage()
	shares, err := live.Shares(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(shares) != 1 {
		t.Fatalf("shares = %+v", shares)
	}
	if shares[0].Used != 12345 || shares[0].Size != 1000000*1024 {
		t.Fatalf("unexpected share usage: %+v", shares[0])
	}
	if shares[0].Free != shares[0].Size-shares[0].Used {
		t.Fatalf("unexpected free space: %+v", shares[0])
	}
}

func TestStatfsUsage(t *testing.T) {
	size, free, ok := statfsUsage(t.TempDir())
	if !ok || size == 0 || free > size {
		t.Fatalf("unexpected statfs result: %d %d %v", size, free, ok)
	}
}

func TestShareUsageTTLAndManualValidation(t *testing.T) {
	if shareUsageTTL < 6*time.Hour {
		t.Fatalf("share usage TTL = %v, want at least 6h", shareUsageTTL)
	}
	state := t.TempDir()
	if err := os.WriteFile(filepath.Join(state, "shares.ini"), []byte(`["media"]
size="0"
free="0"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	live := &Live{
		stateDir:             state,
		shareUsage:           map[string]shareUsageEntry{},
		shareUsageRefreshing: map[string]bool{},
		shareUsageSem:        make(chan struct{}, 1),
	}
	if _, err := live.RefreshShareUsage(context.Background(), "missing"); err == nil {
		t.Fatal("unknown share should be rejected")
	}
}

func TestSharePathsUsesMergedUserView(t *testing.T) {
	root := t.TempDir()
	previous := shareUserRoot
	shareUserRoot = root
	defer func() { shareUserRoot = previous }()
	if err := os.MkdirAll(filepath.Join(root, "media"), 0o755); err != nil {
		t.Fatal(err)
	}
	live := &Live{}
	paths := live.sharePaths("media")
	if len(paths) != 1 || paths[0] != filepath.Join(root, "media") {
		t.Fatalf("unexpected share path: %+v", paths)
	}
	if paths := live.sharePaths("../etc"); len(paths) != 0 {
		t.Fatalf("invalid share name produced a path: %+v", paths)
	}
}
