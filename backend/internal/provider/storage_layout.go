package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurora/internal/model"
)

const (
	defaultStorageBays = 12
	maxStorageBays     = 48
)

// StorageLayout is a visual-only map from disk names to chassis bays. It never
// changes Unraid's array configuration.
func (l *Live) StorageLayout(ctx context.Context) (model.StorageLayout, error) {
	layout := model.StorageLayout{Bays: defaultStorageBays, Assignments: map[string]int{}}
	if l.dataDir == "" {
		return layout, nil
	}
	raw, err := os.ReadFile(filepath.Join(l.dataDir, "storage-layout.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return layout, nil
		}
		return layout, fmt.Errorf("read storage layout: %w", err)
	}
	if err := json.Unmarshal(raw, &layout); err != nil {
		return layout, fmt.Errorf("decode storage layout: %w", err)
	}
	if layout.Bays < 4 || layout.Bays > maxStorageBays {
		layout.Bays = defaultStorageBays
	}
	if layout.Assignments == nil {
		layout.Assignments = map[string]int{}
	}
	return layout, nil
}

func (l *Live) SaveStorageLayout(ctx context.Context, layout model.StorageLayout) error {
	if layout.Bays < 4 || layout.Bays > maxStorageBays {
		return fmt.Errorf("bay count must be between 4 and %d", maxStorageBays)
	}
	seen := make(map[int]string, len(layout.Assignments))
	clean := make(map[string]int, len(layout.Assignments))
	for name, bay := range layout.Assignments {
		name = strings.TrimSpace(name)
		if name == "" || len(name) > 128 {
			return fmt.Errorf("invalid disk name")
		}
		if bay < 0 || bay >= layout.Bays {
			return fmt.Errorf("bay %d is outside the chassis", bay)
		}
		if previous, ok := seen[bay]; ok {
			return fmt.Errorf("disk %s and %s share bay %d", previous, name, bay)
		}
		seen[bay] = name
		clean[name] = bay
	}
	layout.Assignments = clean
	layout.Updated = time.Now().UTC()

	if l.dataDir == "" {
		return nil
	}
	if err := os.MkdirAll(l.dataDir, 0o750); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	raw, err := json.MarshalIndent(layout, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(l.dataDir, "storage-layout.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
