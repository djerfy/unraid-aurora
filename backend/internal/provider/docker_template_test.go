package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurora/internal/model"
)

const sampleTemplate = `<?xml version="1.0"?>
<Container version="2">
  <Name>plex</Name>
  <Repository>linuxserver/plex:latest</Repository>
  <Registry>https://registry-1.docker.io</Registry>
  <Network>bridge</Network>
  <Shell>sh</Shell>
  <Privileged>false</Privileged>
  <Overview>Media server</Overview>
  <Category>MediaServer</Category>
  <WebUI>http://[IP]:[PORT:32400]/web</WebUI>
  <Icon>https://example.test/plex.png</Icon>
  <ExtraParams>--device=/dev/dri</ExtraParams>
  <PostArgs></PostArgs>
  <DateInstalled>1700000000</DateInstalled>
  <Config Name="Config" Target="/config" Default="" Mode="rw" Description="" Type="Path" Display="always" Required="false" Mask="">/mnt/user/appdata/plex</Config>
  <Config Name="WebUI" Target="32400" Default="" Mode="tcp" Description="" Type="Port" Display="always" Required="false" Mask="">32400</Config>
  <Config Name="Claim" Target="PLEX_CLAIM" Default="" Mode="" Description="" Type="Variable" Display="always" Required="false" Mask="">claim-1</Config>
  <Config Name="Legacy" Target="OLD_FLAG" Default="" Mode="" Description="" Type="Variable" Display="always" Required="false" Mask="">1</Config>
  <Config Name="Label" Target="traefik.enable" Default="" Mode="" Description="" Type="Label" Display="always" Required="false" Mask="">true</Config>
  <Config Name="DRI" Target="" Default="" Mode="" Description="" Type="Device" Display="always" Required="false" Mask="">/dev/dri</Config>
</Container>
`

func TestSyncContainerTemplateUpdatesExistingFile(t *testing.T) {
	boot := t.TempDir()
	dir := filepath.Join(boot, userTemplateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "my-plex.xml")
	if err := os.WriteFile(path, []byte(sampleTemplate), 0o644); err != nil {
		t.Fatal(err)
	}

	live := &Live{bootDir: boot}
	err := live.syncContainerTemplate(context.Background(), containerTemplateData{
		Name:          "plex",
		Image:         "linuxserver/plex:1.40",
		Network:       "bridge",
		RestartPolicy: "unless-stopped",
		Volumes: []model.ContainerVolume{
			{Source: "/mnt/user/appdata/plex-new", Destination: "/config", ReadOnly: true},
			{Source: "/mnt/user/media", Destination: "/media"},
		},
		Ports: []model.Port{
			{Host: 32400, Container: 32400, Protocol: "tcp"},
			{Host: 1900, Container: 1900, Protocol: "udp"},
		},
		Env: []model.ContainerEnv{
			{Key: "PLEX_CLAIM", Value: "claim-2"},
			{Key: "PUID", Value: "99"},
			{Key: "HOST_OS", Value: "Unraid"},
		},
		PreserveMeta: true,
	})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parseXMLTree(raw)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}

	if root.text("Repository") != "linuxserver/plex:1.40" {
		t.Errorf("repository = %q", root.text("Repository"))
	}
	if root.text("Overview") != "Media server" || root.text("Category") != "MediaServer" {
		t.Error("template metadata was not preserved")
	}
	if root.text("Icon") != "https://example.test/plex.png" || root.text("WebUI") != "http://[IP]:[PORT:32400]/web" {
		t.Error("icon and webui should be preserved")
	}
	if root.text("ExtraParams") != "--device=/dev/dri --restart=unless-stopped" || root.text("DateInstalled") != "1700000000" {
		t.Errorf("extra params and install date: %q / %q", root.text("ExtraParams"), root.text("DateInstalled"))
	}
	if root.text("RestartPolicy") != "unless-stopped" {
		t.Errorf("restart policy = %q", root.text("RestartPolicy"))
	}

	configs := templateConfigs(root)
	configPath, ok := configs["Path:/config"]
	if !ok || configPath.Text != "/mnt/user/appdata/plex-new" || configPath.attribute("Mode") != "ro" {
		t.Errorf("config path not updated: %+v", configPath)
	}
	if _, ok := configs["Path:/media"]; !ok {
		t.Error("new path was not added")
	}
	if port, ok := configs["Port:32400"]; !ok || port.Text != "32400" {
		t.Errorf("port not updated: %+v", port)
	}
	if _, ok := configs["Port:1900"]; !ok {
		t.Error("new port was not added")
	}
	if claim, ok := configs["Variable:PLEX_CLAIM"]; !ok || claim.Text != "claim-2" {
		t.Errorf("variable not updated: %+v", claim)
	}
	if _, ok := configs["Variable:PUID"]; !ok {
		t.Error("new variable was not added")
	}
	if _, ok := configs["Variable:OLD_FLAG"]; ok {
		t.Error("removed variable should be deleted")
	}
	if _, ok := configs["Variable:HOST_OS"]; ok {
		t.Error("Unraid injected variables must not be written")
	}
	if _, ok := configs["Label:traefik.enable"]; !ok {
		t.Error("label config should be preserved")
	}
	if _, ok := configs["Device:"]; !ok {
		t.Error("device config should be preserved")
	}
}

