package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"aurora/internal/model"
)

// Unraid keeps one XML template per container in templates-user. The official
// webGUI writes it before creating the container, and its update scripts read
// it back, so Aurora mirrors the same structure to keep both UIs in sync.
const userTemplateDir = "config/plugins/dockerMan/templates-user"

// Environment variables injected by Unraid at create time; they must not be
// duplicated as user variables in the template.
var unraidInjectedEnv = map[string]bool{
	"HOST_OS":            true,
	"HOST_HOSTNAME":      true,
	"HOST_CONTAINERNAME": true,
}

type containerTemplateData struct {
	Name          string
	Image         string
	Network       string
	RestartPolicy string
	Ports         []model.Port
	Volumes       []model.ContainerVolume
	Env           []model.ContainerEnv
	Icon          string
	WebUI         string
	Privileged    bool
	// PreserveMeta keeps the icon, WebUI and privilege flag from the existing
	// template when the caller cannot know them (container settings edit).
	PreserveMeta bool
}

/* -------------------------------------------------------------------------- */
/* Minimal XML tree                                                            */
/* -------------------------------------------------------------------------- */

type xmlElement struct {
	Name     xml.Name
	Attrs    []xml.Attr
	Children []*xmlElement
	Text     string
}

func parseXMLTree(raw []byte) (*xmlElement, error) {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	var root *xmlElement
	var stack []*xmlElement
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			element := &xmlElement{Name: value.Name, Attrs: append([]xml.Attr(nil), value.Attr...)}
			if len(stack) == 0 {
				root = element
			} else {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, element)
			}
			stack = append(stack, element)
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].Text += string(value)
			}
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if root == nil {
		return nil, errors.New("template is empty")
	}
	return root, nil
}

func (e *xmlElement) encode(encoder *xml.Encoder) error {
	start := xml.StartElement{Name: e.Name, Attr: e.Attrs}
	if len(e.Children) == 0 {
		return encoder.EncodeElement(strings.TrimSpace(e.Text), start)
	}
	if err := encoder.EncodeToken(start); err != nil {
		return err
	}
	for _, child := range e.Children {
		if err := child.encode(encoder); err != nil {
			return err
		}
	}
	return encoder.EncodeToken(start.End())
}

func (e *xmlElement) child(name string) *xmlElement {
	for _, child := range e.Children {
		if child.Name.Local == name {
			return child
		}
	}
	return nil
}

func (e *xmlElement) ensureChild(name string) *xmlElement {
	if child := e.child(name); child != nil {
		return child
	}
	child := &xmlElement{Name: xml.Name{Local: name}}
	e.Children = append(e.Children, child)
	return child
}

func (e *xmlElement) setText(name, value string) {
	e.ensureChild(name).Text = value
}

func (e *xmlElement) text(name string) string {
	if child := e.child(name); child != nil {
		return strings.TrimSpace(child.Text)
	}
	return ""
}

func (e *xmlElement) attribute(name string) string {
	for _, attr := range e.Attrs {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func (e *xmlElement) setAttribute(name, value string) {
	for index := range e.Attrs {
		if e.Attrs[index].Name.Local == name {
			e.Attrs[index].Value = value
			return
		}
	}
	e.Attrs = append(e.Attrs, xml.Attr{Name: xml.Name{Local: name}, Value: value})
}

func (e *xmlElement) setVersion(value string) {
	for index := range e.Attrs {
		if e.Attrs[index].Name.Local == "version" {
			e.Attrs[index].Value = value
			return
		}
	}
	e.Attrs = append([]xml.Attr{{Name: xml.Name{Local: "version"}, Value: value}}, e.Attrs...)
}

// ContainerTemplates lists the XML templates available on the flash drive so a
// new container can be recreated from an existing definition.
func (l *Live) ContainerTemplates(_ context.Context) ([]model.ContainerTemplate, error) {
	if l.bootDir == "" {
		return []model.ContainerTemplate{}, nil
	}
	directories := []string{
		filepath.Join(l.bootDir, userTemplateDir),
		filepath.Join(l.bootDir, "config/plugins/dockerMan/templates"),
	}
	seen := map[string]bool{}
	templates := make([]model.ContainerTemplate, 0)
	for _, directory := range directories {
		entries, err := os.ReadDir(directory)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".xml") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(directory, entry.Name()))
			if err != nil {
				slog.Warn("read container template", "file", entry.Name(), "error", err)
				continue
			}
			root, err := parseXMLTree(raw)
			if err != nil {
				slog.Warn("parse container template", "file", entry.Name(), "error", err)
				continue
			}
			template := templateFromXML(root)
			if template.Name == "" {
				continue
			}
			key := strings.ToLower(template.Name)
			if seen[key] {
				continue
			}
			seen[key] = true
			template.ID = filepath.Base(directory) + ":" + entry.Name()
			if info, err := entry.Info(); err == nil {
				template.Modified = info.ModTime()
			}
			templates = append(templates, template)
		}
	}
	sort.Slice(templates, func(i, j int) bool { return templates[i].Name < templates[j].Name })
	return templates, nil
}

