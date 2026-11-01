package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurora/internal/model"
)

const pluginManagerScript = "/usr/local/emhttp/plugins/dynamix.plugin.manager/scripts/plugin"

var (
	plgEntityPattern = regexp.MustCompile(`(?i)<!ENTITY\s+([A-Za-z0-9_.-]+)\s+"([^"]*)"`)
	plgTagPattern    = regexp.MustCompile(`(?is)<PLUGIN\b([^>]*)>`)
	plgAttrPattern   = regexp.MustCompile(`([A-Za-z][A-Za-z0-9_.:-]*)\s*=\s*"([^"]*)"`)
	plgDescPattern   = regexp.MustCompile(`(?is)<DESCRIPTION>\s*(?:<!\[CDATA\[)?(.*?)(?:\]\]>)?\s*</DESCRIPTION>`)
	plgTagStrip      = regexp.MustCompile(`(?s)<[^>]+>`)
	plgSpace         = regexp.MustCompile(`\s+`)
	versionPart      = regexp.MustCompile(`[0-9]+|[A-Za-z]+`)
)

type pluginUpdateState struct {
	CheckedAt time.Time               `json:"checkedAt"`
	Plugins   map[string]pluginUpdate `json:"plugins"`
}

type pluginUpdate struct {
	Update bool   `json:"update"`
	Latest string `json:"latest"`
}

func (l *Live) pluginList() []model.Plugin {
	directories := []string{"/var/log/plugins", filepath.Join(l.bootDir, "config", "plugins")}
	seen := map[string]bool{}
	plugins := make([]model.Plugin, 0)
	for _, directory := range directories {
		entries, err := os.ReadDir(directory)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !strings.HasSuffix(strings.ToLower(entry.Name()), ".plg") {
				continue
			}
			path := filepath.Join(directory, entry.Name())
			if info, err := os.Stat(path); err != nil || info.IsDir() {
				continue
			}
			plugin, err := parsePluginFile(path)
			if err != nil {
				slog.Warn("parse plugin", "file", entry.Name(), "error", err)
				continue
			}
			key := strings.ToLower(plugin.Name)
			if seen[key] {
				continue
			}
			seen[key] = true
			plugin.Filename = entry.Name()
			l.pluginMu.Lock()
			if update, ok := l.pluginUpdates[plugin.Name]; ok {
				plugin.UpdateAvailable = update.Update
				plugin.Latest = update.Latest
			}
			l.pluginMu.Unlock()
			plugins = append(plugins, plugin)
		}
	}
	sort.Slice(plugins, func(i, j int) bool { return strings.ToLower(plugins[i].Name) < strings.ToLower(plugins[j].Name) })
	return plugins
}

func parsePluginFile(path string) (model.Plugin, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return model.Plugin{}, err
	}
	if len(raw) > 1<<20 {
		raw = raw[:1<<20]
	}
	content := string(raw)
	entities := map[string]string{}
	for _, match := range plgEntityPattern.FindAllStringSubmatch(content, -1) {
		entities[match[1]] = match[2]
	}
	resolve := func(value string) string {
		for pass := 0; pass < 3; pass++ {
			replaced := false
			for key, replacement := range entities {
				reference := "&" + key + ";"
				if strings.Contains(value, reference) {
					value = strings.ReplaceAll(value, reference, replacement)
					replaced = true
				}
			}
			if !replaced {
				break
			}
		}
		value = strings.NewReplacer("&amp;", "&", "&quot;", `"`, "&lt;", "<", "&gt;", ">").Replace(value)
		return strings.TrimSpace(value)
	}

	tag := plgTagPattern.FindStringSubmatch(content)
	if len(tag) < 2 {
		return model.Plugin{}, fmt.Errorf("missing PLUGIN element")
	}
	attributes := map[string]string{}
	for _, match := range plgAttrPattern.FindAllStringSubmatch(tag[1], -1) {
		attributes[strings.ToLower(match[1])] = resolve(match[2])
	}
	name := attributes["name"]
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	plugin := model.Plugin{
		Name:      name,
		Version:   attributes["version"],
		Author:    attributes["author"],
		Category:  attributes["category"],
		PluginURL: attributes["pluginurl"],
		Enabled:   true,
		Color:     colorFor(name),
	}
	if plugin.Category == "" {
		plugin.Category = "Plugin"
	}
	if description := plgDescPattern.FindStringSubmatch(content); len(description) > 1 {
		plugin.Description = cleanPluginDescription(description[1])
	}
	return plugin, nil
}

