package provider

import (
	"encoding/json"
	"strings"
	"testing"

	"aurora/internal/model"
)

func TestContainerAdvancedValidation(t *testing.T) {
	valid := model.ContainerAdvanced{
		MacAddress:  "02:42:ac:11:00:02",
		Devices:     []model.ContainerDevice{{HostPath: "/dev/dri", ContainerPath: "/dev/dri", Permissions: "rwm"}},
		CapAdd:      []string{"NET_ADMIN"},
		Sysctls:     map[string]string{"net.ipv4.ip_forward": "1"},
		Ulimits:     []model.ContainerUlimit{{Name: "nofile", Soft: 1024, Hard: 2048}},
		Labels:      map[string]string{"com.example.role": "media"},
		MemoryLimit: 512 << 20,
		NanoCPUs:    1_500_000_000,
		CPUSet:      "0-3,8",
		PidsLimit:   512,
	}
	if err := validateContainerAdvanced(valid); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*model.ContainerAdvanced){
		"device outside /dev": func(value *model.ContainerAdvanced) { value.Devices[0].HostPath = "/etc/passwd" },
		"ALL capability":      func(value *model.ContainerAdvanced) { value.CapAdd = []string{"ALL"} },
		"reserved label":      func(value *model.ContainerAdvanced) { value.Labels = map[string]string{"net.unraid.docker.icon": "x"} },
		"bad cpuset":          func(value *model.ContainerAdvanced) { value.CPUSet = "0-3,not-a-cpu" },
		"bad memory":          func(value *model.ContainerAdvanced) { value.MemoryLimit = 1024 },
	} {
		value := valid
		value.Devices = append([]model.ContainerDevice(nil), valid.Devices...)
		value.CapAdd = append([]string(nil), valid.CapAdd...)
		value.Labels = map[string]string{}
		for key, item := range valid.Labels {
			value.Labels[key] = item
		}
		mutate(&value)
		if err := validateContainerAdvanced(value); err == nil {
			t.Fatalf("%s should be rejected", name)
		}
	}
}

func TestContainerAdvancedCreateBody(t *testing.T) {
	body := buildContainerCreateBody(model.ContainerCreateOptions{
		Name:          "plex",
		Image:         "linuxserver/plex:latest",
		NetworkMode:   "br0",
		RestartPolicy: "unless-stopped",
		Privileged:    false,
		Advanced: model.ContainerAdvanced{
			MacAddress:  "02:42:ac:11:00:02",
			Devices:     []model.ContainerDevice{{HostPath: "/dev/dri", ContainerPath: "/dev/dri", Permissions: "rwm"}},
			CapAdd:      []string{"NET_ADMIN"},
			Sysctls:     map[string]string{"net.ipv4.ip_forward": "1"},
			Labels:      map[string]string{"com.example.role": "media"},
			MemoryLimit: 512 << 20,
			CPUSet:      "0-3",
			PidsLimit:   512,
		},
	})
	host, _ := body["HostConfig"].(map[string]any)
	if host["Memory"] != int64(512<<20) || host["CpusetCpus"] != "0-3" || host["PidsLimit"] != int64(512) {
		t.Fatalf("resource limits missing: %+v", host)
	}
	devices, _ := host["Devices"].([]any)
	if len(devices) != 1 {
		t.Fatalf("devices missing: %+v", host["Devices"])
	}
	caps, _ := host["CapAdd"].([]string)
	if len(caps) != 1 || caps[0] != "NET_ADMIN" {
		t.Fatalf("capabilities missing: %+v", host["CapAdd"])
	}
	labels, _ := body["Labels"].(map[string]any)
	if labels["com.example.role"] != "media" || labels["net.unraid.docker.managed"] != "dockerman" {
		t.Fatalf("labels missing: %+v", labels)
	}
	if networking, _ := body["NetworkingConfig"].(map[string]any); networking == nil {
		t.Fatal("fixed MAC should create a network endpoint")
	}
}

func TestContainerAdvancedJSONKeepsEmptyCollections(t *testing.T) {
	raw, err := json.Marshal(model.ContainerConfig{Advanced: model.ContainerAdvanced{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"devices":[]`, `"capAdd":[]`, `"sysctls":{}`, `"ulimits":[]`, `"labels":{}`, `"tmpfs":[]`} {
		if !strings.Contains(string(raw), expected) {
			t.Fatalf("JSON is missing %s: %s", expected, raw)
		}
	}
}

func TestContainerAdvancedExtraParamsRoundTrip(t *testing.T) {
	advanced := model.ContainerAdvanced{
		Devices:     []model.ContainerDevice{{HostPath: "/dev/dri", ContainerPath: "/dev/dri", Permissions: "rwm"}},
		CapAdd:      []string{"NET_ADMIN"},
		Sysctls:     map[string]string{"net.ipv4.ip_forward": "1"},
		Ulimits:     []model.ContainerUlimit{{Name: "nofile", Soft: 1024, Hard: 2048}},
		Labels:      map[string]string{"com.example.role": "media server"},
		MemoryLimit: 512 << 20,
		CPUSet:      "0-3",
	}
	extra := mergeExtraParams("--restart=always --unknown=keep", advanced, "unless-stopped")
	for _, expected := range []string{"--unknown=keep", "--device=/dev/dri:/dev/dri:rwm", "--cap-add=NET_ADMIN", "--sysctl=net.ipv4.ip_forward=1", "--ulimit=nofile=1024:2048", "--restart=unless-stopped"} {
		if !strings.Contains(extra, expected) {
			t.Fatalf("missing %q in %q", expected, extra)
		}
	}
	parsed := containerAdvancedFromExtraParams(extra)
	if len(parsed.Devices) != 1 || len(parsed.CapAdd) != 1 || parsed.Sysctls["net.ipv4.ip_forward"] != "1" {
		t.Fatalf("round trip failed: %+v", parsed)
	}
	if parsed.MemoryLimit != 0 { // memory is stored in the template field, not ExtraParams
		t.Fatalf("unexpected memory in extra params: %d", parsed.MemoryLimit)
	}
}
