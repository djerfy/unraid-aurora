package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"aurora/internal/model"
)

var containerNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

// Unraid keeps the autostart list outside Docker, one container per line.
const dockerAutostartFile = "/var/lib/docker/unraid-autostart"

var networkNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)

// CreateContainer creates and starts a container from the options gathered by
// the UI. The image is pulled first when it is not present locally.
func (l *Live) CreateContainer(ctx context.Context, opts model.ContainerCreateOptions) (_ string, err error) {
	if err := ValidateContainerCreateOptions(opts); err != nil {
		return "", err
	}
	if err := l.ensureImage(ctx, opts.Image); err != nil {
		return "", err
	}
	raw, err := l.dockerDo(ctx, http.MethodPost, "/containers/create?name="+url.QueryEscape(opts.Name), buildContainerCreateBody(opts))
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}
	var created struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(raw, &created); err != nil || created.ID == "" {
		return "", errors.New("docker did not return the new container id")
	}
	if _, err := l.dockerDo(ctx, http.MethodPost, "/containers/"+url.PathEscape(created.ID)+"/start", nil); err != nil {
		// Leaving a stopped container behind would confuse the next attempt.
		_, _ = l.dockerDo(context.WithoutCancel(ctx), http.MethodDelete, "/containers/"+url.PathEscape(created.ID)+"?force=1", nil)
		return "", fmt.Errorf("start container: %w", err)
	}
	l.setImageUpdate(opts.Image, false)
	l.saveImageUpdates()
	l.queueContainerScan(created.ID)
	if err := l.syncContainerTemplate(ctx, containerTemplateData{
		Name:          opts.Name,
		Image:         opts.Image,
		Network:       opts.NetworkMode,
		RestartPolicy: opts.RestartPolicy,
		Ports:         opts.Ports,
		Volumes:       opts.Volumes,
		Env:           opts.Env,
		Icon:          opts.Icon,
		WebUI:         opts.WebUI,
		Privileged:    opts.Privileged,
		Advanced:      opts.Advanced,
		AdvancedSet:   true,
	}); err != nil {
		slog.Error("create container template", "container", opts.Name, "error", err)
	}
	return created.ID, nil
}

// RemoveContainer deletes a container. Docker stops it first when force is set,
// matching the official webGUI "Remove" behaviour.
func (l *Live) RemoveContainer(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" || strings.ContainsAny(id, "/ \t\n\r\x00") {
		return errors.New("invalid container id")
	}
	if _, err := l.dockerDo(ctx, http.MethodDelete, "/containers/"+url.PathEscape(id)+"?force=1", nil); err != nil {
		return fmt.Errorf("remove container: %w", err)
	}
	return nil
}

// DockerNetworks lists the networks available to new containers.
func (l *Live) DockerNetworks(ctx context.Context) ([]string, error) {
	raw, err := l.dockerDo(ctx, http.MethodGet, "/networks", nil)
	if err != nil {
		return nil, err
	}
	var networks []struct {
		Name   string `json:"Name"`
		Driver string `json:"Driver"`
	}
	if err := json.Unmarshal(raw, &networks); err != nil {
		return nil, fmt.Errorf("decode docker networks: %w", err)
	}
	seen := map[string]bool{}
	names := []string{}
	for _, network := range networks {
		if network.Name != "" && !seen[network.Name] {
			seen[network.Name] = true
			names = append(names, network.Name)
		}
	}
	for _, builtin := range []string{"bridge", "host", "none"} {
		if !seen[builtin] {
			names = append(names, builtin)
		}
	}
	sort.Strings(names)
	return names, nil
}

