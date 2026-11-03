package provider

import (
	"strings"
	"testing"

	"aurora/internal/model"
)

func TestValidateVMCreateOptions(t *testing.T) {
	base := model.VMCreateOptions{Name: "Windows 11", VCPUs: 4, MemoryMB: 8192, DiskGB: 64, Network: "br0"}
	if err := validateVMCreateOptions(base); err != nil {
		t.Fatalf("valid options rejected: %v", err)
	}
	cases := map[string]model.VMCreateOptions{
		"bad name":   {Name: "../escape", VCPUs: 4, MemoryMB: 8192, DiskGB: 64},
		"no vcpus":   {Name: "VM", VCPUs: 0, MemoryMB: 8192, DiskGB: 64},
		"low memory": {Name: "VM", VCPUs: 1, MemoryMB: 128, DiskGB: 64},
		"bad disk":   {Name: "VM", VCPUs: 1, MemoryMB: 1024, DiskGB: 0},
		"bad bridge": {Name: "VM", VCPUs: 1, MemoryMB: 1024, DiskGB: 64, Network: "br0;rm"},
	}
	for name, opts := range cases {
		if err := validateVMCreateOptions(opts); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestVMDomainXML(t *testing.T) {
	raw, err := vmDomainXML(model.VMCreateOptions{
		Name: "Test VM", VCPUs: 2, MemoryMB: 2048, Network: "br0",
	}, "/mnt/user/domains/test/vdisk1.img")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{"<name>Test VM</name>", "<vcpu>2</vcpu>", "<memory unit='MiB'>2048</memory>", "/mnt/user/domains/test/vdisk1.img", "bridge='br0'"} {
		if !strings.Contains(text, want) {
			t.Errorf("XML missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "device='cdrom'") {
		t.Errorf("unexpected cdrom without an ISO")
	}
}
