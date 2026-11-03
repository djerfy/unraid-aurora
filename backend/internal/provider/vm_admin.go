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
	"regexp"
	"strings"
	"time"

	"aurora/internal/model"
)

var vmNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _.-]{0,63}$`)
var vmSnapshotPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)
var vmNetworkPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,31}$`)

func validateVMCreateOptions(opts model.VMCreateOptions) error {
	opts.Name = strings.TrimSpace(opts.Name)
	if !vmNamePattern.MatchString(opts.Name) || strings.Contains(opts.Name, "..") {
		return errors.New("VM name must start with a letter or digit and contain only letters, digits, spaces, '_', '.' or '-'")
	}
	if opts.VCPUs < 1 || opts.VCPUs > 128 {
		return errors.New("vCPUs must be between 1 and 128")
	}
	if opts.MemoryMB < 512 || opts.MemoryMB > 1_048_576 {
		return errors.New("memory must be between 512 MiB and 1 TiB")
	}
	if opts.DiskGB < 1 || opts.DiskGB > 10_000 {
		return errors.New("disk size must be between 1 and 10000 GiB")
	}
	if opts.Network == "" {
		opts.Network = "br0"
	}
	if !vmNetworkPattern.MatchString(opts.Network) {
		return errors.New("invalid network bridge")
	}
	if opts.ISOPath != "" {
		iso := filepath.Clean(opts.ISOPath)
		if !filepath.IsAbs(iso) || !strings.HasPrefix(iso, "/mnt/") || !strings.HasSuffix(strings.ToLower(iso), ".iso") {
			return errors.New("ISO must be an absolute .iso path under /mnt")
		}
		info, err := os.Stat(iso)
		if err != nil || info.IsDir() {
			return errors.New("ISO file was not found")
		}
	}
	return nil
}

func (l *Live) CreateVM(ctx context.Context, opts model.VMCreateOptions) (model.VM, error) {
	opts.Name = strings.TrimSpace(opts.Name)
	if err := validateVMCreateOptions(opts); err != nil {
		return model.VM{}, err
	}
	if err := exec.CommandContext(ctx, "virsh", "dominfo", opts.Name).Run(); err == nil {
		return model.VM{}, errors.New("a virtual machine with this name already exists")
	}
	slug := slug(opts.Name)
	dir := filepath.Join("/mnt/user/domains", slug)
	if err := os.MkdirAll(dir, 0o775); err != nil {
		return model.VM{}, fmt.Errorf("create VM directory: %w", err)
	}
	diskPath := filepath.Join(dir, "vdisk1.img")
	if _, err := os.Stat(diskPath); err == nil {
		return model.VM{}, errors.New("the VM disk already exists")
	}
	create := exec.CommandContext(ctx, "qemu-img", "create", "-f", "qcow2", diskPath, fmt.Sprintf("%dG", opts.DiskGB))
	if output, err := create.CombinedOutput(); err != nil {
		return model.VM{}, fmt.Errorf("create VM disk: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(diskPath)
		}
	}()

	raw, err := vmDomainXML(opts, diskPath)
	if err != nil {
		return model.VM{}, err
	}
	tmp, err := os.CreateTemp("", "aurora-vm-*.xml")
	if err != nil {
		return model.VM{}, err
	}
	tmpName := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpName)
	if err := os.WriteFile(tmpName, raw, 0o600); err != nil {
		return model.VM{}, err
	}
	if output, err := exec.CommandContext(ctx, "virsh", "define", tmpName).CombinedOutput(); err != nil {
		return model.VM{}, fmt.Errorf("define VM: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	if opts.Start {
		if output, err := exec.CommandContext(ctx, "virsh", "start", opts.Name).CombinedOutput(); err != nil {
			_, _ = exec.CommandContext(context.WithoutCancel(ctx), "virsh", "undefine", opts.Name, "--nvram").CombinedOutput()
			return model.VM{}, fmt.Errorf("start VM: %w (%s)", err, strings.TrimSpace(string(output)))
		}
	}
	cleanup = false
	l.statsMu.Lock()
	l.vmCacheAt = time.Time{}
	l.statsMu.Unlock()
	return l.vmDetails(ctx, opts.Name), nil
}

