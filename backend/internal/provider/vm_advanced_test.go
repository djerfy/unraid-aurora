package provider

import (
	"testing"

	"aurora/internal/model"
)

func TestValidateVMAdvanced(t *testing.T) {
	valid := model.VMAdvancedConfig{
		VCPUs: 4, MemoryMB: 8192, CPUSet: "0-3",
		Networks:    []model.VMNetworkConfig{{Bridge: "br0", Model: "virtio", MAC: "52:54:00:aa:bb:cc"}},
		SoundModel:  "ich9",
		HostDevices: []model.VMHostDeviceRef{{Type: "pci", Address: "0000:01:00.0"}},
	}
	if err := validateVMAdvanced(valid); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*model.VMAdvancedConfig){
		"too many vcpus": func(value *model.VMAdvancedConfig) { value.VCPUs = 1024 },
		"bad bridge":     func(value *model.VMAdvancedConfig) { value.Networks[0].Bridge = "br0;rm" },
		"bad mac":        func(value *model.VMAdvancedConfig) { value.Networks[0].MAC = "zz" },
		"bad pci":        func(value *model.VMAdvancedConfig) { value.HostDevices[0].Address = "01:00" },
		"bad sound":      func(value *model.VMAdvancedConfig) { value.SoundModel = "unknown" },
		"bad share": func(value *model.VMAdvancedConfig) {
			value.SharedFolders = []model.VMSharedFolder{{Source: "/etc", Target: "etc"}}
		},
	} {
		value := valid
		value.Networks = append([]model.VMNetworkConfig(nil), valid.Networks...)
		value.HostDevices = append([]model.VMHostDeviceRef(nil), valid.HostDevices...)
		mutate(&value)
		if err := validateVMAdvanced(value); err == nil {
			t.Fatalf("%s should be rejected", name)
		}
	}
}

func TestVMUtilityHelpers(t *testing.T) {
	if memoryMB("2", "GiB") != 2048 || memoryMB("512", "MiB") != 512 || memoryMB("1048576", "KiB") != 1024 {
		t.Fatal("memory conversion is incorrect")
	}
	if pciAddress("0x0000", "0x1", "0x0", "0x0") != "0000:01:00.0" {
		t.Fatal("PCI address formatting is incorrect")
	}
	if !vmMACPattern.MatchString(newMAC()) {
		t.Fatal("generated MAC is invalid")
	}
	if len(newUUID()) != 36 {
		t.Fatal("generated UUID is invalid")
	}
}
