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
	maxStorageViews    = 8
	maxStorageColumns  = 3
)

// storageLayoutFile also decodes the single-view format used before multiple
// chassis views existed.
type storageLayoutFile struct {
	Columns     int                 `json:"columns"`
	Views       []model.StorageView `json:"views"`
	Bays        int                 `json:"bays"`
	Assignments map[string]int      `json:"assignments"`
	Updated     time.Time           `json:"updated"`
}

func defaultStorageView() model.StorageView {
	return model.StorageView{
		ID:          "view-tower",
		Name:        "Tower 1",
		Orientation: "vertical",
		Column:      0,
		Bays:        defaultStorageBays,
		Assignments: map[string]int{},
	}
}

func normalizeStorageViews(views []model.StorageView) []model.StorageView {
	out := make([]model.StorageView, 0, len(views))
	for _, view := range views {
		view.ID = strings.TrimSpace(view.ID)
		view.Name = strings.TrimSpace(view.Name)
		view.Orientation = strings.ToLower(strings.TrimSpace(view.Orientation))
		if view.Orientation != "horizontal" && view.Orientation != "vertical" {
			view.Orientation = "vertical"
		}
		maxBays := maxStorageBays
		if view.Orientation == "nas" {
			maxBays = 8
		}
		if view.Bays < 1 || view.Bays > maxBays {
			if view.Orientation == "nas" {
				view.Bays = 4
			} else {
				view.Bays = defaultStorageBays
			}
		}
		if view.Assignments == nil {
			view.Assignments = map[string]int{}
		}
		out = append(out, view)
	}
	return out
}

// StorageLayout is a visual-only map from disk names to chassis bays. It never
// changes Unraid's array configuration.
func (l *Live) StorageLayout(ctx context.Context) (model.StorageLayout, error) {
	layout := model.StorageLayout{Columns: 2, Views: []model.StorageView{defaultStorageView()}}
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
	var file storageLayoutFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return layout, fmt.Errorf("decode storage layout: %w", err)
	}
	layout.Updated = file.Updated
	layout.Columns = file.Columns
	if layout.Columns < 1 || layout.Columns > maxStorageColumns {
		layout.Columns = 2
	}
	legacyMultiView := file.Columns == 0 && len(file.Views) > 1
	layout.Views = normalizeStorageViews(file.Views)
	if len(layout.Views) == 0 {
		if file.Bays > 0 {
			// Migrate the original single-view layout.
			layout.Views = []model.StorageView{{
				ID:          "view-main",
				Name:        "Main case",
				Orientation: "horizontal",
				Column:      0,
				Bays:        file.Bays,
				Assignments: file.Assignments,
			}}
			if layout.Views[0].Assignments == nil {
				layout.Views[0].Assignments = map[string]int{}
			}
		} else {
			layout.Views = []model.StorageView{defaultStorageView()}
		}
	}
	for index := range layout.Views {
		if legacyMultiView {
			layout.Views[index].Column = index % layout.Columns
		} else if layout.Views[index].Column < 0 || layout.Views[index].Column >= layout.Columns {
			layout.Views[index].Column = 0
		}
	}
	return layout, nil
}

func (l *Live) SaveStorageLayout(ctx context.Context, layout model.StorageLayout) error {
	if len(layout.Views) < 1 || len(layout.Views) > maxStorageViews {
		return fmt.Errorf("views must be between 1 and %d", maxStorageViews)
	}
	if layout.Columns < 1 || layout.Columns > maxStorageColumns {
		return fmt.Errorf("columns must be between 1 and %d", maxStorageColumns)
	}
	seenIDs := map[string]bool{}
	seenDisks := map[string]bool{}
	views := make([]model.StorageView, 0, len(layout.Views))
	for _, view := range layout.Views {
		view.ID = strings.TrimSpace(view.ID)
		view.Name = strings.TrimSpace(view.Name)
		if view.ID == "" || len(view.ID) > 64 || seenIDs[view.ID] {
			return fmt.Errorf("each view needs a unique id")
		}
		seenIDs[view.ID] = true
		if view.Name == "" || len(view.Name) > 64 {
			return fmt.Errorf("each view needs a name")
		}
		if view.Column < 0 || view.Column >= layout.Columns {
			return fmt.Errorf("column %d is outside the board", view.Column)
		}
		if view.Orientation != "vertical" && view.Orientation != "horizontal" && view.Orientation != "nas" {
			return fmt.Errorf("orientation must be vertical, horizontal or nas")
		}
		maxBays := maxStorageBays
		if view.Orientation == "nas" {
			maxBays = 8
		}
		if view.Bays < 1 || view.Bays > maxBays {
			return fmt.Errorf("bay count must be between 1 and %d for this view", maxBays)
		}
		usedBays := make(map[int]string, len(view.Assignments))
		clean := make(map[string]int, len(view.Assignments))
		for name, bay := range view.Assignments {
			name = strings.TrimSpace(name)
			if name == "" || len(name) > 128 {
				return fmt.Errorf("invalid disk name")
			}
			if seenDisks[name] {
				return fmt.Errorf("disk %s is assigned to more than one view", name)
			}
			if bay < 0 || bay >= view.Bays {
				return fmt.Errorf("bay %d is outside view %s", bay, view.Name)
			}
			if previous, ok := usedBays[bay]; ok {
				return fmt.Errorf("disk %s and %s share bay %d", previous, name, bay)
			}
			usedBays[bay] = name
			seenDisks[name] = true
			clean[name] = bay
		}
		view.Assignments = clean
		views = append(views, view)
	}
	layout.Views = views
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