func vmDomainXML(opts model.VMCreateOptions, diskPath string) ([]byte, error) {
	var b bytes.Buffer
	esc := func(value string) string {
		var out bytes.Buffer
		_ = xml.EscapeText(&out, []byte(value))
		return out.String()
	}
	fmt.Fprintf(&b, `<domain type='kvm'>
  <name>%s</name>
  <memory unit='MiB'>%d</memory>
  <vcpu>%d</vcpu>
  <os>
    <type arch='x86_64'>hvm</type>
    <boot dev='cdrom'/>
    <boot dev='hd'/>
  </os>
  <features><acpi/><apic/></features>
  <cpu mode='host-passthrough' check='none'/>
  <devices>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='%s'/>
      <target dev='vda' bus='virtio'/>
    </disk>
`, esc(opts.Name), opts.MemoryMB, opts.VCPUs, esc(diskPath))
	if strings.TrimSpace(opts.ISOPath) != "" {
		fmt.Fprintf(&b, `    <disk type='file' device='cdrom'>
      <driver name='qemu' type='raw'/>
      <source file='%s'/>
      <target dev='hda' bus='sata'/>
      <readonly/>
    </disk>
`, esc(filepath.Clean(opts.ISOPath)))
	}
	network := opts.Network
	if network == "" {
		network = "br0"
	}
	fmt.Fprintf(&b, `    <interface type='bridge'>
      <source bridge='%s'/>
      <model type='virtio'/>
    </interface>
    <controller type='usb' index='0' model='qemu-xhci'/>
    <graphics type='vnc' port='-1' autoport='yes' listen='0.0.0.0'/>
    <video><model type='qxl'/></video>
    <input type='tablet' bus='usb'/>
    <input type='keyboard' bus='ps2'/>
  </devices>
</domain>
`, esc(network))
	return b.Bytes(), nil
}

func (l *Live) UpdateVMXML(ctx context.Context, id, raw string) error {
	name, err := l.vmNameForID(ctx, id)
	if err != nil {
		return err
	}
	var header struct {
		Name string `xml:"name"`
	}
	if err := xml.Unmarshal([]byte(raw), &header); err != nil {
		return fmt.Errorf("invalid domain XML: %w", err)
	}
	if strings.TrimSpace(header.Name) != name {
		return errors.New("the domain name cannot be changed from the XML editor")
	}
	tmp, err := os.CreateTemp("", "aurora-vm-*.xml")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpName)
	if err := os.WriteFile(tmpName, []byte(raw), 0o600); err != nil {
		return err
	}
	output, err := exec.CommandContext(ctx, "virsh", "define", tmpName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("update VM: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	l.statsMu.Lock()
	l.vmCacheAt = time.Time{}
	l.statsMu.Unlock()
	return nil
}

func (l *Live) VMSnapshots(ctx context.Context, id string) ([]model.VMSnapshot, error) {
	name, err := l.vmNameForID(ctx, id)
	if err != nil {
		return nil, err
	}
	output, err := exec.CommandContext(ctx, "virsh", "snapshot-list", name, "--name").Output()
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}
	out := []model.VMSnapshot{}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, model.VMSnapshot{Name: line})
		}
	}
	return out, nil
}

func (l *Live) CreateVMSnapshot(ctx context.Context, id, snapshotName, description string) error {
	name, err := l.vmNameForID(ctx, id)
	if err != nil {
		return err
	}
	snapshotName = strings.TrimSpace(snapshotName)
	if !vmSnapshotPattern.MatchString(snapshotName) {
		return errors.New("invalid snapshot name")
	}
	args := []string{"snapshot-create-as", name, snapshotName}
	if strings.TrimSpace(description) != "" {
		args = append(args, "--description", strings.TrimSpace(description))
	}
	output, err := exec.CommandContext(ctx, "virsh", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("create snapshot: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (l *Live) RestoreVMSnapshot(ctx context.Context, id, snapshotName string) error {
	name, err := l.vmNameForID(ctx, id)
	if err != nil {
		return err
	}
	if !vmSnapshotPattern.MatchString(snapshotName) {
		return errors.New("invalid snapshot name")
	}
	output, err := exec.CommandContext(ctx, "virsh", "snapshot-revert", name, snapshotName, "--running").CombinedOutput()
	if err != nil {
		return fmt.Errorf("restore snapshot: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (l *Live) DeleteVMSnapshot(ctx context.Context, id, snapshotName string) error {
	name, err := l.vmNameForID(ctx, id)
	if err != nil {
		return err
	}
	if !vmSnapshotPattern.MatchString(snapshotName) {
		return errors.New("invalid snapshot name")
	}
	output, err := exec.CommandContext(ctx, "virsh", "snapshot-delete", name, snapshotName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("delete snapshot: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}