func ValidateContainerCreateOptions(opts model.ContainerCreateOptions) error {
	if !containerNamePattern.MatchString(opts.Name) {
		return errors.New("container name must start with a letter or digit and contain only letters, digits, '_', '.' or '-'")
	}
	if strings.TrimSpace(opts.Image) == "" {
		return errors.New("an image repository is required")
	}
	if _, err := parseImageRef(opts.Image); err != nil {
		return err
	}
	if opts.NetworkMode != "" && !networkNamePattern.MatchString(opts.NetworkMode) {
		return fmt.Errorf("invalid network %q", opts.NetworkMode)
	}
	if opts.RestartPolicy == "" {
		return errors.New("a restart policy is required")
	}
	for _, value := range []string{opts.Icon, opts.WebUI} {
		if strings.ContainsAny(value, "\x00\n\r") {
			return errors.New("icon and web UI values must be single-line")
		}
	}
	advanced := opts.Advanced
	normalizeContainerAdvanced(&advanced)
	if err := validateContainerAdvanced(advanced); err != nil {
		return err
	}
	return ValidateContainerConfig(model.ContainerConfig{
		RestartPolicy: opts.RestartPolicy,
		Ports:         opts.Ports,
		Volumes:       opts.Volumes,
		Env:           opts.Env,
	})
}

func buildContainerCreateBody(opts model.ContainerCreateOptions) map[string]any {
	network := opts.NetworkMode
	if network == "" {
		network = "bridge"
	}
	env := envList(opts.Env)
	exposed := map[string]any{}
	bindings := map[string]any{}
	for _, port := range opts.Ports {
		key := fmt.Sprintf("%d/%s", port.Container, port.Protocol)
		exposed[key] = map[string]any{}
		bindings[key] = []any{map[string]any{"HostIp": "", "HostPort": fmt.Sprintf("%d", port.Host)}}
	}
	labels := map[string]any{"net.unraid.docker.managed": "dockerman"}
	if opts.Icon != "" {
		labels["net.unraid.docker.icon"] = opts.Icon
	}
	if opts.WebUI != "" {
		labels["net.unraid.docker.webui"] = opts.WebUI
	}
	config := map[string]any{
		"Image":        opts.Image,
		"Env":          env,
		"ExposedPorts": exposed,
		"Labels":       labels,
	}
	host := map[string]any{
		"Binds":         bindList(opts.Volumes),
		"PortBindings":  bindings,
		"NetworkMode":   network,
		"Privileged":    opts.Privileged,
		"RestartPolicy": map[string]any{"Name": opts.RestartPolicy, "MaximumRetryCount": 0},
	}
	applyContainerAdvanced(config, host, opts.Advanced)
	if opts.Advanced.MacAddress != "" && network != "host" && network != "none" && !strings.HasPrefix(network, "container:") {
		config["NetworkingConfig"] = map[string]any{
			"EndpointsConfig": map[string]any{
				network: map[string]any{"MacAddress": opts.Advanced.MacAddress},
			},
		}
	}
	config["HostConfig"] = host
	return config
}

func (l *Live) ensureImage(ctx context.Context, image string) error {
	if _, err := l.dockerDo(ctx, http.MethodGet, "/images/"+url.PathEscape(image)+"/json", nil); err == nil {
		return nil
	}
	return l.pullImage(ctx, image)
}

// UpdateContainerImage pulls the image again and recreates the container with
// its current settings, so no Unraid template is required. Each stage is
// reported to the caller so the UI can follow the operation live.
func (l *Live) UpdateContainerImage(ctx context.Context, id string, report func(model.ContainerUpdateEvent)) error {
	emit := func(event model.ContainerUpdateEvent) {
		if report != nil {
			report(event)
		}
	}
	cfg, err := l.ContainerConfig(ctx, id)
	if err != nil {
		return err
	}
	emit(model.ContainerUpdateEvent{Step: "pull", Message: "Pulling " + cfg.Image})
	if err := l.pullImageWithProgress(ctx, cfg.Image, func(message string, percent float64) {
		emit(model.ContainerUpdateEvent{Step: "pull", Message: message, Progress: percent})
	}); err != nil {
		return err
	}
	if err := l.updateContainerConfig(ctx, id, cfg, report); err != nil {
		return err
	}
	l.setImageUpdate(cfg.Image, false)
	l.saveImageUpdates()
	emit(model.ContainerUpdateEvent{Step: "done", Message: "Container updated", Progress: 100, Done: true})
	return nil
}

