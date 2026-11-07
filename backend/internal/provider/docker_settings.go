package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"aurora/internal/model"
)

func (l *Live) dockerConfigPath() string {
	if l.bootDir == "" {
		return ""
	}
	return filepath.Join(l.bootDir, "config", "docker.cfg")
}

func dockerConfigValue(values map[string]string, key, fallback string) string {
	if value, ok := values[key]; ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func dockerConfigBool(values map[string]string, key string, fallback bool) bool {
	value, ok := values[key]
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.EqualFold(strings.TrimSpace(value), "yes")
}

func dockerConfigInt(values map[string]string, key string, fallback int) int {
	value, ok := values[key]
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return number
}

func (l *Live) DockerSettings(ctx context.Context) (model.DockerSettings, error) {
	values := map[string]string{}
	if path := l.dockerConfigPath(); path != "" {
		if ini, err := parseINI(path); err == nil {
			values = ini.section("")
		} else if !errors.Is(err, os.ErrNotExist) {
			return model.DockerSettings{}, fmt.Errorf("read docker.cfg: %w", err)
		}
	}
	settings := model.DockerSettings{
		Enabled:              dockerConfigBool(values, "DOCKER_ENABLED", false),
		StopTimeout:          dockerConfigInt(values, "DOCKER_TIMEOUT", 10),
		PIDLimit:             dockerConfigInt(values, "DOCKER_PID_LIMIT", 0),
		ImageType:            dockerConfigValue(values, "DOCKER_IMAGE_TYPE", ""),
		ImageSize:            dockerConfigInt(values, "DOCKER_IMAGE_SIZE", 20),
		ImageFile:            dockerConfigValue(values, "DOCKER_IMAGE_FILE", ""),
		AppConfigPath:        dockerConfigValue(values, "DOCKER_APP_CONFIG_PATH", ""),
		AppUnraidPath:        dockerConfigValue(values, "DOCKER_APP_UNRAID_PATH", ""),
		BackingFS:            dockerConfigValue(values, "DOCKER_BACKINGFS", "overlay2"),
		LogRotation:          dockerConfigBool(values, "DOCKER_LOG_ROTATION", true),
		LogSize:              dockerConfigValue(values, "DOCKER_LOG_SIZE", "50m"),
		LogFiles:             dockerConfigInt(values, "DOCKER_LOG_FILES", 1),
		ReadMore:             dockerConfigBool(values, "DOCKER_READMORE", true),
		AuthoringMode:        dockerConfigBool(values, "DOCKER_AUTHORING_MODE", false),
		NetworkType:          dockerConfigValue(values, "DOCKER_NETWORK_TYPE", ""),
		AllowAccess:          dockerConfigBool(values, "DOCKER_ALLOW_ACCESS", false),
		PreserveUserNetworks: strings.EqualFold(dockerConfigValue(values, "DOCKER_USER_NETWORKS", "remove"), "preserve"),
	}
	if info, err := l.dockerInfo(ctx); err == nil {
		settings.Running = true
		settings.Version = info.ServerVersion
		settings.StorageDriver = info.Driver
	}
	return settings, nil
}

type dockerInfo struct {
	ServerVersion string `json:"ServerVersion"`
	Driver        string `json:"Driver"`
}

func (l *Live) dockerInfo(ctx context.Context) (dockerInfo, error) {
	raw, err := l.dockerDo(ctx, http.MethodGet, "/info", nil)
	if err != nil {
		return dockerInfo{}, err
	}
	var info dockerInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return dockerInfo{}, err
	}
	return info, nil
}

func validDockerPath(value string, folder bool) bool {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "/mnt/") || strings.ContainsAny(value, "\x00\n\r\\") {
		return false
	}
	if folder {
		return strings.HasSuffix(value, "/")
	}
	return strings.HasSuffix(value, "docker.img") || strings.HasSuffix(value, "docker-xfs.img")
}

