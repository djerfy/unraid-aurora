package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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
		Enabled:              configBool(values, "DOCKER_ENABLED", false),
		StopTimeout:          configInt(values, "DOCKER_TIMEOUT", 10),
		PIDLimit:             configInt(values, "DOCKER_PID_LIMIT", 0),
		ImageType:            configValue(values, "DOCKER_IMAGE_TYPE", ""),
		ImageSize:            configInt(values, "DOCKER_IMAGE_SIZE", 20),
		ImageFile:            configValue(values, "DOCKER_IMAGE_FILE", ""),
		AppConfigPath:        configValue(values, "DOCKER_APP_CONFIG_PATH", ""),
		AppUnraidPath:        configValue(values, "DOCKER_APP_UNRAID_PATH", ""),
		BackingFS:            configValue(values, "DOCKER_BACKINGFS", "overlay2"),
		LogRotation:          configBool(values, "DOCKER_LOG_ROTATION", true),
		LogSize:              configValue(values, "DOCKER_LOG_SIZE", "50m"),
		LogFiles:             configInt(values, "DOCKER_LOG_FILES", 1),
		ReadMore:             configBool(values, "DOCKER_READMORE", true),
		AuthoringMode:        configBool(values, "DOCKER_AUTHORING_MODE", false),
		NetworkType:          configValue(values, "DOCKER_NETWORK_TYPE", ""),
		AllowAccess:          configBool(values, "DOCKER_ALLOW_ACCESS", false),
		PreserveUserNetworks: strings.EqualFold(configValue(values, "DOCKER_USER_NETWORKS", "remove"), "preserve"),
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
	raw, err := updateConfigValues(path, updates)
	if err != nil {
		return err
	}
	return l.writeConfigFile("docker.settings", "Updated Docker settings", path, raw)
}
