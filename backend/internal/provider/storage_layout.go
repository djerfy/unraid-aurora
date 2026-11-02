package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"aurora/internal/model"
)

const (
	storageBoardColumns   = 12
	maxStorageViews       = 12
	maxStorageSections    = 12
	maxStorageSectionRows = 8
	maxStorageSectionCols = 8
)

type legacyStorageView struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Orientation string         `json:"orientation"`
	Column      int            `json:"column"`
	Bays        int            `json:"bays"`
	Assignments map[string]int `json:"assignments"`
}

type legacyStorageLayout struct {
	Columns     int                 `json:"columns"`
	Views       []legacyStorageView `json:"views"`
	Bays        int                 `json:"bays"`
	Assignments map[string]int      `json:"assignments"`
	Updated     time.Time           `json:"updated"`
}

func panelDimensions(kind, size string) (int, int) {
	if kind == "server" {
		switch size {
		case "s":
			return 6, 10
		case "l":
			return 12, 18
		default:
			return 9, 14
		}
	}
	if kind == "nas" {
		switch size {
		case "s":
			return 4, 10
		case "l":
			return 9, 18
		default:
			return 6, 14
		}
	}
	switch size {
	case "s":
		return 4, 14
	case "l":
		return 9, 26
	default:
		return 6, 20
	}
}

func defaultStorageSection(kind string) model.StorageSection {
	rows, cols := 4, 2
	if kind == "server" {
		rows, cols = 2, 4
	} else if kind == "nas" {
		rows, cols = 1, 4
	}
	return model.StorageSection{
		ID: "data", Name: "Data", Color: "#4aa3ff", Media: "any",
		Rows: rows, Cols: cols, Assignments: map[string]int{},
	}
}

func defaultStorageView() model.StorageView {
	w, h := panelDimensions("tower", "m")
	return model.StorageView{
		ID: "view-tower", Name: "Desktop 1", Kind: "tower", Size: "m",
		X: 0, Y: 0, W: w, H: h,
		Sections: []model.StorageSection{defaultStorageSection("tower")},
	}
}

func normalizeSections(sections []model.StorageSection) []model.StorageSection {
	out := make([]model.StorageSection, 0, len(sections))
	for _, section := range sections {
		section.ID = strings.TrimSpace(section.ID)
		section.Name = strings.TrimSpace(section.Name)
		if section.Media != "hdd" && section.Media != "ssd" && section.Media != "nvme" {
			section.Media = "any"
		}
		if section.Rows < 1 || section.Rows > maxStorageSectionRows {
			section.Rows = 1
		}
		if section.Cols < 1 || section.Cols > maxStorageSectionCols {
			section.Cols = 4
		}
		if section.Assignments == nil {
			section.Assignments = map[string]int{}
		}
		out = append(out, section)
	}
	return out
}

func normalizeStorageViews(views []model.StorageView) []model.StorageView {
	out := make([]model.StorageView, 0, len(views))
	for _, view := range views {
		view.ID = strings.TrimSpace(view.ID)
		view.Name = strings.TrimSpace(view.Name)
		if view.Kind != "tower" && view.Kind != "server" && view.Kind != "nas" {
			view.Kind = "tower"
		}
		if view.Size != "s" && view.Size != "m" && view.Size != "l" && view.Size != "custom" {
			view.Size = "m"
		}
		fallbackW, fallbackH := panelDimensions(view.Kind, "m")
		if view.Size == "s" || view.Size == "l" {
			fallbackW, fallbackH = panelDimensions(view.Kind, view.Size)
		}
		if view.W < 1 || view.W > storageBoardColumns {
			view.W = fallbackW
		}
		if view.H < 1 || view.H > 60 {
			view.H = fallbackH
		}
		if view.X < 0 {
			view.X = 0
		}
		if view.Y < 0 {
			view.Y = 0
		}
		if view.X+view.W > storageBoardColumns {
			view.X = storageBoardColumns - view.W
		}
		view.Sections = normalizeSections(view.Sections)
		out = append(out, view)
	}
	return out
}

func migrateLegacyLayout(file legacyStorageLayout) model.StorageLayout {
	layout := model.StorageLayout{Version: 2, Updated: file.Updated, Views: []model.StorageView{}}
	legacyViews := file.Views
	if len(legacyViews) == 0 {
		legacyViews = []legacyStorageView{{
			ID: "view-main", Name: "Main case", Orientation: "horizontal",
			Bays: file.Bays, Assignments: file.Assignments,
		}}
	}
	y := 0
	for index, legacy := range legacyViews {
		kind := "tower"
		switch legacy.Orientation {
		case "horizontal":
			kind = "server"
		case "nas":
			kind = "nas"
		}
		if legacy.ID == "" {
			legacy.ID = fmt.Sprintf("view-%d", index+1)
		}
		if legacy.Name == "" {
			legacy.Name = strings.ToUpper(kind[:1]) + kind[1:] + " " + fmt.Sprint(index+1)
		}
		bays := legacy.Bays
		if bays < 1 {
			bays = 12
		}
		cols := 4
		if kind == "tower" {
			cols = 2
		}
		rows := int(math.Ceil(float64(bays) / float64(cols)))
		if rows < 1 {
			rows = 1
		}
		assignments := legacy.Assignments
		if assignments == nil {
			assignments = map[string]int{}
		}
		w, h := panelDimensions(kind, "m")
		layout.Views = append(layout.Views, model.StorageView{
			ID: legacy.ID, Name: legacy.Name, Kind: kind, Size: "m",
			X: 0, Y: y, W: w, H: h,
			Sections: []model.StorageSection{{
				ID: "data", Name: "Data", Color: "#4aa3ff", Media: "any",
				Rows: min(rows, maxStorageSectionRows), Cols: cols, Assignments: assignments,
			}},
		})
		y += h + 2
	}
	return layout
}

