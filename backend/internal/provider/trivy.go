package provider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurora/internal/model"
)

// Start launches background tasks that must outlive a single request.
func (l *Live) Start(ctx context.Context) {
	if l.trivyManager == nil {
		return
	}
	go l.trivyManager.RunScheduler(ctx, func() model.Settings {
		settings, _ := l.Settings(ctx)
		return settings
	})
}

func (l *Live) TrivyStatus(ctx context.Context) (model.TrivyStatus, error) {
	settings, _ := l.Settings(ctx)
	if l.trivyManager == nil {
		return model.TrivyStatus{
			Enabled: settings.TrivyEnabled, DBUpdateHour: settings.TrivyDBUpdateHour,
			Error: "Trivy storage is unavailable",
		}, nil
	}
	status := l.trivyManager.Status()
	status.Enabled = settings.TrivyEnabled
	status.DBUpdateHour = settings.TrivyDBUpdateHour
	return status, nil
}

func (l *Live) TrivyVersions(ctx context.Context) ([]model.TrivyVersion, error) {
	if l.trivyManager == nil {
		return nil, errors.New("Trivy storage is unavailable")
	}
	return l.trivyManager.Versions(ctx)
}

func (l *Live) StartTrivyInstall(version string) error {
	if l.trivyManager == nil {
		return errors.New("Trivy storage is unavailable")
	}
	return l.trivyManager.StartInstall(version)
}

func (l *Live) StartTrivyDBUpdate() error {
	if l.trivyManager == nil {
		return errors.New("Trivy storage is unavailable")
	}
	return l.trivyManager.StartDBUpdate()
}

func (l *Live) ContainerSecurity(ctx context.Context, id string) (model.ContainerSecurityDetail, error) {
	settings, _ := l.Settings(ctx)
	if l.trivyManager == nil {
		return model.ContainerSecurityDetail{ContainerSecurity: model.ContainerSecurity{Enabled: false}}, nil
	}
	image, imageID, err := l.containerImageIdentity(ctx, id)
	if err != nil {
		return model.ContainerSecurityDetail{}, err
	}
	detail := l.trivyManager.Detail(image, imageID, settings.TrivyEnabled)
	if settings.TrivyEnabled && !detail.Available && !detail.Scanning {
		l.trivyManager.EnqueueScan(image, imageID, false)
	}
	return detail, nil
}

func (l *Live) ScanContainer(ctx context.Context, id string) error {
	settings, _ := l.Settings(ctx)
	if !settings.TrivyEnabled {
		return errors.New("image scanning is disabled in Settings")
	}
	if l.trivyManager == nil {
		return errors.New("Trivy storage is unavailable")
	}
	image, imageID, err := l.containerImageIdentity(ctx, id)
	if err != nil {
		return err
	}
	if !l.trivyManager.EnqueueScan(image, imageID, true) {
		return errors.New("Trivy is not ready or a scan is already running for this image")
	}
	return nil
}

func (l *Live) ScanAllContainers(ctx context.Context) (int, error) {
	settings, _ := l.Settings(ctx)
	if !settings.TrivyEnabled {
		return 0, errors.New("image scanning is disabled in Settings")
	}
	if l.trivyManager == nil {
		return 0, errors.New("Trivy storage is unavailable")
	}
	status := l.trivyManager.Status()
	if !status.Installed || !status.RuntimeReady {
		return 0, errors.New("Trivy is not ready; wait for the installation to finish")
	}
	if !status.DBReady || !status.JavaDBReady {
		return 0, errors.New("the vulnerability databases are not ready")
	}
	containers, err := l.listContainers(ctx)
	if err != nil {
		return 0, err
	}
	seen := map[string]bool{}
	started, skipped := 0, 0
	for _, container := range containers {
		key := container.ImageID
		if key == "" {
			key = container.Image
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		if l.trivyManager.EnqueueScan(container.Image, container.ImageID, true) {
			started++
		} else {
			skipped++
		}
	}
	if started == 0 {
		if skipped > 0 {
			return 0, errors.New("scans are already running for these images")
		}
		return 0, errors.New("no container image was found to scan")
	}
	return started, nil
}

func (l *Live) enrichContainerSecurity(ctx context.Context, containers []model.Container) {
	if l.trivyManager == nil {
		return
	}
	settings, _ := l.Settings(ctx)
	for index := range containers {
		containers[index].Security = l.trivyManager.Security(
			containers[index].Image, containers[index].ImageID, settings.TrivyEnabled,
		)
	}
}

func (l *Live) queueContainerScan(id string) {
	if l.trivyManager == nil {
		return
	}
	settings, _ := l.Settings(context.Background())
	if !settings.TrivyEnabled {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	image, imageID, err := l.containerImageIdentity(ctx, id)
	if err != nil {
		slog.Warn("inspect container for Trivy scan", "container", id, "error", err)
		return
	}
	l.trivyManager.EnqueueScan(image, imageID, false)
}

func (l *Live) applyTrivySettings(settings model.Settings) {
	if l.trivyManager == nil || !settings.TrivyEnabled {
		return
	}
	go func() {
		status := l.trivyManager.Status()
		desired := strings.TrimPrefix(strings.TrimSpace(settings.TrivyVersion), "v")
		if !status.Installed || !status.RuntimeReady || (desired != "" && desired != status.Version) {
			if err := l.trivyManager.StartInstall(desired); err != nil {
				slog.Warn("install Trivy", "error", err)
			}
			return
		}
		if !status.DBReady || !status.JavaDBReady {
			if err := l.trivyManager.StartDBUpdate(); err != nil {
				slog.Warn("update Trivy database", "error", err)
			}
		}
	}()
}

func (l *Live) containerImageIdentity(ctx context.Context, id string) (string, string, error) {
	inspect, err := l.inspectContainer(ctx, id)
	if err != nil {
		return "", "", fmt.Errorf("inspect container: %w", err)
	}
	image := ""
	if config := childMap(inspect, "Config"); config != nil {
		image = stringValue(config["Image"])
	}
	imageID := stringValue(inspect["Image"])
	if image == "" {
		image = imageID
	}
	if image == "" {
		return "", "", errors.New("container image is unknown")
	}
	return image, imageID, nil
}