func templateFromXML(root *xmlElement) model.ContainerTemplate {
	template := model.ContainerTemplate{
		Name:          root.text("Name"),
		Image:         root.text("Repository"),
		Network:       root.text("Network"),
		RestartPolicy: templateRestartPolicy(root),
		Icon:          root.text("Icon"),
		WebUI:         root.text("WebUI"),
		Privileged:    strings.EqualFold(root.text("Privileged"), "true"),
		Ports:         []model.Port{},
		Volumes:       []model.ContainerVolume{},
		Env:           []model.ContainerEnv{},
	}
	for _, node := range root.Children {
		if node.Name.Local != "Config" {
			continue
		}
		target := node.attribute("Target")
		value := strings.TrimSpace(node.Text)
		switch strings.ToLower(node.attribute("Type")) {
		case "path":
			if target == "" || value == "" {
				continue
			}
			template.Volumes = append(template.Volumes, model.ContainerVolume{
				Source:      value,
				Destination: target,
				ReadOnly:    strings.EqualFold(node.attribute("Mode"), "ro"),
			})
		case "port":
			containerPort := atoi(target)
			hostPort := atoi(value)
			if containerPort < 1 || hostPort < 1 {
				continue
			}
			protocol := node.attribute("Mode")
			if protocol == "" {
				protocol = "tcp"
			}
			template.Ports = append(template.Ports, model.Port{Host: hostPort, Container: containerPort, Protocol: protocol})
		case "variable":
			if target == "" || unraidInjectedEnv[target] {
				continue
			}
			template.Env = append(template.Env, model.ContainerEnv{Key: target, Value: value})
		}
	}
	return template
}

func templateRestartPolicy(root *xmlElement) string {
	if policy := root.text("RestartPolicy"); policy != "" {
		return policy
	}
	match := restartParamPattern.FindStringSubmatch(root.text("ExtraParams"))
	if len(match) >= 4 {
		return strings.TrimSpace(match[3])
	}
	return ""
}

/* -------------------------------------------------------------------------- */
/* Template synchronisation                                                    */
/* -------------------------------------------------------------------------- */

func (l *Live) syncContainerTemplate(ctx context.Context, data containerTemplateData) error {
	if l.bootDir == "" {
		return errors.New("boot directory is not configured")
	}
	path, exists := l.userTemplatePath(data.Name)
	var root *xmlElement
	if exists {
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read template: %w", err)
		}
		root, err = parseXMLTree(raw)
		if err != nil {
			return fmt.Errorf("parse template: %w", err)
		}
	} else {
		root = &xmlElement{Name: xml.Name{Local: "Container"}}
		root.setVersion("2")
		seedTemplate(root, data.Name)
	}

	root.setVersion("2")
	root.setText("Name", data.Name)
	root.setText("Repository", data.Image)
	if data.Network != "" {
		root.setText("Network", data.Network)
	} else if root.text("Network") == "" {
		root.setText("Network", "bridge")
	}
	root.setText("RestartPolicy", data.RestartPolicy)
	// Unraid has no dedicated restart policy field; keeping the flag in
	// ExtraParams makes the setting survive a recreation from its own webGUI.
	root.setText("ExtraParams", applyRestartPolicy(root.text("ExtraParams"), data.RestartPolicy))
	if !data.PreserveMeta {
		root.setText("Privileged", strconv.FormatBool(data.Privileged))
		root.setText("Icon", data.Icon)
		root.setText("WebUI", data.WebUI)
	}
	if root.text("Shell") == "" {
		root.setText("Shell", "sh")
	}
	if root.text("DateInstalled") == "" {
		root.setText("DateInstalled", strconv.FormatInt(time.Now().Unix(), 10))
	}

	bridgeLike := l.isBridgeNetwork(ctx, data.Network)
	syncTemplateConfigs(root, data, bridgeLike)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create template directory: %w", err)
	}
	var buffer bytes.Buffer
	buffer.WriteString(xml.Header)
	encoder := xml.NewEncoder(&buffer)
	encoder.Indent("", "  ")
	if err := root.encode(encoder); err != nil {
		return fmt.Errorf("encode template: %w", err)
	}
	if err := encoder.Flush(); err != nil {
		return fmt.Errorf("encode template: %w", err)
	}
	buffer.WriteString("\n")
	if err := os.WriteFile(path, buffer.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write template: %w", err)
	}
	return nil
}

