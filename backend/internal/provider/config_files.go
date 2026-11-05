package provider

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func (l *Live) configDir() string {
	if l.configRoot != "" {
		return l.configRoot
	}
	if l.bootDir != "" {
		return filepath.Join(l.bootDir, "config", "aurora")
	}
	return l.dataDir
}

func (l *Live) configPath(name string) string {
	dir := l.configDir()
	name = strings.TrimSpace(name)
	if dir == "" || name == "" || filepath.Base(name) != name {
		return ""
	}
	return filepath.Join(dir, name)
}

func (l *Live) readConfigFile(name string) ([]byte, error) {
	path := l.configPath(name)
	raw, err := os.ReadFile(path)
	if err == nil {
		return raw, nil
	}
	if l.dataDir == "" {
		return nil, err
	}
	legacy := filepath.Join(l.dataDir, name)
	if legacy == path {
		return nil, err
	}
	raw, err = os.ReadFile(legacy)
	if err != nil {
		return nil, err
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o750)
	// #nosec G703 -- configPath rejects anything that is not a base name.
	_ = os.WriteFile(path, raw, 0o600)
	return raw, nil
}

func (l *Live) saveConfigFile(name string, raw []byte) error {
	path := l.configPath(name)
	if path == "" {
		return os.ErrInvalid
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	// #nosec G703 -- configPath rejects anything that is not a base name.
	return os.WriteFile(path, raw, 0o600)
}

// migrateDurableState moves the small configuration/history files from the
// old tmpfs data directory to the persistent flash config directory.
func (l *Live) migrateDurableState() {
	if l.bootDir == "" || l.dataDir == "" || l.configDir() == l.dataDir {
		return
	}
	_ = os.MkdirAll(l.configDir(), 0o750)
	for _, name := range []string{
		"settings.json",
		"storage-layout.json",
		"mover-history.json",
		"balance-history.json",
	} {
		target := filepath.Join(l.configDir(), name)
		if _, err := os.Stat(target); err == nil {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(l.dataDir, name))
		if err != nil {
			continue
		}
		// #nosec G703 -- target is under the root-owned Aurora config directory.
		_ = os.WriteFile(target, raw, 0o600)
	}
	copyDirIfMissing(filepath.Join(l.dataDir, configHistoryDir), filepath.Join(l.configDir(), configHistoryDir))
}

func copyDirIfMissing(source, target string) {
	if _, err := os.Stat(target); err == nil {
		return
	}
	if _, err := os.Stat(source); err != nil {
		return
	}
	_ = filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return nil
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			_ = os.MkdirAll(destination, 0o750)
			return nil
		}
		// #nosec G122 -- source is the root-owned Aurora config directory.
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		_ = os.MkdirAll(filepath.Dir(destination), 0o750)
		// #nosec G703 G122 -- destination is under the root-owned config directory.
		_ = os.WriteFile(destination, raw, 0o600)
		return nil
	})
}