func TestSyncContainerTemplateCreatesFile(t *testing.T) {
	boot := t.TempDir()
	live := &Live{bootDir: boot}
	err := live.syncContainerTemplate(context.Background(), containerTemplateData{
		Name:          "sonarr",
		Image:         "linuxserver/sonarr:latest",
		Network:       "bridge",
		RestartPolicy: "unless-stopped",
		Ports:         []model.Port{{Host: 8989, Container: 8989, Protocol: "tcp"}},
		Volumes:       []model.ContainerVolume{{Source: "/mnt/user/appdata/sonarr", Destination: "/config"}},
		Env:           []model.ContainerEnv{{Key: "PUID", Value: "99"}},
		Icon:          "https://example.test/sonarr.png",
		WebUI:         "http://[IP]:[PORT:8989]",
		Privileged:    true,
	})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	path := filepath.Join(boot, userTemplateDir, "my-sonarr.xml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("template file: %v", err)
	}
	if !strings.HasPrefix(string(raw), "<?xml") {
		t.Error("template should start with an XML declaration")
	}
	root, err := parseXMLTree(raw)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if root.text("Name") != "sonarr" || root.text("Repository") != "linuxserver/sonarr:latest" {
		t.Errorf("identity = %q / %q", root.text("Name"), root.text("Repository"))
	}
	if root.text("Network") != "bridge" || root.text("Privileged") != "true" {
		t.Errorf("network/privileged = %q / %q", root.text("Network"), root.text("Privileged"))
	}
	if root.text("Icon") == "" || root.text("WebUI") == "" {
		t.Error("icon and webui should be written for new templates")
	}
	configs := templateConfigs(root)
	if _, ok := configs["Path:/config"]; !ok {
		t.Error("path config missing")
	}
	if _, ok := configs["Port:8989"]; !ok {
		t.Error("port config missing")
	}
	if _, ok := configs["Variable:PUID"]; !ok {
		t.Error("variable config missing")
	}
}

func templateConfigs(root *xmlElement) map[string]*xmlElement {
	out := map[string]*xmlElement{}
	for _, child := range root.Children {
		if child.Name.Local != "Config" {
			continue
		}
		out[child.attribute("Type")+":"+child.attribute("Target")] = child
	}
	return out
}

func TestTemplateFromXML(t *testing.T) {
	root, err := parseXMLTree([]byte(sampleTemplate))
	if err != nil {
		t.Fatal(err)
	}
	template := templateFromXML(root)
	if template.Name != "plex" || template.Image != "linuxserver/plex:latest" || template.Network != "bridge" {
		t.Fatalf("template = %+v", template)
	}
	if !template.Privileged && template.Icon == "" {
		t.Fatalf("metadata = %+v", template)
	}
	if len(template.Volumes) != 1 || template.Volumes[0].Destination != "/config" {
		t.Fatalf("volumes = %+v", template.Volumes)
	}
	if len(template.Ports) != 1 || template.Ports[0].Host != 32400 {
		t.Fatalf("ports = %+v", template.Ports)
	}
	if len(template.Env) != 2 {
		t.Fatalf("env = %+v", template.Env)
	}
}

func TestContainerTemplatesListsUserTemplates(t *testing.T) {
	boot := t.TempDir()
	dir := filepath.Join(boot, userTemplateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "my-plex.xml"), []byte(sampleTemplate), 0o644); err != nil {
		t.Fatal(err)
	}
	live := &Live{bootDir: boot}
	templates, err := live.ContainerTemplates(context.Background())
	if err != nil {
		t.Fatalf("templates: %v", err)
	}
	if len(templates) != 1 || templates[0].Name != "plex" || templates[0].ID == "" {
		t.Fatalf("templates = %+v", templates)
	}
}

func TestSyncContainerTemplatePreservesExtraParams(t *testing.T) {
	boot := t.TempDir()
	dir := filepath.Join(boot, userTemplateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "my-plex.xml")
	if err := os.WriteFile(path, []byte(sampleTemplate), 0o644); err != nil {
		t.Fatal(err)
	}

	live := &Live{bootDir: boot}
	err := live.syncContainerTemplate(context.Background(), containerTemplateData{
		Name:          "plex",
		Image:         "linuxserver/plex:latest",
		Network:       "bridge",
		RestartPolicy: "unless-stopped",
		AdvancedSet:   true,
		Advanced: model.ContainerAdvanced{
			Labels:     map[string]string{"build-date": "20261022", "vendor": "VMware"},
			Hostname:   "94b32b702031",
			MacAddress: "7e:0c:eb:09:3c:68",
			PidsLimit:  2048,
		},
		PreserveMeta:        true,
		PreserveExtraParams: true,
	})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parseXMLTree(raw)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if got := root.text("ExtraParams"); got != "--device=/dev/dri --restart=unless-stopped" {
		t.Fatalf("image labels leaked into ExtraParams: %q", got)
	}
}

