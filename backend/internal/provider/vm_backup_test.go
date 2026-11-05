package provider

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVMBackupRoot(t *testing.T) {
	for _, value := range []string{"/mnt/user/backups", "/mnt/cache/vm-backups", "/mnt/disk1/aurora"} {
		if _, err := vmBackupRoot(value); err != nil {
			t.Fatalf("expected %q to be accepted: %v", value, err)
		}
	}
	for _, value := range []string{"", "backups", "/mnt", "/tmp/backups", "/mnt/../etc", "/mnt"} {
		if _, err := vmBackupRoot(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestParseVMDomainDisks(t *testing.T) {
	raw := []byte(`<domain type='kvm'>
  <name>ubuntu-media</name>
  <nvram>/etc/libvirt/qemu/nvram/ubuntu-media_VARS.fd</nvram>
  <devices>
    <disk type='file' device='disk'><source file='/mnt/user/domains/ubuntu/vdisk1.img'/></disk>
    <disk type='file' device='cdrom'><source file='/mnt/user/isos/ubuntu.iso'/></disk>
    <disk type='file' device='disk'><source file='/mnt/user/domains/ubuntu/vdisk2.img'/></disk>
    <disk type='file' device='disk'><source file='/mnt/user/domains/ubuntu/vdisk1.img'/></disk>
    <disk type='network' device='disk'><source file=''/></disk>
  </devices>
</domain>`)
	domain, err := parseVMDomain(raw)
	if err != nil {
		t.Fatal(err)
	}
	if domain.Name != "ubuntu-media" {
		t.Fatalf("unexpected name: %q", domain.Name)
	}
	if domain.NVRAM != "/etc/libvirt/qemu/nvram/ubuntu-media_VARS.fd" {
		t.Fatalf("unexpected NVRAM: %q", domain.NVRAM)
	}
	disks := vmDiskSources(domain)
	if len(disks) != 2 || disks[0] != "/mnt/user/domains/ubuntu/vdisk1.img" || disks[1] != "/mnt/user/domains/ubuntu/vdisk2.img" {
		t.Fatalf("unexpected disks: %#v", disks)
	}
}

func TestReadVMBackupManifest(t *testing.T) {
	dir := t.TempDir()
	payload := `{"version":1,"vmId":"vm-test","vmName":"test","created":"2026-11-05T00:00:00Z","files":[{"name":"disk-01.qcow2","kind":"disk","target":"/mnt/user/domains/test/vdisk1.img","format":"raw","size":1024,"sha256":"abc"}]}`
	if err := os.WriteFile(filepath.Join(dir, vmBackupManifestName), []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := readVMBackupManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.VMName != "test" || len(manifest.Files) != 1 || manifest.Files[0].Target == "" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
}

func TestReadVMBackupManifestRejectsIncomplete(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, vmBackupManifestName), []byte(`{"version":2,"vmName":"test","files":[]}`), 0o600)
	if _, err := readVMBackupManifest(dir); err == nil {
		t.Fatal("expected an incomplete manifest to be rejected")
	}
}

func TestParseQEMUProgress(t *testing.T) {
	values := []float64{}
	parseQEMUProgress(strings.NewReader("\r    (12.50/100%)\r    (50.00/100%)\r    (100.00/100%)\n"), func(value float64) {
		values = append(values, value)
	})
	if len(values) != 3 || values[0] != 12.5 || values[1] != 50 || values[2] != 100 {
		t.Fatalf("unexpected progress values: %#v", values)
	}
}

func TestConvertDiskWithQEMUIMG(t *testing.T) {
	if _, err := exec.LookPath("qemu-img"); err != nil {
		t.Skip("qemu-img is not installed")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "source.raw")
	if err := os.WriteFile(source, make([]byte, 1<<20), 0o600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(dir, "target.qcow2")
	var last float64
	if err := convertDisk(context.Background(), source, destination, "qcow2", func(percent float64) {
		last = percent
	}); err != nil {
		t.Fatal(err)
	}
	if last < 0 || last > 100 {
		t.Fatalf("invalid progress: %v", last)
	}
	if info, err := os.Stat(destination); err != nil || info.Size() == 0 {
		t.Fatalf("destination was not written: %v", err)
	}
}