func (l *Live) SaveDockerSettings(ctx context.Context, settings model.DockerSettings) error {
	if settings.StopTimeout < 1 || settings.StopTimeout > 3600 {
		return errors.New("Docker stop timeout must be between 1 and 3600 seconds")
	}
	if settings.PIDLimit < 0 || settings.PIDLimit > 1_000_000 {
		return errors.New("Docker PID limit must be between 0 and 1000000")
	}
	switch settings.ImageType {
	case "", "xfs", "folder":
	default:
		return errors.New("Docker vDisk type must be btrfs, xfs or directory")
	}
	if settings.ImageType != "folder" {
		if settings.ImageSize < 1 || settings.ImageSize > 100_000 {
			return errors.New("Docker vDisk size must be between 1 and 100000 GB")
		}
		if !validDockerPath(settings.ImageFile, false) {
			return errors.New("Docker vDisk location must be an absolute /mnt path ending with docker.img or docker-xfs.img")
		}
	} else {
		if !validDockerPath(settings.ImageFile, true) {
			return errors.New("Docker directory must be an absolute /mnt path ending with /")
		}
	}
	if !validDockerPath(settings.AppConfigPath, true) {
		return errors.New("default appdata path must be an absolute /mnt path ending with /")
	}
	if settings.AppUnraidPath != "" && !validDockerPath(settings.AppUnraidPath, true) {
		return errors.New("/unraid path must be an absolute /mnt path ending with / or empty")
	}
	switch settings.BackingFS {
	case "overlay2", "native":
	default:
		return errors.New("Docker storage driver must be overlay2 or native")
	}
	switch settings.LogSize {
	case "10m", "20m", "50m", "100m", "500m", "1g":
	default:
		return errors.New("Docker log size must be 10m, 20m, 50m, 100m, 500m or 1g")
	}
	if settings.LogFiles < 1 || settings.LogFiles > 3 {
		return errors.New("Docker log file count must be between 1 and 3")
	}
	switch settings.NetworkType {
	case "", "1":
	default:
		return errors.New("Docker network type must be macvlan or ipvlan")
	}

	updates := map[string]string{
		"DOCKER_ENABLED":         yesNo(settings.Enabled),
		"DOCKER_TIMEOUT":         strconv.Itoa(settings.StopTimeout),
		"DOCKER_PID_LIMIT":       intOrEmpty(settings.PIDLimit),
		"DOCKER_IMAGE_TYPE":      settings.ImageType,
		"DOCKER_IMAGE_SIZE":      intOrEmpty(settings.ImageSize),
		"DOCKER_IMAGE_FILE":      settings.ImageFile,
		"DOCKER_APP_CONFIG_PATH": settings.AppConfigPath,
		"DOCKER_APP_UNRAID_PATH": settings.AppUnraidPath,
		"DOCKER_BACKINGFS":       settings.BackingFS,
		"DOCKER_LOG_ROTATION":    yesNo(settings.LogRotation),
		"DOCKER_LOG_SIZE":        settings.LogSize,
		"DOCKER_LOG_FILES":       strconv.Itoa(settings.LogFiles),
		"DOCKER_READMORE":        yesNo(settings.ReadMore),
		"DOCKER_AUTHORING_MODE":  yesNo(settings.AuthoringMode),
		"DOCKER_NETWORK_TYPE":    settings.NetworkType,
		"DOCKER_ALLOW_ACCESS":    yesNo(settings.AllowAccess),
		"DOCKER_USER_NETWORKS":   map[bool]string{true: "preserve", false: "remove"}[settings.PreserveUserNetworks],
	}
	if settings.ImageType == "folder" {
		updates["DOCKER_IMAGE_SIZE"] = ""
	}
	path := l.dockerConfigPath()
	if path == "" {
		return errors.New("boot directory is unavailable")
	}
	raw, err := updateDockerConfig(path, updates)
	if err != nil {
		return err
	}
	return l.writeConfigFile("docker.settings", "Updated Docker settings", path, raw)
}

func dockerYesEmpty(value bool) string {
	if value {
		return "yes"
	}
	return ""
}

func intOrEmpty(value int) string {
	if value <= 0 {
		return ""
	}
	return strconv.Itoa(value)
}

func updateDockerConfig(path string, updates map[string]string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read docker.cfg: %w", err)
	}
	remaining := map[string]string{}
	for key, value := range updates {
		remaining[key] = value
	}
	lines := []string{}
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
			lines = append(lines, line)
			continue
		}
		key, _, ok := strings.Cut(trimmed, "=")
		if !ok {
			lines = append(lines, line)
			continue
		}
		key = strings.TrimSpace(key)
		value, exists := remaining[key]
		if !exists {
			lines = append(lines, line)
			continue
		}
		lines = append(lines, key+"=\""+strings.ReplaceAll(value, "\"", "\\\"")+"\"")
		delete(remaining, key)
	}
	if len(remaining) > 0 {
		keys := make([]string, 0, len(remaining))
		for key := range remaining {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			lines = append(lines, key+"=\""+strings.ReplaceAll(remaining[key], "\"", "\\\"")+"\"")
		}
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}
