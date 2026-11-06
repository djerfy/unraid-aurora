package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"aurora/internal/model"
)

type dockerImageInfo struct {
	ID          string   `json:"Id"`
	RepoTags    []string `json:"RepoTags"`
	RepoDigests []string `json:"RepoDigests"`
	Created     int64    `json:"Created"`
	Size        int64    `json:"Size"`
	SharedSize  int64    `json:"SharedSize"`
}

type dockerContainerImageRef struct {
	ID      string   `json:"Id"`
	Names   []string `json:"Names"`
	ImageID string   `json:"ImageID"`
}

// ImageInUseError is returned when Docker still references an image from a
// container. The caller must not force the removal.
type ImageInUseError struct {
	Names []string
}

func (e *ImageInUseError) Error() string {
	if len(e.Names) == 0 {
		return "image is still used by a container"
	}
	return "image is still used by: " + strings.Join(e.Names, ", ")
}

var imageIDPattern = regexp.MustCompile(`^(sha256:)?[a-fA-F0-9]{12,64}$`)

func validImageID(id string) bool {
	return imageIDPattern.MatchString(strings.TrimSpace(id))
}

func (l *Live) ContainerImages(ctx context.Context) ([]model.ContainerImage, error) {
	rawImages, err := l.dockerDo(ctx, http.MethodGet, "/images/json", nil)
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}
	var images []dockerImageInfo
	if err := json.Unmarshal(rawImages, &images); err != nil {
		return nil, fmt.Errorf("decode images: %w", err)
	}
	rawContainers, err := l.dockerDo(ctx, http.MethodGet, "/containers/json?all=1", nil)
	if err != nil {
		return nil, fmt.Errorf("list image users: %w", err)
	}
	var containers []dockerContainerImageRef
	if err := json.Unmarshal(rawContainers, &containers); err != nil {
		return nil, fmt.Errorf("decode image users: %w", err)
	}
	return buildContainerImages(images, containers), nil
}

func buildContainerImages(images []dockerImageInfo, containers []dockerContainerImageRef) []model.ContainerImage {
	usedBy := map[string][]string{}
	for _, container := range containers {
		name := ""
		if len(container.Names) > 0 {
			name = strings.TrimPrefix(container.Names[0], "/")
		}
		if name == "" {
			name = shortImageID(container.ID)
		}
		usedBy[container.ImageID] = append(usedBy[container.ImageID], name)
	}
	out := make([]model.ContainerImage, 0, len(images))
	for _, image := range images {
		names := append([]string(nil), usedBy[image.ID]...)
		sort.Strings(names)
		tags := append([]string(nil), image.RepoTags...)
		if tags == nil {
			tags = []string{}
		}
		digests := append([]string(nil), image.RepoDigests...)
		if digests == nil {
			digests = []string{}
		}
		dangling := len(tags) == 0
		for _, tag := range tags {
			if tag != "<none>:<none>" {
				dangling = false
				break
			}
		}
		out = append(out, model.ContainerImage{
			ID: image.ID, RepoTags: tags, RepoDigests: digests,
			Size: image.Size, SharedSize: image.SharedSize,
			Created:    time.Unix(image.Created, 0).UTC(),
			Containers: len(names), UsedBy: names,
			InUse: len(names) > 0, Dangling: dangling,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].InUse != out[j].InUse {
			return !out[i].InUse
		}
		return containerImageName(out[i]) < containerImageName(out[j])
	})
	return out
}

func containerImageName(image model.ContainerImage) string {
	if len(image.RepoTags) > 0 {
		return image.RepoTags[0]
	}
	return image.ID
}

func shortImageID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func (l *Live) imageUsage(ctx context.Context, id string) ([]string, error) {
	raw, err := l.dockerDo(ctx, http.MethodGet, "/containers/json?all=1", nil)
	if err != nil {
		return nil, err
	}
	var containers []dockerContainerImageRef
	if err := json.Unmarshal(raw, &containers); err != nil {
		return nil, err
	}
	names := []string{}
	for _, container := range containers {
		if container.ImageID != id {
			continue
		}
		name := ""
		if len(container.Names) > 0 {
			name = strings.TrimPrefix(container.Names[0], "/")
		}
		if name == "" {
			name = shortImageID(container.ID)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (l *Live) deleteImage(ctx context.Context, id string) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, "http://docker/images/"+url.PathEscape(id), nil)
	if err != nil {
		return 0, "", err
	}
	resp, err := l.dockerLongClient().Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, strings.TrimSpace(string(payload)), nil
}

func (l *Live) RemoveContainerImage(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if !validImageID(id) {
		return errors.New("invalid image id")
	}
	names, err := l.imageUsage(ctx, id)
	if err != nil {
		return fmt.Errorf("check image usage: %w", err)
	}
	if len(names) > 0 {
		return &ImageInUseError{Names: names}
	}
	status, body, err := l.deleteImage(ctx, id)
	if err != nil {
		return fmt.Errorf("remove image: %w", err)
	}
	switch {
	case status == http.StatusNotFound:
		return nil
	case status == http.StatusConflict:
		return &ImageInUseError{}
	case status < 200 || status >= 300:
		if body == "" {
			body = http.StatusText(status)
		}
		return fmt.Errorf("remove image: %s", body)
	}
	return nil
}

func (l *Live) RemoveUnusedContainerImages(ctx context.Context) (model.ContainerImageCleanup, error) {
	images, err := l.ContainerImages(ctx)
	if err != nil {
		return model.ContainerImageCleanup{}, err
	}
	result := model.ContainerImageCleanup{
		Removed: []string{}, Skipped: []model.ContainerImageSkip{}, Failed: []model.ContainerImageSkip{},
	}
	for _, image := range images {
		if image.InUse {
			result.Skipped = append(result.Skipped, model.ContainerImageSkip{ID: image.ID, Reason: "used by " + strings.Join(image.UsedBy, ", ")})
			continue
		}
		if err := l.RemoveContainerImage(ctx, image.ID); err != nil {
			var inUse *ImageInUseError
			if errors.As(err, &inUse) {
				result.Skipped = append(result.Skipped, model.ContainerImageSkip{ID: image.ID, Reason: err.Error()})
				continue
			}
			result.Failed = append(result.Failed, model.ContainerImageSkip{ID: image.ID, Reason: err.Error()})
			continue
		}
		result.Removed = append(result.Removed, image.ID)
	}
	return result, nil
}

// removeImageIfUnused is a best-effort cleanup after an image update. It never
// forces Docker and silently keeps an image that another container still uses.
func (l *Live) removeImageIfUnused(ctx context.Context, id string) {
	if !validImageID(id) {
		return
	}
	names, err := l.imageUsage(ctx, id)
	if err != nil {
		slog.Debug("image cleanup skipped", "image", id, "error", err)
		return
	}
	if len(names) > 0 {
		slog.Debug("image kept because it is still used", "image", id, "containers", names)
		return
	}
	status, body, err := l.deleteImage(ctx, id)
	if err != nil {
		slog.Warn("image cleanup failed", "image", id, "error", err)
		return
	}
	if status == http.StatusConflict || status == http.StatusNotFound || (status >= 200 && status < 300) {
		return
	}
	slog.Warn("image cleanup failed", "image", id, "status", status, "response", body)
}

func (l *Live) containerImageID(ctx context.Context, id string) string {
	inspect, err := l.inspectContainer(ctx, id)
	if err != nil {
		return ""
	}
	return stringValue(inspect["Image"])
}