func (l *Live) pullImage(ctx context.Context, image string) error {
	return l.pullImageWithProgress(ctx, image, nil)
}

type dockerPullLayer struct {
	current uint64
	total   uint64
}

func (l *Live) pullImageWithProgress(ctx context.Context, image string, report func(message string, percent float64)) error {
	ref, err := parseImageRef(image)
	if err != nil {
		return err
	}
	query := url.Values{}
	if ref.digest != "" {
		query.Set("fromImage", ref.pullName()+"@"+ref.digest)
	} else {
		query.Set("fromImage", ref.pullName())
		query.Set("tag", ref.tag)
	}
	headers := map[string]string{}
	if creds := dockerCredentialsFor(ref.server); creds.Username != "" {
		if encoded, err := encodeRegistryAuth(creds, ref.server); err == nil {
			headers["X-Registry-Auth"] = encoded
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://docker/images/create?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	client := l.client
	if client == nil {
		client = &http.Client{}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("pull image %s: %w", image, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		message := strings.TrimSpace(string(payload))
		if message == "" {
			message = resp.Status
		}
		return fmt.Errorf("pull image %s: %s", image, message)
	}

	layers := map[string]*dockerPullLayer{}
	var lastMessage string
	lastPercent := -1.0
	decoder := json.NewDecoder(resp.Body)
	for {
		var event struct {
			Status string `json:"status"`
			ID     string `json:"id"`
			Error  string `json:"error"`
			Detail struct {
				Current uint64 `json:"current"`
				Total   uint64 `json:"total"`
			} `json:"progressDetail"`
			ErrorDetail struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
		}
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("pull image %s: %w", image, err)
		}
		if event.ErrorDetail.Message != "" {
			return fmt.Errorf("pull image %s: %s", image, event.ErrorDetail.Message)
		}
		if event.Error != "" {
			return fmt.Errorf("pull image %s: %s", image, event.Error)
		}
		if event.ID != "" && event.Detail.Total > 0 {
			layers[event.ID] = &dockerPullLayer{current: event.Detail.Current, total: event.Detail.Total}
		}
		if report == nil || event.Status == "" {
			continue
		}
		message := event.Status
		if event.ID != "" {
			message = fmt.Sprintf("%s %s", event.Status, shortLayerID(event.ID))
		}
		var current, total uint64
		for _, layer := range layers {
			current += layer.current
			total += layer.total
		}
		percent := 0.0
		if total > 0 {
			percent = float64(current) / float64(total) * 100
			if percent > 99 {
				percent = 99
			}
		}
		if message == lastMessage && percent < lastPercent+2 {
			continue
		}
		lastMessage = message
		lastPercent = percent
		report(message, percent)
	}
	if report != nil {
		report("Image pulled", 100)
	}
	return nil
}

func shortLayerID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func (l *Live) containerAutostart() map[string]bool {
	entries, err := readAutostartFile(dockerAutostartFile)
	if err != nil {
		return map[string]bool{}
	}
	names := map[string]bool{}
	for _, line := range entries {
		name, _, _ := strings.Cut(line, ";")
		if name = strings.TrimSpace(name); name != "" {
			names[name] = true
		}
	}
	return names
}

func readAutostartFile(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	entries := []string{}
	for _, line := range strings.Split(string(raw), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			entries = append(entries, line)
		}
	}
	return entries, nil
}

func updatedAutostartEntries(path, name string, enabled bool) ([]string, error) {
	entries, err := readAutostartFile(path)
	if err != nil {
		return nil, fmt.Errorf("read autostart file: %w", err)
	}
	kept := make([]string, 0, len(entries)+1)
	for _, line := range entries {
		entryName, _, _ := strings.Cut(line, ";")
		if strings.TrimSpace(entryName) != name {
			kept = append(kept, line)
		}
	}
	if enabled {
		kept = append(kept, name)
	}
	return kept, nil
}

func writeAutostartFile(path, name string, enabled bool) error {
	kept, err := updatedAutostartEntries(path, name, enabled)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create autostart directory: %w", err)
	}
	if len(kept) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove autostart file: %w", err)
		}
		return nil
	}
	if err := os.WriteFile(path, []byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil {
		return fmt.Errorf("write autostart file: %w", err)
	}
	return nil
}

