package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/pmezard/go-difflib/difflib"

	"aurora/internal/model"
)

const (
	configHistoryDir   = "config-history"
	configHistoryLimit = 200
	configHistoryMax   = 4 << 20
)

var historyIDPattern = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}-[0-9]{3}-[a-f0-9]{6}$`)

type configChange struct {
	Path   string
	Data   []byte
	Delete bool
}

type historyMeta struct {
	ID          string        `json:"id"`
	Created     time.Time     `json:"created"`
	Action      string        `json:"action"`
	Description string        `json:"description"`
	Files       []historyFile `json:"files"`
}

type historyFile struct {
	Path    string `json:"path"`
	Existed bool   `json:"existed"`
	Backup  string `json:"backup"`
	Diff    string `json:"diff"`
	Size    int64  `json:"size"`
	Mode    uint32 `json:"mode"`
}

// writeConfigFile snapshots the current file then writes the new content,
// recording a restorable history entry.
func (l *Live) writeConfigFile(action, description, path string, data []byte) error {
	return l.writeConfigChanges(action, description, []configChange{{Path: path, Data: data}})
}

func (l *Live) deleteConfigFile(action, description, path string) error {
	return l.writeConfigChanges(action, description, []configChange{{Path: path, Delete: true}})
}

func (l *Live) writeConfigChanges(action, description string, changes []configChange) error {
	if len(changes) == 0 {
		return nil
	}
	if l.dataDir == "" {
		// Tests and configurations without a data directory: keep the write
		// but skip history persistence.
		for _, change := range changes {
			if err := applyConfigChange(change); err != nil {
				return err
			}
		}
		return nil
	}

	entryID := newHistoryID()
	entryDir := filepath.Join(l.dataDir, configHistoryDir, entryID)
	filesDir := filepath.Join(entryDir, "files")
	if err := os.MkdirAll(filesDir, 0o750); err != nil {
		return fmt.Errorf("create history directory: %w", err)
	}
	meta := historyMeta{
		ID: entryID, Created: time.Now().UTC(), Action: action, Description: description,
	}
	var firstErr error
	for index, change := range changes {
		current, err := os.ReadFile(change.Path)
		existed := err == nil
		if err != nil && !os.IsNotExist(err) {
			firstErr = fmt.Errorf("read %s: %w", change.Path, err)
			break
		}
		mode := uint32(0o644)
		if existed {
			if info, statErr := os.Stat(change.Path); statErr == nil {
				mode = uint32(info.Mode().Perm())
			}
			backupName := fmt.Sprintf("%03d.bak", index)
			if err := os.WriteFile(filepath.Join(filesDir, backupName), current, 0o600); err != nil {
				firstErr = fmt.Errorf("write history backup: %w", err)
				break
			}
			meta.Files = append(meta.Files, historyFile{
				Path: change.Path, Existed: true, Backup: backupName,
				Diff: renderConfigDiff(change.Path, current, change.Data, change.Delete),
				Size: int64(len(current)), Mode: mode,
			})
		} else {
			meta.Files = append(meta.Files, historyFile{
				Path: change.Path, Existed: false,
				Diff: renderConfigDiff(change.Path, nil, change.Data, change.Delete),
				Mode: mode,
			})
		}
		if err := applyConfigChange(change); err != nil {
			firstErr = err
			break
		}
	}
	if raw, err := json.MarshalIndent(meta, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(entryDir, "meta.json"), raw, 0o600)
	}
	l.pruneConfigHistory()
	return firstErr
}

func applyConfigChange(change configChange) error {
	if change.Delete {
		if err := os.Remove(change.Path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", change.Path, err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(change.Path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", change.Path, err)
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(change.Path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(change.Path, change.Data, mode); err != nil {
		return fmt.Errorf("write %s: %w", change.Path, err)
	}
	return nil
}

func newHistoryID() string {
	now := time.Now().UTC()
	suffix := make([]byte, 3)
	if _, err := rand.Read(suffix); err != nil {
		return fmt.Sprintf("%s-%03d-000000", now.Format("20060102-150405"), now.Nanosecond()/1e6)
	}
	return fmt.Sprintf("%s-%03d-%s", now.Format("20060102-150405"), now.Nanosecond()/1e6, hex.EncodeToString(suffix))
}

func renderConfigDiff(path string, old, current []byte, deleted bool) string {
	if !deleted && string(old) == string(current) {
		return ""
	}
	if !isTextContent(old) || !isTextContent(current) {
		return "binary file changed\n"
	}
	fromFile := path
	toFile := path
	if len(old) == 0 && !deleted {
		fromFile = "/dev/null"
	}
	if deleted {
		toFile = "/dev/null"
	}
	diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(old)),
		B:        difflib.SplitLines(string(current)),
		FromFile: fromFile,
		ToFile:   toFile,
		Context:  3,
	})
	if err != nil {
		return ""
	}
	return diff
}

func isTextContent(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	if len(data) > configHistoryMax {
		return false
	}
	if !utf8.Valid(data) {
		return false
	}
	for _, value := range data {
		if value == 0 {
			return false
		}
	}
	return true
}

func (l *Live) ConfigHistory(_ context.Context) ([]model.ConfigHistory, error) {
	directory := filepath.Join(l.dataDir, configHistoryDir)
	entries, err := os.ReadDir(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.ConfigHistory{}, nil
		}
		return nil, err
	}
	history := make([]model.ConfigHistory, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		meta, _, err := l.loadHistoryMeta(entry.Name())
		if err != nil {
			continue
		}
		item := historyToModel(meta)
		for index := range item.Files {
			item.Files[index].Diff = ""
		}
		history = append(history, item)
	}
	sort.Slice(history, func(i, j int) bool { return history[i].Created.After(history[j].Created) })
	return history, nil
}

func (l *Live) ConfigHistoryDetail(_ context.Context, id string) (model.ConfigHistory, error) {
	meta, _, err := l.loadHistoryMeta(id)
	if err != nil {
		return model.ConfigHistory{}, err
	}
	return historyToModel(meta), nil
}

// RollbackConfig restores every file from a history entry. The current state is
// snapshotted first so the rollback itself can be reverted from the same page.
func (l *Live) RollbackConfig(_ context.Context, id string) error {
	meta, entryDir, err := l.loadHistoryMeta(id)
	if err != nil {
		return err
	}
	current := make([]configChange, 0, len(meta.Files))
	for _, file := range meta.Files {
		data, readErr := os.ReadFile(file.Path)
		if readErr != nil {
			if !os.IsNotExist(readErr) {
				return fmt.Errorf("read current %s: %w", file.Path, readErr)
			}
			current = append(current, configChange{Path: file.Path, Delete: true})
			continue
		}
		current = append(current, configChange{Path: file.Path, Data: data})
	}
	if err := l.writeConfigChanges("rollback", "Before rollback of "+id, current); err != nil {
		return fmt.Errorf("snapshot before rollback: %w", err)
	}

	for _, file := range meta.Files {
		if !file.Existed {
			if err := os.Remove(file.Path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove %s: %w", file.Path, err)
			}
			continue
		}
		backup, err := os.ReadFile(filepath.Join(entryDir, "files", file.Backup))
		if err != nil {
			return fmt.Errorf("read backup for %s: %w", file.Path, err)
		}
		mode := os.FileMode(file.Mode)
		if mode == 0 {
			mode = 0o644
		}
		if err := os.MkdirAll(filepath.Dir(file.Path), 0o755); err != nil {
			return fmt.Errorf("create directory for %s: %w", file.Path, err)
		}
		if err := os.WriteFile(file.Path, backup, mode); err != nil {
			return fmt.Errorf("restore %s: %w", file.Path, err)
		}
	}
	return nil
}

func (l *Live) loadHistoryMeta(id string) (historyMeta, string, error) {
	if !historyIDPattern.MatchString(id) {
		return historyMeta{}, "", errors.New("invalid history id")
	}
	entryDir := filepath.Join(l.dataDir, configHistoryDir, id)
	raw, err := os.ReadFile(filepath.Join(entryDir, "meta.json"))
	if err != nil {
		return historyMeta{}, "", err
	}
	var meta historyMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return historyMeta{}, "", err
	}
	return meta, entryDir, nil
}

func historyToModel(meta historyMeta) model.ConfigHistory {
	files := make([]model.ConfigFileChange, 0, len(meta.Files))
	for _, file := range meta.Files {
		files = append(files, model.ConfigFileChange{
			Path: file.Path, Existed: file.Existed, Size: file.Size, Diff: file.Diff,
		})
	}
	return model.ConfigHistory{
		ID: meta.ID, Created: meta.Created, Action: meta.Action,
		Description: meta.Description, Files: files,
	}
}

func (l *Live) pruneConfigHistory() {
	if l.dataDir == "" {
		return
	}
	directory := filepath.Join(l.dataDir, configHistoryDir)
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	dirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && historyIDPattern.MatchString(entry.Name()) {
			dirs = append(dirs, entry.Name())
		}
	}
	if len(dirs) <= configHistoryLimit {
		return
	}
	sort.Strings(dirs)
	for _, name := range dirs[:len(dirs)-configHistoryLimit] {
		_ = os.RemoveAll(filepath.Join(directory, name))
	}
}
