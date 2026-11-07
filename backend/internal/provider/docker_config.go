package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"aurora/internal/model"
)

var envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var volumeNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func (l *Live) dockerDo(ctx context.Context, method, path string, body any) ([]byte, error) {
	return l.dockerDoHeaders(ctx, method, path, body, nil)
}

// dockerLongClient drops the short timeout used for list and summary calls.
// Container recreation, image pulls and stats reads can legitimately outlive
// it, and http.Client.Timeout also aborts streaming bodies.
func (l *Live) dockerLongClient() *http.Client {
	client := l.client
	if client == nil {
		return &http.Client{}
	}
	if client.Timeout == 0 {
		return client
	}
	clone := *client
	clone.Timeout = 0
	return &clone
}

func (l *Live) dockerDoHeaders(ctx context.Context, method, path string, body any, headers map[string]string) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://docker"+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := l.dockerLongClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusNotModified {
			return payload, nil
		}
		message := strings.TrimSpace(string(payload))
		if message == "" {
			message = resp.Status
		}
		return nil, fmt.Errorf("docker %s %s: %s", method, path, message)
	}
	return payload, nil
}

func (l *Live) inspectContainer(ctx context.Context, id string) (map[string]any, error) {
	raw, err := l.dockerDo(ctx, http.MethodGet, "/containers/"+url.PathEscape(id)+"/json", nil)
	if err != nil {
		return nil, err
	}
	var inspect map[string]any
	if err := json.Unmarshal(raw, &inspect); err != nil {
		return nil, fmt.Errorf("decode container inspect: %w", err)
	}
	return inspect, nil
}

func (l *Live) ContainerConfig(ctx context.Context, id string) (model.ContainerConfig, error) {
	inspect, err := l.inspectContainer(ctx, id)
	if err != nil {
		return model.ContainerConfig{}, err
	}
	config := childMap(inspect, "Config")
	host := childMap(inspect, "HostConfig")
	name := strings.TrimPrefix(stringValue(inspect["Name"]), "/")

	cfg := model.ContainerConfig{
		ID:            stringValue(inspect["Id"]),
		Name:          name,
		Image:         stringValue(config["Image"]),
		NetworkMode:   stringValue(host["NetworkMode"]),
		RestartPolicy: "no",
		Ports:         []model.Port{},
		Volumes:       []model.ContainerVolume{},
		Env:           []model.ContainerEnv{},
		Advanced:      parseContainerAdvanced(config, host),
		Privileged:    boolValue(host["Privileged"]),
	}
	if cfg.Advanced.MacAddress == "" {
		if settings := childMap(inspect, "NetworkSettings"); settings != nil {
			for _, raw := range childMap(settings, "Networks") {
				if endpoint := mapValue(raw); endpoint != nil {
					if address := stringValue(endpoint["MacAddress"]); address != "" {
						cfg.Advanced.MacAddress = address
						break
					}
				}
			}
		}
	}
	if template, ok := l.containerTemplateFor(name); ok {
		cfg.Advanced.ExtraParams = template.ExtraParams
		cfg.Advanced.PostArgs = template.PostArgs
	}
	if policy := childMap(host, "RestartPolicy"); policy != nil {
		cfg.RestartPolicy = stringValue(policy["Name"])
	}
	for _, value := range stringSlice(config["Env"]) {
		key, val, ok := strings.Cut(value, "=")
		if !ok {
			key, val = value, ""
		}
		cfg.Env = append(cfg.Env, model.ContainerEnv{Key: key, Value: val})
	}
	for _, binding := range stringSlice(host["Binds"]) {
		if volume, ok := parseBind(binding); ok {
			cfg.Volumes = append(cfg.Volumes, volume)
		}
	}
	for key, raw := range childMap(host, "PortBindings") {
		containerPort, protocol, ok := strings.Cut(key, "/")
		if !ok {
			protocol = "tcp"
		}
		portNumber := atoi(containerPort)
		for _, item := range anySlice(raw) {
			entry := mapValue(item)
			hostPort := atoi(stringValue(entry["HostPort"]))
			if hostPort == 0 {
				continue
			}
			cfg.Ports = append(cfg.Ports, model.Port{Host: hostPort, Container: portNumber, Protocol: protocol})
		}
	}
	sort.Slice(cfg.Ports, func(i, j int) bool { return cfg.Ports[i].Host < cfg.Ports[j].Host })
	sort.Slice(cfg.Volumes, func(i, j int) bool { return cfg.Volumes[i].Destination < cfg.Volumes[j].Destination })
	return cfg, nil
}