// StorageLayout is a visual-only map from disk names to chassis bays. It never
// changes Unraid's array configuration.
func (l *Live) StorageLayout(ctx context.Context) (model.StorageLayout, error) {
	layout := model.StorageLayout{Version: 2, Views: []model.StorageView{defaultStorageView()}}
	if l.configDir() == "" {
		return layout, nil
	}
	raw, err := l.readConfigFile("storage-layout.json")
	if err != nil {
		if os.IsNotExist(err) {
			return layout, nil
		}
		return layout, fmt.Errorf("read storage layout: %w", err)
	}
	var header struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return layout, fmt.Errorf("decode storage layout: %w", err)
	}
	if header.Version < 2 {
		var legacy legacyStorageLayout
		if err := json.Unmarshal(raw, &legacy); err != nil {
			return layout, fmt.Errorf("decode legacy storage layout: %w", err)
		}
		return migrateLegacyLayout(legacy), nil
	}
	if err := json.Unmarshal(raw, &layout); err != nil {
		return layout, fmt.Errorf("decode storage layout: %w", err)
	}
	layout.Version = 2
	layout.Views = normalizeStorageViews(layout.Views)
	if len(layout.Views) == 0 {
		layout.Views = []model.StorageView{defaultStorageView()}
	}
	return layout, nil
}

func validateStorageLayout(layout model.StorageLayout) ([]model.StorageView, error) {
	if len(layout.Views) < 1 || len(layout.Views) > maxStorageViews {
		return nil, fmt.Errorf("views must be between 1 and %d", maxStorageViews)
	}
	seenViews := map[string]bool{}
	seenDisks := map[string]bool{}
	views := make([]model.StorageView, 0, len(layout.Views))
	for _, view := range layout.Views {
		view.ID = strings.TrimSpace(view.ID)
		view.Name = strings.TrimSpace(view.Name)
		if view.ID == "" || len(view.ID) > 64 || seenViews[view.ID] {
			return nil, fmt.Errorf("each view needs a unique id")
		}
		seenViews[view.ID] = true
		if view.Name == "" || len(view.Name) > 64 {
			return nil, fmt.Errorf("each view needs a name")
		}
		if view.Kind != "tower" && view.Kind != "server" && view.Kind != "nas" {
			return nil, fmt.Errorf("view kind must be tower, server or nas")
		}
		if view.Size != "s" && view.Size != "m" && view.Size != "l" && view.Size != "custom" {
			return nil, fmt.Errorf("view size must be s, m, l or custom")
		}
		if view.W < 1 || view.H < 1 || view.W > storageBoardColumns || view.H > 60 {
			return nil, fmt.Errorf("invalid panel size")
		}
		if view.X < 0 || view.Y < 0 || view.X+view.W > storageBoardColumns {
			return nil, fmt.Errorf("panel is outside the board")
		}
		if len(view.Sections) < 1 || len(view.Sections) > maxStorageSections {
			return nil, fmt.Errorf("each view needs between 1 and %d sections", maxStorageSections)
		}
		seenSections := map[string]bool{}
		for _, section := range view.Sections {
			section.ID = strings.TrimSpace(section.ID)
			section.Name = strings.TrimSpace(section.Name)
			if section.ID == "" || seenSections[section.ID] {
				return nil, fmt.Errorf("each section needs a unique id")
			}
			seenSections[section.ID] = true
			if section.Name == "" || len(section.Name) > 64 {
				return nil, fmt.Errorf("each section needs a name")
			}
			if section.Media != "any" && section.Media != "hdd" && section.Media != "ssd" && section.Media != "nvme" {
				return nil, fmt.Errorf("invalid section media")
			}
			if section.Rows < 1 || section.Rows > maxStorageSectionRows || section.Cols < 1 || section.Cols > maxStorageSectionCols {
				return nil, fmt.Errorf("invalid section grid")
			}
			usedSlots := map[int]string{}
			clean := map[string]int{}
			for name, slot := range section.Assignments {
				name = strings.TrimSpace(name)
				if name == "" || len(name) > 128 {
					return nil, fmt.Errorf("invalid disk name")
				}
				if seenDisks[name] {
					return nil, fmt.Errorf("disk %s is assigned more than once", name)
				}
				if slot < 0 || slot >= section.Rows*section.Cols {
					return nil, fmt.Errorf("slot %d is outside section %s", slot, section.Name)
				}
				if previous, ok := usedSlots[slot]; ok {
					return nil, fmt.Errorf("disk %s and %s share slot %d", previous, name, slot)
				}
				usedSlots[slot] = name
				seenDisks[name] = true
				clean[name] = slot
			}
			section.Assignments = clean
		}
		views = append(views, view)
	}
	for i := range views {
		for j := i + 1; j < len(views); j++ {
			if views[i].X < views[j].X+views[j].W &&
				views[j].X < views[i].X+views[i].W &&
				views[i].Y < views[j].Y+views[j].H &&
				views[j].Y < views[i].Y+views[i].H {
				return nil, fmt.Errorf("panels %s and %s overlap", views[i].Name, views[j].Name)
			}
		}
	}
	return views, nil
}

func (l *Live) SaveStorageLayout(ctx context.Context, layout model.StorageLayout) error {
	views, err := validateStorageLayout(layout)
	if err != nil {
		return err
	}
	layout.Version = 2
	layout.Views = views
	layout.Updated = time.Now().UTC()
	if l.configDir() == "" {
		return nil
	}
	raw, err := json.MarshalIndent(layout, "", "  ")
	if err != nil {
		return err
	}
	path := l.configPath("storage-layout.json")
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
