package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurora/internal/model"
)

func TestDockerSettingsRoundTrip(t *testing.T) {
	boot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(boot, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(boot, "config", "docker.cfg")
	initial := `DOCKER_ENABLED="yes"
DOCKER_IMAGE_TYPE="folder"
DOCKER_IMAGE_FILE="/mnt/user/system/docker/"
DOCKER_IMAGE_SIZE="20"
DOCKER_APP_CONFIG_PATH="/mnt/user/appdata/"
DOCKER_TIMEOUT="10"
DOCKER_LOG_ROTATION="yes"
DOCKER_LOG_SIZE="50m"
DOCKER_LOG_FILES="1"
DOCKER_UNKNOWN="keep"
`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	live, _ := newFakeLive()
	live.bootDir = boot

	settings, err := live.DockerSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !settings.Enabled || settings.ImageType != "folder" || settings.ImageFile != "/mnt/user/system/docker/" || settings.LogSize != "50m" {
		t.Fatalf("unexpected settings: %+v", settings)
	}

	settings.Enabled = false
	settings.StopTimeout = 20
	settings.LogSize = "100m"
	settings.PreserveUserNetworks = true
	if err := live.SaveDockerSettings(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, expected := range []string{`DOCKER_ENABLED="no"`, `DOCKER_TIMEOUT="20"`, `DOCKER_LOG_SIZE="100m"`, `DOCKER_USER_NETWORKS="preserve"`, `DOCKER_UNKNOWN="keep"`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("docker.cfg is missing %q:\n%s", expected, text)
		}
	}
}

func TestDockerSettingsValidation(t *testing.T) {
	live := &Live{bootDir: t.TempDir()}
	valid := model.DockerSettings{
		StopTimeout: 10, ImageType: "folder", ImageFile: "/mnt/user/system/docker/",
		AppConfigPath: "/mnt/user/appdata/", BackingFS: "overlay2",
		LogSize: "50m", LogFiles: 1, LogRotation: true, ReadMore: true,
	}
	if err := live.SaveDockerSettings(context.Background(), valid); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*model.DockerSettings){
		"timeout": func(value *model.DockerSettings) { value.StopTimeout = 0 },
		"path":    func(value *model.DockerSettings) { value.ImageFile = "/etc/docker/" },
		"logsize": func(value *model.DockerSettings) { value.LogSize = "4g" },
	}
	for name, mutate := range cases {
		value := valid
		mutate(&value)
		if err := live.SaveDockerSettings(context.Background(), value); err == nil {
			t.Fatalf("%s should be rejected", name)
		}
	}
}