func (l *Live) setContainerAutostart(name string, enabled bool) error {
	kept, err := updatedAutostartEntries(dockerAutostartFile, name, enabled)
	if err != nil {
		return err
	}
	if len(kept) == 0 {
		return l.deleteConfigFile("container.autostart", "Remove autostart entry · "+name, dockerAutostartFile)
	}
	return l.writeConfigFile(
		"container.autostart",
		fmt.Sprintf("Set autostart %s · %s", map[bool]string{true: "on", false: "off"}[enabled], name),
		dockerAutostartFile,
		[]byte(strings.Join(kept, "\n")+"\n"),
	)
}

// SetContainerAutostart updates Unraid's own autostart list so the official
// Docker page and the array startup script see the same setting.
func (l *Live) SetContainerAutostart(ctx context.Context, id string, enabled bool) error {
	inspect, err := l.inspectContainer(ctx, id)
	if err != nil {
		return err
	}
	name := strings.TrimPrefix(stringValue(inspect["Name"]), "/")
	if name == "" {
		return errors.New("container name is missing")
	}

	return l.setContainerAutostart(name, enabled)
}

func (l *Live) imageUpdate(image string) bool {
	l.imageMu.Lock()
	defer l.imageMu.Unlock()
	return l.imageUpdates[image]
}

func (l *Live) setImageUpdate(image string, update bool) {
	l.imageMu.Lock()
	defer l.imageMu.Unlock()
	if l.imageUpdates == nil {
		l.imageUpdates = map[string]bool{}
	}
	l.imageUpdates[image] = update
}

type imageUpdateState struct {
	CheckedAt time.Time       `json:"checkedAt"`
	Images    map[string]bool `json:"images"`
}

// loadImageUpdates restores the last check so update badges survive a browser
// refresh and an Aurora restart.
func (l *Live) loadImageUpdates() {
	if l.dataDir == "" {
		return
	}
	raw, err := os.ReadFile(filepath.Join(l.dataDir, "image-updates.json"))
	if err != nil {
		return
	}
	var state imageUpdateState
	if json.Unmarshal(raw, &state) != nil || state.Images == nil {
		return
	}
	l.imageMu.Lock()
	l.imageUpdates = state.Images
	l.imageCheckedAt = state.CheckedAt
	l.imageMu.Unlock()
}

func (l *Live) saveImageUpdates() {
	if l.dataDir == "" {
		return
	}
	l.imageMu.Lock()
	state := imageUpdateState{CheckedAt: l.imageCheckedAt, Images: make(map[string]bool, len(l.imageUpdates))}
	for image, update := range l.imageUpdates {
		state.Images[image] = update
	}
	l.imageMu.Unlock()

	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(l.dataDir, 0o750); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(l.dataDir, "image-updates.json"), raw, 0o600)
}

// CheckContainerUpdates compares the local repository digest of every image
// used by a container with the digest published by its registry, like the
// official webGUI "Check for Updates" button.
func (l *Live) CheckContainerUpdates(ctx context.Context) ([]model.ImageCheck, error) {
	containers, err := l.listContainers(ctx)
	if err != nil {
		return nil, err
	}
	images := make([]string, 0, len(containers))
	seen := map[string]bool{}
	for _, container := range containers {
		if container.Image == "" || seen[container.Image] {
			continue
		}
		seen[container.Image] = true
		images = append(images, container.Image)
	}

	results := make([]model.ImageCheck, len(images))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for index, image := range images {
		wg.Add(1)
		go func(index int, image string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[index] = l.checkImageUpdate(ctx, image)
		}(index, image)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool { return results[i].Image < results[j].Image })
	l.imageMu.Lock()
	l.imageCheckedAt = time.Now()
	l.imageMu.Unlock()
	l.saveImageUpdates()
	return results, nil
}