func (l *Live) userTemplatePath(name string) (string, bool) {
	dir := filepath.Join(l.bootDir, userTemplateDir)
	target := "my-" + strings.NewReplacer("/", "", "\\", "").Replace(name) + ".xml"
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if strings.EqualFold(entry.Name(), target) {
				return filepath.Join(dir, entry.Name()), true
			}
		}
	}
	return filepath.Join(dir, target), false
}

// seedTemplate writes the metadata elements the Unraid UI expects, in the same
// shape as a template saved by the Add Container page.
func seedTemplate(root *xmlElement, name string) {
	root.setText("Name", name)
	for _, field := range []string{
		"Repository", "Registry", "Network", "ExtraNetworks", "MyIP", "MyMAC",
		"Shell", "Privileged", "Support", "Project", "ReadMe", "Overview",
		"Category", "WebUI", "TemplateURL", "Icon", "ExtraParams", "PostArgs",
		"CPUset", "Memory", "DateInstalled", "DonateText", "DonateLink",
		"Requires", "RestartPolicy",
	} {
		root.ensureChild(field)
	}
}

func syncTemplateConfigs(root *xmlElement, data containerTemplateData, bridgeLike bool) {
	existing := make([]*xmlElement, 0)
	for _, child := range root.Children {
		if child.Name.Local == "Config" {
			existing = append(existing, child)
		}
	}

	usedPaths := map[string]bool{}
	usedPorts := map[string]bool{}
	usedEnv := map[string]bool{}

	for _, node := range existing {
		switch strings.ToLower(node.attribute("Type")) {
		case "path":
			target := node.attribute("Target")
			volume, ok := findVolume(data.Volumes, target)
			if !ok {
				root.Children = removeElement(root.Children, node)
				continue
			}
			node.Text = volume.Source
			node.setAttribute("Mode", readOnlyMode(volume.ReadOnly))
			usedPaths[target] = true
		case "port":
			target := node.attribute("Target")
			protocol := node.attribute("Mode")
			port, ok := findPort(data.Ports, target, protocol)
			if !ok {
				// Ports on host/macvlan networks are rendered as variables by
				// Unraid and cannot be inferred from Docker port bindings.
				if bridgeLike {
					root.Children = removeElement(root.Children, node)
				}
				continue
			}
			node.Text = strconv.Itoa(port.Host)
			usedPorts[portKey(target, protocol)] = true
		case "variable":
			key := node.attribute("Target")
			entry, ok := findEnv(data.Env, key)
			if !ok {
				if !unraidInjectedEnv[key] {
					root.Children = removeElement(root.Children, node)
				}
				continue
			}
			node.Text = entry.Value
			usedEnv[key] = true
		}
	}

	for _, volume := range data.Volumes {
		if usedPaths[volume.Destination] {
			continue
		}
		root.Children = append(root.Children, newConfigElement(map[string]string{
			"Name":        volume.Destination,
			"Target":      volume.Destination,
			"Default":     "",
			"Mode":        readOnlyMode(volume.ReadOnly),
			"Description": "",
			"Type":        "Path",
			"Display":     "always",
			"Required":    "false",
			"Mask":        "",
		}, volume.Source))
	}

	if bridgeLike {
		for _, port := range data.Ports {
			if usedPorts[portKey(strconv.Itoa(port.Container), port.Protocol)] {
				continue
			}
			root.Children = append(root.Children, newConfigElement(map[string]string{
				"Name":        fmt.Sprintf("%d/%s", port.Container, port.Protocol),
				"Target":      strconv.Itoa(port.Container),
				"Default":     "",
				"Mode":        port.Protocol,
				"Description": "",
				"Type":        "Port",
				"Display":     "always",
				"Required":    "false",
				"Mask":        "",
			}, strconv.Itoa(port.Host)))
		}
	}

	for _, entry := range data.Env {
		if usedEnv[entry.Key] || unraidInjectedEnv[entry.Key] {
			continue
		}
		root.Children = append(root.Children, newConfigElement(map[string]string{
			"Name":        entry.Key,
			"Target":      entry.Key,
			"Default":     "",
			"Mode":        "",
			"Description": "",
			"Type":        "Variable",
			"Display":     "always",
			"Required":    "false",
			"Mask":        "",
		}, entry.Value))
	}
}