func cleanPluginDescription(value string) string {
	value = strings.ReplaceAll(value, "<![CDATA[", "")
	value = strings.ReplaceAll(value, "]]>", "")
	value = plgTagStrip.ReplaceAllString(value, " ")
	value = plgSpace.ReplaceAllString(value, " ")
	value = strings.TrimSpace(value)
	if len(value) > 400 {
		value = value[:400] + "…"
	}
	return value
}

// CheckPluginUpdates downloads every plugin definition to compare versions,
// like the official Plugins page update check.
func (l *Live) CheckPluginUpdates(ctx context.Context) ([]model.Plugin, error) {
	plugins := l.pluginList()
	client := l.webHTTPClient()
	results := make([]pluginUpdate, len(plugins))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for index, plugin := range plugins {
		if strings.TrimSpace(plugin.PluginURL) == "" {
			continue
		}
		wg.Add(1)
		go func(index int, plugin model.Plugin) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			requestCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			remote, err := fetchPluginVersion(requestCtx, client, plugin.PluginURL)
			if err != nil {
				slog.Warn("check plugin update", "plugin", plugin.Name, "error", err)
				return
			}
			if remote == "" || !versionNewer(remote, plugin.Version) {
				return
			}
			results[index] = pluginUpdate{Update: true, Latest: remote}
		}(index, plugin)
	}
	wg.Wait()

	l.pluginMu.Lock()
	if l.pluginUpdates == nil {
		l.pluginUpdates = map[string]pluginUpdate{}
	}
	for index, plugin := range plugins {
		l.pluginUpdates[plugin.Name] = results[index]
	}
	l.pluginCheckedAt = time.Now()
	l.pluginMu.Unlock()
	l.savePluginUpdates()
	return l.pluginList(), nil
}

func fetchPluginVersion(ctx context.Context, client *http.Client, url string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "Aurora")
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("plugin url returned %s", response.Status)
	}
	raw := make([]byte, 0, 64<<10)
	buffer := make([]byte, 32<<10)
	for len(raw) < 1<<20 {
		read, err := response.Body.Read(buffer)
		raw = append(raw, buffer[:read]...)
		if err != nil {
			break
		}
	}
	tag := plgTagPattern.FindSubmatch(raw)
	if len(tag) < 2 {
		return "", fmt.Errorf("remote plugin definition is missing PLUGIN")
	}
	attributes := map[string]string{}
	for _, match := range plgAttrPattern.FindAllSubmatch(tag[1], -1) {
		attributes[strings.ToLower(string(match[1]))] = strings.TrimSpace(string(match[2]))
	}
	return attributes["version"], nil
}

func versionNewer(remote, local string) bool {
	remoteParts := versionPart.FindAllString(remote, -1)
	localParts := versionPart.FindAllString(local, -1)
	for index := 0; index < len(remoteParts) || index < len(localParts); index++ {
		var left, right string
		if index < len(remoteParts) {
			left = remoteParts[index]
		}
		if index < len(localParts) {
			right = localParts[index]
		}
		leftNumber, leftErr := strconv.Atoi(left)
		rightNumber, rightErr := strconv.Atoi(right)
		if leftErr == nil && rightErr == nil {
			if leftNumber != rightNumber {
				return leftNumber > rightNumber
			}
			continue
		}
		if comparison := strings.Compare(strings.ToLower(left), strings.ToLower(right)); comparison != 0 {
			return comparison > 0
		}
	}
	return false
}

// InstallPlugin downloads and installs a .plg definition through the official
// plugin manager. The caller must explicitly confirm the URL.
func (l *Live) InstallPlugin(ctx context.Context, rawURL string, report func(model.PluginEvent)) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.New("plugin URL must be an http(s) URL")
	}
	if info, err := os.Stat(pluginManagerScript); err != nil || info.IsDir() {
		return errors.New("plugin manager is not available")
	}
	return l.runPluginCommand(ctx, "install", parsed.String(), report)
}

func (l *Live) RemovePlugin(ctx context.Context, name string, report func(model.PluginEvent)) error {
	plugins := l.pluginList()
	filename := ""
	for _, plugin := range plugins {
		if strings.EqualFold(plugin.Name, name) || strings.EqualFold(plugin.Filename, name) {
			filename = plugin.Filename
			break
		}
	}
	if filename == "" {
		return fmt.Errorf("plugin %q not found", name)
	}
	if info, err := os.Stat(pluginManagerScript); err != nil || info.IsDir() {
		return errors.New("plugin manager is not available")
	}
	return l.runPluginCommand(ctx, "remove", filename, report)
}