// UpdateContainerConfig recreates the container with the edited mounts, ports,
// environment and restart policy. The original container is stopped, renamed as
// a backup and restored if any step fails.
func (l *Live) UpdateContainerConfig(ctx context.Context, id string, cfg model.ContainerConfig) error {
	return l.updateContainerConfig(ctx, id, cfg, nil, true, true)
}

// updateContainerConfig recreates a container with new settings. syncTemplate
// is false for image updates: the template already describes the container and
// must not be rewritten from Docker inspect data, which includes image labels,
// generated MAC addresses and other runtime defaults.
func (l *Live) updateContainerConfig(ctx context.Context, id string, cfg model.ContainerConfig, report func(model.ContainerUpdateEvent), syncTemplate, syncExtra bool) error {
	emit := func(step, message string) {
		if report != nil {
			report(model.ContainerUpdateEvent{Step: step, Message: message})
		}
	}
	if err := ValidateContainerConfig(cfg); err != nil {
		return err
	}
	inspect, err := l.inspectContainer(ctx, id)
	if err != nil {
		return err
	}
	config := childMap(inspect, "Config")
	host := childMap(inspect, "HostConfig")
	if config == nil || host == nil {
		return errors.New("docker inspect returned an incomplete container")
	}
	oldID := stringValue(inspect["Id"])
	name := strings.TrimPrefix(stringValue(inspect["Name"]), "/")
	if oldID == "" || name == "" {
		return errors.New("container identity is incomplete")
	}
	running := false
	if state := childMap(inspect, "State"); state != nil {
		running = boolValue(state["Running"])
	}

	// Rebuild only the editable fields, keeping every other Docker option.
	config["Env"] = envList(cfg.Env)
	delete(config, "Hostname")
	delete(config, "Domainname")
	delete(config, "MacAddress")
	exposed := map[string]any{}
	bindings := map[string]any{}
	for _, port := range cfg.Ports {
		key := fmt.Sprintf("%d/%s", port.Container, port.Protocol)
		exposed[key] = map[string]any{}
		bindings[key] = []any{map[string]any{"HostIp": "", "HostPort": fmt.Sprintf("%d", port.Host)}}
	}
	config["ExposedPorts"] = exposed
	host["PortBindings"] = bindings
	host["Binds"] = bindList(cfg.Volumes)
	host["RestartPolicy"] = map[string]any{
		"Name":              cfg.RestartPolicy,
		"MaximumRetryCount": 0,
	}
	// Mounts would duplicate Binds when present; Unraid stores user paths as
	// Binds, so let the edited bind list be authoritative.
	delete(host, "Mounts")
	for _, key := range []string{
		"Devices", "CapAdd", "CapDrop", "Sysctls", "Ulimits", "ExtraHosts", "Dns", "DnsSearch",
		"Tmpfs", "Memory", "NanoCpus", "CpusetCpus", "PidsLimit", "ReadonlyRootfs",
	} {
		delete(host, key)
	}
	if labels := childMap(config, "Labels"); labels != nil {
		for key := range labels {
			if !strings.HasPrefix(strings.ToLower(key), "net.unraid.") {
				delete(labels, key)
			}
		}
	}
	advanced := cfg.Advanced
	if syncExtra && strings.TrimSpace(advanced.ExtraParams) != "" {
		advanced = mergeContainerAdvanced(advanced, containerAdvancedFromExtraParams(advanced.ExtraParams))
	}
	applyContainerAdvanced(config, host, advanced)
	host["Privileged"] = cfg.Privileged
	previousPostArgs := ""
	if template, ok := l.containerTemplateFor(name); ok {
		previousPostArgs = template.PostArgs
	}
	applyPostArgs(config, previousPostArgs, advanced.PostArgs)

	body := map[string]any{}
	for key, value := range config {
		body[key] = value
	}
	body["HostConfig"] = host
	if endpoints := networkEndpoints(inspect); len(endpoints) > 0 {
		mode := stringValue(host["NetworkMode"])
		if mode != "host" && mode != "none" && !strings.HasPrefix(mode, "container:") {
			if cfg.Advanced.MacAddress != "" {
				for _, raw := range endpoints {
					if endpoint := mapValue(raw); endpoint != nil {
						endpoint["MacAddress"] = cfg.Advanced.MacAddress
					}
				}
			}
			body["NetworkingConfig"] = map[string]any{"EndpointsConfig": endpoints}
		}
	}

	backupName := fmt.Sprintf("%s-aurora-backup-%d", name, time.Now().Unix())
	if len(backupName) > 200 {
		backupName = backupName[:200]
	}
	if running {
		emit("stop", "Stopping "+name)
		if _, err := l.dockerDo(ctx, http.MethodPost, "/containers/"+url.PathEscape(oldID)+"/stop?t=20", nil); err != nil {
			return fmt.Errorf("stop container: %w", err)
		}
	}
	if _, err := l.dockerDo(ctx, http.MethodPost, "/containers/"+url.PathEscape(oldID)+"/rename?name="+url.QueryEscape(backupName), nil); err != nil {
		l.restartIfRunning(ctx, oldID, running)
		return fmt.Errorf("rename container: %w", err)
	}

	emit("create", "Recreating "+name)
	created, createErr := l.dockerDo(ctx, http.MethodPost, "/containers/create?name="+url.QueryEscape(name), body)
	if createErr != nil {
		l.restoreContainer(ctx, oldID, name, running)
		return fmt.Errorf("create replacement container: %w", createErr)
	}
	var createdInfo struct {
		ID string `json:"Id"`
	}
	_ = json.Unmarshal(created, &createdInfo)
	if createdInfo.ID == "" {
		l.restoreContainer(ctx, oldID, name, running)
		return errors.New("docker did not return the replacement container id")
	}
	if running {
		emit("start", "Starting "+name)
		if _, err := l.dockerDo(ctx, http.MethodPost, "/containers/"+url.PathEscape(createdInfo.ID)+"/start", nil); err != nil {
			_, _ = l.dockerDo(ctx, http.MethodDelete, "/containers/"+url.PathEscape(createdInfo.ID)+"?force=1", nil)
			l.restoreContainer(ctx, oldID, name, running)
			return fmt.Errorf("start replacement container: %w", err)
		}
	}
	// The new container is healthy; drop the backup. A failure here leaves a
	// stopped backup but does not invalidate the successful update.
	emit("cleanup", "Removing the previous container")
	_, _ = l.dockerDo(ctx, http.MethodDelete, "/containers/"+url.PathEscape(oldID)+"?force=1", nil)
	// Keep the Unraid template in sync so the official webGUI and its update
	// scripts recreate the container with the same settings. Image updates skip
	// this: Docker inspect carries image labels and generated values that do
	// not belong in the template.
	if syncTemplate {
		l.syncTemplateAfterUpdate(ctx, cfg, syncExtra)
	}
	l.queueContainerScan(createdInfo.ID)
	return nil
}