func (l *Live) checkImageUpdate(ctx context.Context, image string) model.ImageCheck {
	result := model.ImageCheck{Image: image}
	ref, err := parseImageRef(image)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	local, err := l.localImageDigest(ctx, image)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Local = local
	if local == "" {
		result.Error = "local image has no repository digest"
		return result
	}
	if ref.digest != "" {
		result.Remote = ref.digest
		result.Update = ref.digest != local
		l.setImageUpdate(image, result.Update)
		return result
	}
	remoteCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	remote, err := fetchRemoteDigest(remoteCtx, l.webHTTPClient(), ref, dockerCredentialsFor(ref.server))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Remote = remote
	result.Update = remote != "" && remote != local
	l.setImageUpdate(image, result.Update)
	return result
}

func (l *Live) localImageDigest(ctx context.Context, image string) (string, error) {
	raw, err := l.dockerDo(ctx, http.MethodGet, "/images/"+url.PathEscape(image)+"/json", nil)
	if err != nil {
		return "", err
	}
	var inspect struct {
		RepoDigests []string `json:"RepoDigests"`
	}
	if err := json.Unmarshal(raw, &inspect); err != nil {
		return "", fmt.Errorf("decode image inspect: %w", err)
	}
	if len(inspect.RepoDigests) == 0 {
		return "", nil
	}
	digest := inspect.RepoDigests[len(inspect.RepoDigests)-1]
	if index := strings.Index(digest, "@sha256:"); index >= 0 {
		return digest[index+1:], nil
	}
	return "", nil
}

func (l *Live) webHTTPClient() *http.Client {
	if l.webClient != nil {
		return l.webClient
	}
	return &http.Client{Timeout: 20 * time.Second}
}

type imageRef struct {
	apiBase    string
	server     string
	repository string
	tag        string
	digest     string
}

func (r imageRef) pullName() string {
	switch r.server {
	case "index.docker.io":
		return strings.TrimPrefix(r.repository, "library/")
	default:
		return r.server + "/" + r.repository
	}
}

func parseImageRef(image string) (imageRef, error) {
	image = strings.TrimSpace(image)
	if image == "" {
		return imageRef{}, errors.New("an image repository is required")
	}
	if strings.ContainsAny(image, " \t\n\r\x00") {
		return imageRef{}, errors.New("image references cannot contain whitespace")
	}
	ref := imageRef{tag: "latest"}
	if index := strings.Index(image, "@"); index > 0 {
		ref.digest = image[index+1:]
		image = image[:index]
	}
	first, rest, hasSlash := strings.Cut(image, "/")
	if hasSlash && (strings.ContainsAny(first, ".:") || first == "localhost") {
		ref.server = first
		ref.repository = rest
	} else {
		ref.repository = image
	}
	if ref.server == "" || ref.server == "docker.io" || ref.server == "registry-1.docker.io" {
		ref.server = "index.docker.io"
		ref.apiBase = "https://registry-1.docker.io"
		if !strings.Contains(ref.repository, "/") {
			ref.repository = "library/" + ref.repository
		}
	} else {
		ref.apiBase = "https://" + ref.server
	}
	if colon := strings.LastIndex(ref.repository, ":"); colon > strings.LastIndex(ref.repository, "/") {
		ref.tag = ref.repository[colon+1:]
		ref.repository = ref.repository[:colon]
	}
	if ref.repository == "" || ref.tag == "" {
		return imageRef{}, errors.New("image reference is incomplete")
	}
	return ref, nil
}

type dockerCredentials struct {
	Username string
	Password string
}

