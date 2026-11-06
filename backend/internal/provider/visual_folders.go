package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"aurora/internal/model"
)

const (
	visualFoldersContainers = "containers"
	visualFoldersVMs        = "vms"
	visualFoldersVersion    = 1
)

func visualFoldersFileName(kind string) (string, error) {
	switch kind {
	case visualFoldersContainers:
		return "container-folders.json", nil
	case visualFoldersVMs:
		return "vm-folders.json", nil
	default:
		return "", errors.New("unknown folder kind")
	}
}

func normalizeVisualFolders(layout model.VisualFolders) (model.VisualFolders, error) {
	folders := make([]model.VisualFolder, 0, len(layout.Folders))
	byID := make(map[string]bool, len(layout.Folders))
	names := make(map[string]bool, len(layout.Folders))
	for _, folder := range layout.Folders {
		id := strings.TrimSpace(folder.ID)
		name := strings.TrimSpace(folder.Name)
		if id == "" || name == "" {
			continue
		}
		if len(id) > 64 || len(name) > 64 {
			return model.VisualFolders{}, errors.New("folder names and IDs must be 64 characters or fewer")
		}
		if byID[id] {
			return model.VisualFolders{}, fmt.Errorf("duplicate folder ID %q", id)
		}
		if names[strings.ToLower(name)] {
			return model.VisualFolders{}, fmt.Errorf("duplicate folder name %q", name)
		}
		byID[id] = true
		names[strings.ToLower(name)] = true
		folders = append(folders, model.VisualFolder{ID: id, Name: name, Order: folder.Order})
		if len(folders) > 200 {
			return model.VisualFolders{}, errors.New("too many folders")
		}
	}
	sort.SliceStable(folders, func(i, j int) bool {
		if folders[i].Order == folders[j].Order {
			return strings.ToLower(folders[i].Name) < strings.ToLower(folders[j].Name)
		}
		return folders[i].Order < folders[j].Order
	})
	for index := range folders {
		folders[index].Order = index
	}
	assignments := make(map[string]string, len(layout.Assignments))
	for name, folderID := range layout.Assignments {
		name = strings.TrimSpace(name)
		folderID = strings.TrimSpace(folderID)
		if name == "" || folderID == "" || len(name) > 256 {
			continue
		}
		if !byID[folderID] {
			continue
		}
		assignments[name] = folderID
	}
	return model.VisualFolders{Version: visualFoldersVersion, Folders: folders, Assignments: assignments}, nil
}

func (l *Live) VisualFolders(_ context.Context, kind string) (model.VisualFolders, error) {
	file, err := visualFoldersFileName(kind)
	if err != nil {
		return model.VisualFolders{}, err
	}
	raw, err := l.readConfigFile(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.VisualFolders{Version: visualFoldersVersion, Folders: []model.VisualFolder{}, Assignments: map[string]string{}}, nil
		}
		return model.VisualFolders{}, err
	}
	var layout model.VisualFolders
	if err := json.Unmarshal(raw, &layout); err != nil {
		return model.VisualFolders{}, fmt.Errorf("parse visual folders: %w", err)
	}
	return normalizeVisualFolders(layout)
}

func (l *Live) SaveVisualFolders(_ context.Context, kind string, layout model.VisualFolders) error {
	file, err := visualFoldersFileName(kind)
	if err != nil {
		return err
	}
	normalized, err := normalizeVisualFolders(layout)
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return err
	}
	path := l.configPath(file)
	if path == "" {
		return errors.New("configuration directory is unavailable")
	}
	return l.writeConfigFile("visual-folders."+kind, "Updated "+kind+" folders", path, raw)
}