func (l *Live) runPluginCommand(ctx context.Context, action, argument string, report func(model.PluginEvent)) error {
	emit := func(event model.PluginEvent) {
		if report != nil {
			report(event)
		}
	}
	command := exec.CommandContext(ctx, pluginManagerScript, action, argument)
	writer := &pluginLineWriter{emit: func(line string) { emit(model.PluginEvent{Line: line}) }}
	command.Stdout = writer
	command.Stderr = writer
	err := command.Run()
	writer.flush()
	if err != nil {
		emit(model.PluginEvent{Line: err.Error(), Error: true, Done: true})
		return err
	}
	l.pluginMu.Lock()
	delete(l.pluginUpdates, argument)
	for key := range l.pluginUpdates {
		if strings.EqualFold(key, argument) {
			delete(l.pluginUpdates, key)
		}
	}
	l.pluginMu.Unlock()
	l.savePluginUpdates()
	emit(model.PluginEvent{Done: true, Line: "Plugin " + action + " completed"})
	return nil
}

// UpdatePlugin runs the official plugin manager and streams its output.
func (l *Live) UpdatePlugin(ctx context.Context, name string, report func(model.PluginEvent)) error {
	emit := func(event model.PluginEvent) {
		if report != nil {
			report(event)
		}
	}
	plugins := l.pluginList()
	var filename string
	for _, plugin := range plugins {
		if strings.EqualFold(plugin.Name, name) || strings.EqualFold(plugin.Filename, name) {
			filename = plugin.Filename
			break
		}
	}
	if filename == "" {
		return fmt.Errorf("plugin %q not found", name)
	}
	if info, err := os.Stat(pluginManagerScript); err != nil || info.IsDir() {
		return fmt.Errorf("plugin manager is not available")
	}
	command := exec.CommandContext(ctx, pluginManagerScript, "update", filename)
	writer := &pluginLineWriter{emit: func(line string) {
		emit(model.PluginEvent{Line: line})
	}}
	command.Stdout = writer
	command.Stderr = writer
	err := command.Run()
	writer.flush()
	if err != nil {
		emit(model.PluginEvent{Line: err.Error(), Error: true, Done: true})
		return err
	}
	l.pluginMu.Lock()
	delete(l.pluginUpdates, name)
	for key := range l.pluginUpdates {
		if strings.EqualFold(key, name) {
			delete(l.pluginUpdates, key)
		}
	}
	l.pluginMu.Unlock()
	l.savePluginUpdates()
	emit(model.PluginEvent{Done: true, Line: "Plugin updated"})
	return nil
}

type pluginLineWriter struct {
	mu     sync.Mutex
	buffer string
	emit   func(string)
}

func (w *pluginLineWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buffer += string(data)
	for {
		index := strings.IndexByte(w.buffer, '\n')
		if index < 0 {
			break
		}
		line := strings.TrimRight(w.buffer[:index], "\r")
		w.buffer = w.buffer[index+1:]
		if strings.TrimSpace(line) != "" {
			w.emit(line)
		}
	}
	return len(data), nil
}

func (w *pluginLineWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if line := strings.TrimSpace(w.buffer); line != "" {
		w.emit(line)
	}
	w.buffer = ""
}

func (l *Live) loadPluginUpdates() {
	if l.dataDir == "" {
		return
	}
	raw, err := os.ReadFile(filepath.Join(l.dataDir, "plugin-updates.json"))
	if err != nil {
		return
	}
	var state pluginUpdateState
	if json.Unmarshal(raw, &state) != nil || state.Plugins == nil {
		return
	}
	l.pluginMu.Lock()
	l.pluginUpdates = state.Plugins
	l.pluginCheckedAt = state.CheckedAt
	l.pluginMu.Unlock()
}

func (l *Live) savePluginUpdates() {
	if l.dataDir == "" {
		return
	}
	l.pluginMu.Lock()
	state := pluginUpdateState{CheckedAt: l.pluginCheckedAt, Plugins: make(map[string]pluginUpdate, len(l.pluginUpdates))}
	for name, update := range l.pluginUpdates {
		state.Plugins[name] = update
	}
	l.pluginMu.Unlock()
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(l.dataDir, 0o750); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(l.dataDir, "plugin-updates.json"), raw, 0o600)
}