// dockerCredentialsFor reads the registry login Unraid stores for the Docker
// CLI, so private registries keep working without duplicating secrets.
func dockerCredentialsFor(server string) dockerCredentials {
	dir := os.Getenv("DOCKER_CONFIG")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return dockerCredentials{}
		}
		dir = filepath.Join(home, ".docker")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		return dockerCredentials{}
	}
	var config struct {
		Auths map[string]struct {
			Auth     string `json:"auth"`
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"auths"`
	}
	if json.Unmarshal(raw, &config) != nil {
		return dockerCredentials{}
	}
	candidates := []string{server, "https://" + server}
	if server == "index.docker.io" {
		candidates = append(candidates, "https://index.docker.io/v1/", "docker.io", "https://docker.io")
	}
	for _, key := range candidates {
		entry, ok := config.Auths[key]
		if !ok {
			continue
		}
		if entry.Username != "" {
			return dockerCredentials{Username: entry.Username, Password: entry.Password}
		}
		decoded, err := base64.StdEncoding.DecodeString(entry.Auth)
		if err != nil {
			continue
		}
		if user, password, ok := strings.Cut(string(decoded), ":"); ok {
			return dockerCredentials{Username: user, Password: password}
		}
	}
	return dockerCredentials{}
}

func encodeRegistryAuth(creds dockerCredentials, server string) (string, error) {
	raw, err := json.Marshal(map[string]string{
		"username":      creds.Username,
		"password":      creds.Password,
		"serveraddress": server,
	})
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

var manifestAccept = strings.Join([]string{
	"application/vnd.docker.distribution.manifest.list.v2+json",
	"application/vnd.docker.distribution.manifest.v2+json",
	"application/vnd.oci.image.index.v1+json",
	"application/vnd.oci.image.manifest.v1+json",
}, ", ")

func fetchRemoteDigest(ctx context.Context, client *http.Client, ref imageRef, creds dockerCredentials) (string, error) {
	reference := ref.tag
	if ref.digest != "" {
		reference = ref.digest
	}
	manifestURL := ref.apiBase + "/v2/" + ref.repository + "/manifests/" + reference

	request := func(authorization string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, manifestURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", manifestAccept)
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		return client.Do(req)
	}

	resp, err := request("")
	if err != nil {
		return "", fmt.Errorf("registry request: %w", err)
	}
	challenge := resp.Header.Get("Www-Authenticate")
	resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized && strings.HasPrefix(strings.ToLower(challenge), "bearer"):
		token, err := registryToken(ctx, client, challenge, creds)
		if err != nil {
			return "", err
		}
		resp, err = request("Bearer " + token)
		if err != nil {
			return "", fmt.Errorf("registry request: %w", err)
		}
		challenge = resp.Header.Get("Www-Authenticate")
		resp.Body.Close()
	case resp.StatusCode == http.StatusUnauthorized && strings.HasPrefix(strings.ToLower(challenge), "basic"):
		if creds.Username == "" {
			return "", errors.New("registry requires credentials")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, manifestURL, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Accept", manifestAccept)
		req.SetBasicAuth(creds.Username, creds.Password)
		resp, err = client.Do(req)
		if err != nil {
			return "", fmt.Errorf("registry request: %w", err)
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return "", errors.New("registry authentication failed")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry returned %s", resp.Status)
	}
	digest := strings.TrimSpace(resp.Header.Get("Docker-Content-Digest"))
	if digest == "" {
		return "", errors.New("registry did not return a digest")
	}
	return digest, nil
}

func registryToken(ctx context.Context, client *http.Client, challenge string, creds dockerCredentials) (string, error) {
	params := map[string]string{}
	for _, part := range strings.Split(strings.TrimPrefix(challenge, "Bearer "), ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		params[strings.ToLower(strings.TrimSpace(key))] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	realm := params["realm"]
	if realm == "" {
		return "", errors.New("registry challenge has no realm")
	}
	query := url.Values{}
	if service := params["service"]; service != "" {
		query.Set("service", service)
	}
	if scope := params["scope"]; scope != "" {
		query.Set("scope", scope)
	}
	target := realm
	if encoded := query.Encode(); encoded != "" {
		target += "?" + encoded
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", err
	}
	if creds.Username != "" {
		req.SetBasicAuth(creds.Username, creds.Password)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("registry token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry token returned %s", resp.Status)
	}
	var payload struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode registry token: %w", err)
	}
	if payload.Token != "" {
		return payload.Token, nil
	}
	if payload.AccessToken != "" {
		return payload.AccessToken, nil
	}
	return "", errors.New("registry token response was empty")
}
