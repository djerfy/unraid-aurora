package provider

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const maxIconBytes = 4 << 20

// ContainerIcon returns the icon Unraid cached for a container. The original
// webGUI stores it under /var/lib/docker/unraid/images and falls back to the
// user template's <Icon> value.
func (l *Live) ContainerIcon(ctx context.Context, id string) ([]byte, string, error) {
	containers, err := l.Containers(ctx)
	if err != nil {
		return nil, "", err
	}
	var name, configured string
	for _, container := range containers {
		if container.ID == id || strings.EqualFold(container.Name, id) {
			name, configured = container.Name, container.Icon
			break
		}
	}
	if name == "" {
		return nil, "", fmt.Errorf("container %q not found", id)
	}
	name = filepath.Base(name)

	candidates := []string{
		filepath.Join("/var/lib/docker/unraid/images", name+"-icon.png"),
		filepath.Join("/usr/local/emhttp/state/plugins/dynamix.docker.manager/images", name+"-icon.png"),
		filepath.Join("/var/local/emhttp/plugins/dynamix.docker.manager/images", name+"-icon.png"),
	}
	if icon := l.templateIcon(name); icon != "" {
		candidates = append([]string{icon}, candidates...)
	}
	if isLocalIconPath(configured) {
		candidates = append([]string{configured}, candidates...)
	}
	return readFirstIcon(candidates...)
}

// UserIcon reads the optional avatar stored by the Unraid user editor.
func (l *Live) UserIcon(_ context.Context, name string) ([]byte, string, error) {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == ".." {
		return nil, "", errors.New("invalid user name")
	}
	return readFirstIcon(filepath.Join(l.bootDir, "config/plugins/dynamix/users", name+".png"))
}

// VMIcon resolves the icon declared in the domain metadata, then the stock
// Unraid OS icons.
func (l *Live) VMIcon(ctx context.Context, id string) ([]byte, string, error) {
	vms, err := l.VMs(ctx)
	if err != nil {
		return nil, "", err
	}
	var name, osName, icon string
	for _, vm := range vms {
		if vm.ID == id || strings.EqualFold(vm.Name, id) {
			name, osName, icon = vm.Name, vm.OS, vm.Icon
			break
		}
	}
	if name == "" {
		return nil, "", fmt.Errorf("virtual machine %q not found", id)
	}

	if xmlOut, err := execCommand(ctx, "virsh", "dumpxml", name); err == nil {
		if declared := iconFromDomainXML(xmlOut); declared != "" {
			icon = declared
		}
	}

	base := filepath.Base(strings.TrimSpace(icon))
	if base == "" || base == "." || base == "/" {
		base = defaultVMIcon(osName)
	}
	candidates := []string{}
	if filepath.IsAbs(icon) {
		candidates = append(candidates, icon)
	}
	candidates = append(candidates,
		filepath.Join("/usr/local/emhttp/plugins/dynamix.vm.manager/templates/images", base),
		filepath.Join(l.bootDir, "config/plugins/dynamix.vm.manager/templates/images", base),
		filepath.Join("/usr/local/emhttp/plugins/dynamix.vm.manager/templates/images", defaultVMIcon(osName)),
		filepath.Join(l.bootDir, "config/plugins/dynamix.vm.manager/templates/images", defaultVMIcon(osName)),
	)
	return readFirstIcon(candidates...)
}

func (l *Live) templateIcon(name string) string {
	path := filepath.Join(l.bootDir, "config/plugins/dockerMan/templates-user", name+".xml")
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	icon := iconElementText(raw)
	if icon == "" {
		return ""
	}
	if filepath.IsAbs(icon) {
		return icon
	}
	if strings.HasPrefix(icon, "/plugins/") || strings.HasPrefix(icon, "/webGui/") {
		return filepath.Join("/usr/local/emhttp", icon)
	}
	return ""
}

func readFirstIcon(paths ...string) ([]byte, string, error) {
	for _, path := range paths {
		if !isLocalIconPath(path) {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Size() == 0 || info.Size() > maxIconBytes {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		return data, iconContentType(path), nil
	}
	return nil, "", errors.New("no icon available")
}

func isLocalIconPath(path string) bool {
	if path == "" || strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return false
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
		return true
	}
	return false
}

func iconContentType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".svg":
		return "image/svg+xml"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	}
	return "image/png"
}

func iconElementText(raw []byte) string {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	inIcon := false
	var value strings.Builder
	for {
		token, err := dec.Token()
		if err != nil {
			return ""
		}
		switch token := token.(type) {
		case xml.StartElement:
			if strings.EqualFold(token.Name.Local, "Icon") {
				inIcon = true
				value.Reset()
			}
		case xml.EndElement:
			if inIcon && strings.EqualFold(token.Name.Local, "Icon") {
				return strings.TrimSpace(value.String())
			}
		case xml.CharData:
			if inIcon {
				value.Write(token)
			}
		}
	}
}

func iconFromDomainXML(raw []byte) string {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	for {
		token, err := dec.Token()
		if err != nil {
			return ""
		}
		if element, ok := token.(xml.StartElement); ok {
			for _, attr := range element.Attr {
				if strings.EqualFold(attr.Name.Local, "icon") {
					return attr.Value
				}
			}
		}
	}
}

func defaultVMIcon(osName string) string {
	lower := strings.ToLower(osName)
	switch {
	case strings.Contains(lower, "windows 11"):
		return "windows11.png"
	case strings.Contains(lower, "windows"):
		return "windows.png"
	case strings.Contains(lower, "ubuntu"):
		return "ubuntu.png"
	case strings.Contains(lower, "debian"):
		return "debian.png"
	case strings.Contains(lower, "fedora"):
		return "fedora.png"
	case strings.Contains(lower, "arch"):
		return "arch.png"
	case strings.Contains(lower, "unraid"):
		return "unraid.png"
	case strings.Contains(lower, "linux"):
		return "linux.png"
	default:
		return "default.png"
	}
}

func execCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	// #nosec G204 -- callers pass fixed system binaries; VM names are validated.
	return exec.CommandContext(ctx, name, args...).Output()
}