func applyPostArgs(config map[string]any, previous, next string) {
	previousTokens := parseShellArgs(previous)
	current := stringSlice(config["Cmd"])
	base := append([]string(nil), current...)
	if len(previousTokens) > 0 && len(base) >= len(previousTokens) {
		match := true
		offset := len(base) - len(previousTokens)
		for index := range previousTokens {
			if base[offset+index] != previousTokens[index] {
				match = false
				break
			}
		}
		if match {
			base = base[:offset]
		}
	}
	nextTokens := parseShellArgs(next)
	if len(nextTokens) > 0 {
		config["Cmd"] = append(append([]string{}, base...), nextTokens...)
		return
	}
	if len(previousTokens) > 0 {
		if len(base) == 0 {
			delete(config, "Cmd")
		} else {
			config["Cmd"] = base
		}
	}
}

func (l *Live) restartIfRunning(ctx context.Context, id string, running bool) {
	if running {
		_, _ = l.dockerDo(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/start", nil)
	}
}

func (l *Live) restoreContainer(ctx context.Context, oldID, name string, running bool) {
	_, _ = l.dockerDo(ctx, http.MethodPost, "/containers/"+url.PathEscape(oldID)+"/rename?name="+url.QueryEscape(name), nil)
	l.restartIfRunning(ctx, oldID, running)
}

func ValidateContainerConfig(cfg model.ContainerConfig) error {
	switch cfg.RestartPolicy {
	case "no", "always", "unless-stopped", "on-failure":
	default:
		return fmt.Errorf("unsupported restart policy %q", cfg.RestartPolicy)
	}
	destinations := map[string]bool{}
	for _, volume := range cfg.Volumes {
		if volume.Source == "" || volume.Destination == "" {
			return errors.New("volume source and destination are required")
		}
		if !filepath.IsAbs(volume.Destination) {
			return fmt.Errorf("volume destination %q must be absolute", volume.Destination)
		}
		if !filepath.IsAbs(volume.Source) && !volumeNamePattern.MatchString(volume.Source) {
			return fmt.Errorf("volume source %q must be an absolute path or a named volume", volume.Source)
		}
		if strings.ContainsAny(volume.Source+volume.Destination, "\x00\n\r") {
			return errors.New("volume paths contain control characters")
		}
		if destinations[volume.Destination] {
			return fmt.Errorf("duplicate volume destination %q", volume.Destination)
		}
		destinations[volume.Destination] = true
	}
	hostPorts := map[string]bool{}
	for _, port := range cfg.Ports {
		if port.Host < 1 || port.Host > 65535 || port.Container < 1 || port.Container > 65535 {
			return errors.New("ports must be between 1 and 65535")
		}
		switch port.Protocol {
		case "tcp", "udp", "sctp":
		default:
			return fmt.Errorf("unsupported protocol %q", port.Protocol)
		}
		key := fmt.Sprintf("%d/%s", port.Host, port.Protocol)
		if hostPorts[key] {
			return fmt.Errorf("duplicate host port %s", key)
		}
		hostPorts[key] = true
	}
	for _, env := range cfg.Env {
		if !envKeyPattern.MatchString(env.Key) {
			return fmt.Errorf("invalid environment variable name %q", env.Key)
		}
		if strings.ContainsAny(env.Value, "\x00\n\r") {
			return fmt.Errorf("environment variable %s contains control characters", env.Key)
		}
	}
	advanced := cfg.Advanced
	normalizeContainerAdvanced(&advanced)
	return validateContainerAdvanced(advanced)
}

func parseBind(value string) (model.ContainerVolume, bool) {
	parts := strings.Split(value, ":")
	if len(parts) < 2 {
		return model.ContainerVolume{}, false
	}
	source, destination := parts[0], parts[1]
	readOnly := false
	if len(parts) > 2 {
		for _, option := range strings.Split(parts[2], ",") {
			if option == "ro" {
				readOnly = true
			}
		}
	}
	if source == "" || destination == "" {
		return model.ContainerVolume{}, false
	}
	return model.ContainerVolume{Source: source, Destination: destination, ReadOnly: readOnly}, true
}

func bindList(volumes []model.ContainerVolume) []any {
	out := make([]any, 0, len(volumes))
	for _, volume := range volumes {
		value := volume.Source + ":" + volume.Destination
		if volume.ReadOnly {
			value += ":ro"
		}
		out = append(out, value)
	}
	return out
}

func envList(env []model.ContainerEnv) []any {
	out := make([]any, 0, len(env))
	for _, item := range env {
		out = append(out, item.Key+"="+item.Value)
	}
	return out
}

func networkEndpoints(inspect map[string]any) map[string]any {
	networkSettings := childMap(inspect, "NetworkSettings")
	networks := childMap(networkSettings, "Networks")
	endpoints := map[string]any{}
	for name, raw := range networks {
		network := mapValue(raw)
		if network == nil {
			continue
		}
		entry := map[string]any{}
		for _, key := range []string{"Aliases", "IPAMConfig", "Links", "DriverOpts"} {
			if value, ok := network[key]; ok && value != nil {
				entry[key] = value
			}
		}
		endpoints[name] = entry
	}
	return endpoints
}

func childMap(parent map[string]any, key string) map[string]any {
	if parent == nil {
		return nil
	}
	return mapValue(parent[key])
}

func mapValue(value any) map[string]any {
	if mapped, ok := value.(map[string]any); ok {
		return mapped
	}
	return nil
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func boolValue(value any) bool {
	if flag, ok := value.(bool); ok {
		return flag
	}
	return false
}

func stringSlice(value any) []string {
	items := anySlice(value)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}
	return out
}

func anySlice(value any) []any {
	if items, ok := value.([]any); ok {
		return items
	}
	return nil
}

func atoi(value string) int {
	var n int
	_, _ = fmt.Sscanf(value, "%d", &n)
	return n
}