func newConfigElement(attributes map[string]string, value string) *xmlElement {
	element := &xmlElement{Name: xml.Name{Local: "Config"}, Text: value}
	for _, key := range []string{"Name", "Target", "Default", "Mode", "Description", "Type", "Display", "Required", "Mask"} {
		element.Attrs = append(element.Attrs, xml.Attr{Name: xml.Name{Local: key}, Value: attributes[key]})
	}
	return element
}

func removeElement(children []*xmlElement, target *xmlElement) []*xmlElement {
	kept := children[:0]
	for _, child := range children {
		if child != target {
			kept = append(kept, child)
		}
	}
	return kept
}

func findVolume(volumes []model.ContainerVolume, destination string) (model.ContainerVolume, bool) {
	for _, volume := range volumes {
		if volume.Destination == destination {
			return volume, true
		}
	}
	return model.ContainerVolume{}, false
}

func findPort(ports []model.Port, target, protocol string) (model.Port, bool) {
	containerPort, err := strconv.Atoi(target)
	if err != nil {
		return model.Port{}, false
	}
	if protocol == "" {
		protocol = "tcp"
	}
	for _, port := range ports {
		if port.Container == containerPort && strings.EqualFold(port.Protocol, protocol) {
			return port, true
		}
	}
	return model.Port{}, false
}

func findEnv(env []model.ContainerEnv, key string) (model.ContainerEnv, bool) {
	for _, entry := range env {
		if entry.Key == key {
			return entry, true
		}
	}
	return model.ContainerEnv{}, false
}

var restartParamPattern = regexp.MustCompile(`(^|\s)--restart(=\s*|\s+)\S+`)

func applyRestartPolicy(extraParams, policy string) string {
	cleaned := restartParamPattern.ReplaceAllString(extraParams, "")
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	if policy == "" || policy == "no" {
		return cleaned
	}
	if cleaned == "" {
		return "--restart=" + policy
	}
	return cleaned + " --restart=" + policy
}

func readOnlyMode(readOnly bool) string {
	if readOnly {
		return "ro"
	}
	return "rw"
}

func portKey(target, protocol string) string {
	return strings.ToLower(target + "/" + protocol)
}

// isBridgeNetwork reports whether published ports are real Docker bindings for
// this network mode. Unknown or unavailable drivers are treated conservatively.
func (l *Live) isBridgeNetwork(ctx context.Context, network string) bool {
	switch network {
	case "", "bridge":
		return true
	case "host", "none":
		return false
	}
	if l.client == nil || strings.HasPrefix(network, "container:") {
		return false
	}
	raw, err := l.dockerDo(ctx, http.MethodGet, "/networks/"+network, nil)
	if err != nil {
		return false
	}
	var payload struct {
		Driver string `json:"Driver"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	return payload.Driver == "bridge"
}

// syncTemplateAfterUpdate is best-effort: the container has already been
// recreated, so a template write failure must not mask a successful update.
func (l *Live) syncTemplateAfterUpdate(ctx context.Context, cfg model.ContainerConfig) {
	if l.bootDir == "" {
		return
	}
	// Only touch templates Unraid already manages; creating one for an orphan
	// container would surface it as a managed app in the official webGUI.
	if _, exists := l.userTemplatePath(cfg.Name); !exists {
		return
	}
	err := l.syncContainerTemplate(ctx, containerTemplateData{
		Name:          cfg.Name,
		Image:         cfg.Image,
		Network:       cfg.NetworkMode,
		RestartPolicy: cfg.RestartPolicy,
		Ports:         cfg.Ports,
		Volumes:       cfg.Volumes,
		Env:           cfg.Env,
		PreserveMeta:  true,
	})
	if err != nil {
		slog.Error("sync container template", "container", cfg.Name, "error", err)
	}
}