func TestSyncTemplateAfterUpdateKeepsExtraParams(t *testing.T) {
	boot := t.TempDir()
	dir := filepath.Join(boot, userTemplateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "my-plex.xml")
	if err := os.WriteFile(path, []byte(sampleTemplate), 0o644); err != nil {
		t.Fatal(err)
	}

	live := &Live{bootDir: boot}
	live.syncTemplateAfterUpdate(context.Background(), model.ContainerConfig{
		Name:          "plex",
		Image:         "linuxserver/plex:latest",
		NetworkMode:   "bridge",
		RestartPolicy: "unless-stopped",
		Advanced: model.ContainerAdvanced{
			Labels:     map[string]string{"build-date": "20261022"},
			Hostname:   "94b32b702031",
			MacAddress: "7e:0c:eb:09:3c:68",
			PidsLimit:  2048,
		},
	}, false)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parseXMLTree(raw)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if got := root.text("ExtraParams"); got != "--device=/dev/dri --restart=unless-stopped" {
		t.Fatalf("template sync leaked Docker defaults: %q", got)
	}
}

func TestContainerConfigLoadsRawTemplateFields(t *testing.T) {
	boot := t.TempDir()
	dir := filepath.Join(boot, userTemplateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	template := strings.Replace(sampleTemplate, `<PostArgs></PostArgs>`, `<PostArgs>--old value</PostArgs>`, 1)
	template = strings.Replace(template, `<ExtraParams>--device=/dev/dri</ExtraParams>`, `<ExtraParams>--unknown=keep</ExtraParams>`, 1)
	if err := os.WriteFile(filepath.Join(dir, "my-plex.xml"), []byte(template), 0o644); err != nil {
		t.Fatal(err)
	}

	live, _ := newFakeLive()
	live.bootDir = boot
	cfg, err := live.ContainerConfig(context.Background(), "old-container")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Advanced.ExtraParams != "--unknown=keep" || cfg.Advanced.PostArgs != "--old value" {
		t.Fatalf("raw template fields were not loaded: %+v", cfg.Advanced)
	}
	if !cfg.Privileged {
		t.Fatal("privileged flag was not read from Docker inspect")
	}
}

func TestSyncContainerTemplateWritesExtraParamsAndPostArgs(t *testing.T) {
	boot := t.TempDir()
	dir := filepath.Join(boot, userTemplateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "my-plex.xml")
	if err := os.WriteFile(path, []byte(sampleTemplate), 0o644); err != nil {
		t.Fatal(err)
	}
	live := &Live{bootDir: boot}
	err := live.syncContainerTemplate(context.Background(), containerTemplateData{
		Name:          "plex",
		Image:         "linuxserver/plex:latest",
		Network:       "bridge",
		RestartPolicy: "unless-stopped",
		PreserveMeta:  true,
		AdvancedSet:   true,
		Advanced: model.ContainerAdvanced{
			ExtraParams: "--unknown=keep --pids-limit=512",
			PostArgs:    "--new value",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parseXMLTree(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := root.text("ExtraParams"); !strings.Contains(got, "--unknown=keep") || !strings.Contains(got, "--pids-limit=512") {
		t.Fatalf("extra params = %q", got)
	}
	if got := root.text("PostArgs"); got != "--new value" {
		t.Fatalf("post args = %q", got)
	}
}

func TestSyncTemplateAfterUpdateWritesPrivileged(t *testing.T) {
	boot := t.TempDir()
	dir := filepath.Join(boot, userTemplateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "my-plex.xml")
	if err := os.WriteFile(path, []byte(sampleTemplate), 0o644); err != nil {
		t.Fatal(err)
	}
	live := &Live{bootDir: boot}
	live.syncTemplateAfterUpdate(context.Background(), model.ContainerConfig{
		Name:          "plex",
		Image:         "linuxserver/plex:latest",
		NetworkMode:   "bridge",
		RestartPolicy: "unless-stopped",
		Privileged:    true,
		Advanced:      model.ContainerAdvanced{ExtraParams: "--unknown=keep", PostArgs: "--new"},
	}, true)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parseXMLTree(raw)
	if err != nil {
		t.Fatal(err)
	}
	if root.text("Privileged") != "true" {
		t.Fatalf("privileged = %q", root.text("Privileged"))
	}
	if root.text("PostArgs") != "--new" || !strings.Contains(root.text("ExtraParams"), "--unknown=keep") {
		t.Fatalf("raw fields not written: %q / %q", root.text("ExtraParams"), root.text("PostArgs"))
	}
}
