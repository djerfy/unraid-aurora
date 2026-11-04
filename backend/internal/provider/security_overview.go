package provider

import (
	"context"
	"errors"
	"sort"
	"strings"

	"aurora/internal/model"
)

// SecurityOverview merges the scans of every container image. Containers that
// share an image are folded into one entry so findings are counted once.
func (l *Live) SecurityOverview(ctx context.Context) (model.SecurityOverview, error) {
	settings, _ := l.Settings(ctx)
	overview := model.SecurityOverview{Enabled: settings.TrivyEnabled, Images: []model.SecurityImage{}}
	if l.trivyManager == nil {
		return overview, errors.New("Trivy storage is unavailable")
	}
	containers, err := l.listContainers(ctx)
	if err != nil {
		return overview, err
	}
	details := make(map[string]model.ContainerSecurityDetail, len(containers))
	byImage := make(map[string]model.ContainerSecurityDetail)
	for _, container := range containers {
		key := strings.TrimSpace(container.ImageID)
		if key == "" {
			key = strings.TrimSpace(container.Image)
		}
		if key == "" {
			continue
		}
		detail, ok := byImage[key]
		if !ok {
			detail = l.trivyManager.Detail(container.Image, container.ImageID, settings.TrivyEnabled)
			byImage[key] = detail
		}
		details[container.ID] = detail
	}
	return buildSecurityOverview(settings.TrivyEnabled, containers, details), nil
}

func buildSecurityOverview(enabled bool, containers []model.Container, details map[string]model.ContainerSecurityDetail) model.SecurityOverview {
	overview := model.SecurityOverview{Enabled: enabled, Images: []model.SecurityImage{}}
	index := make(map[string]int)
	for _, container := range containers {
		key := strings.TrimSpace(container.ImageID)
		if key == "" {
			key = strings.TrimSpace(container.Image)
		}
		if key == "" {
			continue
		}
		detail := details[container.ID]
		position, ok := index[key]
		if !ok {
			image := detail.ContainerSecurity
			image.Enabled = enabled
			if image.Image == "" {
				image.Image = container.Image
			}
			if image.ImageID == "" {
				image.ImageID = container.ImageID
			}
			name := container.Image
			if name == "" {
				name = image.Image
			}
			imageID := container.ImageID
			if imageID == "" {
				imageID = image.ImageID
			}
			vulnerabilities := detail.Vulnerabilities
			if vulnerabilities == nil {
				vulnerabilities = []model.ContainerVulnerability{}
			}
			position = len(overview.Images)
			index[key] = position
			overview.Images = append(overview.Images, model.SecurityImage{
				Image:           name,
				ImageID:         imageID,
				Containers:      []model.SecurityContainerRef{},
				Security:        image,
				Vulnerabilities: vulnerabilities,
			})
		}
		overview.Images[position].Containers = append(overview.Images[position].Containers, model.SecurityContainerRef{
			ID: container.ID, Name: container.Name, State: container.State,
		})
	}

	for index := range overview.Images {
		image := &overview.Images[index]
		security := image.Security
		overview.Totals.Images++
		overview.Totals.Containers += len(image.Containers)
		if security.Error != "" {
			overview.Totals.Failed++
		} else if security.Available {
			overview.Totals.ScannedImages++
			overview.Totals.ScannedContainers += len(image.Containers)
		}
		if security.Scanning {
			overview.Totals.Scanning++
		}
		if security.Stale {
			overview.Totals.Stale++
		}
		overview.Totals.Critical += security.Critical
		overview.Totals.High += security.High
		overview.Totals.Medium += security.Medium
		overview.Totals.Low += security.Low
		overview.Totals.Unknown += security.Unknown
		overview.Totals.Total += security.Total
		if security.ScannedAt != nil && (overview.Totals.LastScanAt == nil || security.ScannedAt.After(*overview.Totals.LastScanAt)) {
			scannedAt := *security.ScannedAt
			overview.Totals.LastScanAt = &scannedAt
		}
	}

	sort.SliceStable(overview.Images, func(i, j int) bool {
		left, right := overview.Images[i], overview.Images[j]
		leftPriority, rightPriority := imagePriority(left.Security), imagePriority(right.Security)
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		leftRisk, rightRisk := imageRisk(left.Security), imageRisk(right.Security)
		if leftRisk != rightRisk {
			return leftRisk > rightRisk
		}
		return left.Image < right.Image
	})
	return overview
}

func imagePriority(security model.ContainerSecurity) int {
	switch {
	case security.Error != "":
		return 0
	case security.Total > 0:
		return 1
	case security.Scanning:
		return 2
	case !security.Available:
		return 3
	default:
		return 4
	}
}

func imageRisk(security model.ContainerSecurity) int {
	return security.Critical*1_000_000_000 +
		security.High*10_000_000 +
		security.Medium*100_000 +
		security.Low*1_000 +
		security.Unknown
}
