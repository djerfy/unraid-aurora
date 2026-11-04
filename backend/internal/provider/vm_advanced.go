package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"aurora/internal/model"
)

var vmCPUSetPattern = regexp.MustCompile(`^[0-9]+(-[0-9]+)?(,[0-9]+(-[0-9]+)?)*$`)
var vmMACPattern = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`)
var vmPCIAddressPattern = regexp.MustCompile(`^[0-9a-fA-F]{4}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}\.[0-7]$`)
var usbIDPattern = regexp.MustCompile(`^[0-9a-f]{4}$`)
var vmShareTargetPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

func (l *Live) VMAdvanced(ctx context.Context, id string) (model.VMAdvancedConfig, error) {
	name, err := l.vmNameForID(ctx, id)
	if err != nil {
		return model.VMAdvancedConfig{}, err
	}
	raw, err := exec.CommandContext(ctx, "virsh", "dumpxml", name).Output()
	if err != nil {
		return model.VMAdvancedConfig{}, fmt.Errorf("virsh dumpxml: %w", err)
	}
	root, err := parseXMLTree(raw)
	if err != nil {
		return model.VMAdvancedConfig{}, fmt.Errorf("parse domain XML: %w", err)
	}
	cfg := model.VMAdvancedConfig{Networks: []model.VMNetworkConfig{}, HostDevices: []model.VMHostDeviceRef{}, SharedFolders: []model.VMSharedFolder{}}
	if vcpu := root.child("vcpu"); vcpu != nil {
		cfg.VCPUs = atoi(strings.TrimSpace(vcpu.Text))
		cfg.CPUSet = vcpu.attribute("cpuset")
	}
	if memory := root.child("memory"); memory != nil {
		cfg.MemoryMB = memoryMB(strings.TrimSpace(memory.Text), memory.attribute("unit"))
	}
	devices := root.child("devices")
	if devices != nil {
		for _, child := range devices.Children {
			switch child.Name.Local {
			case "interface":
				if child.attribute("type") != "bridge" {
					// NAT, user and passthrough interfaces are preserved as-is.
					continue
				}
				network := model.VMNetworkConfig{Model: "virtio"}
				if source := child.child("source"); source != nil {
					network.Bridge = source.attribute("bridge")
				}
				if modelNode := child.child("model"); modelNode != nil {
					network.Model = modelNode.attribute("type")
				}
				if mac := child.child("mac"); mac != nil {
					network.MAC = mac.attribute("address")
				}
				cfg.Networks = append(cfg.Networks, network)
			case "sound":
				if cfg.SoundModel == "" {
					cfg.SoundModel = child.attribute("model")
				}
			case "filesystem":
				driver := child.child("driver")
				if driver == nil || driver.attribute("type") != "virtiofs" {
					continue
				}
				folder := model.VMSharedFolder{ReadOnly: child.child("readonly") != nil}
				if source := child.child("source"); source != nil {
					folder.Source = source.attribute("dir")
				}
				if target := child.child("target"); target != nil {
					folder.Target = target.attribute("dir")
				}
				cfg.SharedFolders = append(cfg.SharedFolders, folder)
			case "hostdev":
				device := model.VMHostDeviceRef{Type: child.attribute("type")}
				if source := child.child("source"); source != nil {
					if address := source.child("address"); address != nil {
						device.Address = pciAddress(
							address.attribute("domain"), address.attribute("bus"),
							address.attribute("slot"), address.attribute("function"),
						)
					}
					if vendor := source.child("vendor"); vendor != nil {
						device.Vendor = trimHex(vendor.attribute("id"))
					}
					if product := source.child("product"); product != nil {
						device.Product = trimHex(product.attribute("id"))
					}
				}
				cfg.HostDevices = append(cfg.HostDevices, device)
			}
		}
	}
	return cfg, nil
}

func (l *Live) UpdateVMAdvanced(ctx context.Context, id string, cfg model.VMAdvancedConfig) error {
	if err := validateVMAdvanced(cfg); err != nil {
		return err
	}
	name, err := l.vmNameForID(ctx, id)
	if err != nil {
		return err
	}
	raw, err := exec.CommandContext(ctx, "virsh", "dumpxml", name).Output()
	if err != nil {
		return fmt.Errorf("virsh dumpxml: %w", err)
	}
	root, err := parseXMLTree(raw)
	if err != nil {
		return fmt.Errorf("parse domain XML: %w", err)
	}
	vcpu := root.ensureChild("vcpu")
	vcpu.Text = strconv.Itoa(cfg.VCPUs)
	if cfg.CPUSet == "" {
		removeAttribute(vcpu, "cpuset")
	} else {
		vcpu.setAttribute("cpuset", cfg.CPUSet)
	}
	memory := root.ensureChild("memory")
	memory.Text = strconv.Itoa(cfg.MemoryMB)
	memory.setAttribute("unit", "MiB")
	if current := root.child("currentMemory"); current != nil {
		current.Text = strconv.Itoa(cfg.MemoryMB)
		current.setAttribute("unit", "MiB")
	}
	devices := root.ensureChild("devices")
	kept := make([]*xmlElement, 0, len(devices.Children)+len(cfg.Networks))
	for _, child := range devices.Children {
		switch child.Name.Local {
		case "interface":
			if child.attribute("type") == "bridge" {
				continue
			}
			kept = append(kept, child)
		case "sound", "hostdev":
			continue
		case "filesystem":
			if driver := child.child("driver"); driver != nil && driver.attribute("type") == "virtiofs" {
				continue
			}
			kept = append(kept, child)
		default:
			kept = append(kept, child)
		}
	}
	devices.Children = kept
	for _, network := range cfg.Networks {
		iface := &xmlElement{Name: xml.Name{Local: "interface"}}
		iface.setAttribute("type", "bridge")
		source := &xmlElement{Name: xml.Name{Local: "source"}}
		source.setAttribute("bridge", network.Bridge)
		iface.Children = append(iface.Children, source)
		if network.MAC != "" {
			mac := &xmlElement{Name: xml.Name{Local: "mac"}}
			mac.setAttribute("address", strings.ToLower(network.MAC))
			iface.Children = append(iface.Children, mac)
		}
		modelNode := &xmlElement{Name: xml.Name{Local: "model"}}
		modelNode.setAttribute("type", network.Model)
		iface.Children = append(iface.Children, modelNode)
		devices.Children = append(devices.Children, iface)
	}
	if cfg.SoundModel != "" {
		sound := &xmlElement{Name: xml.Name{Local: "sound"}}
		sound.setAttribute("model", cfg.SoundModel)
		devices.Children = append(devices.Children, sound)
	}
	for _, folder := range cfg.SharedFolders {
		filesystem := &xmlElement{Name: xml.Name{Local: "filesystem"}}
		filesystem.setAttribute("type", "mount")
		filesystem.setAttribute("accessmode", "passthrough")
		driver := &xmlElement{Name: xml.Name{Local: "driver"}}
		driver.setAttribute("type", "virtiofs")
		source := &xmlElement{Name: xml.Name{Local: "source"}}
		source.setAttribute("dir", folder.Source)
		target := &xmlElement{Name: xml.Name{Local: "target"}}
		target.setAttribute("dir", folder.Target)
		filesystem.Children = append(filesystem.Children, driver, source, target)
		if folder.ReadOnly {
			filesystem.Children = append(filesystem.Children, &xmlElement{Name: xml.Name{Local: "readonly"}})
		}
		devices.Children = append(devices.Children, filesystem)
	}

	for _, device := range cfg.HostDevices {
		hostdev := &xmlElement{Name: xml.Name{Local: "hostdev"}}
		hostdev.setAttribute("mode", "subsystem")
		hostdev.setAttribute("type", device.Type)
		hostdev.setAttribute("managed", "yes")
		source := &xmlElement{Name: xml.Name{Local: "source"}}
		if device.Type == "usb" {
			vendor := &xmlElement{Name: xml.Name{Local: "vendor"}}
			vendor.setAttribute("id", "0x"+device.Vendor)
			product := &xmlElement{Name: xml.Name{Local: "product"}}
			product.setAttribute("id", "0x"+device.Product)
			source.Children = append(source.Children, vendor, product)
		} else {
			address := &xmlElement{Name: xml.Name{Local: "address"}}
			parts := strings.Split(device.Address, ":")
			if len(parts) == 2 {
				busSlot := strings.Split(parts[1], ".")
				if len(busSlot) == 2 {
					address.setAttribute("domain", "0x"+parts[0])
					address.setAttribute("bus", "0x"+busSlot[0])
					address.setAttribute("slot", "0x"+busSlot[1])
					address.setAttribute("function", "0x0")
				}
			} else if len(parts) == 3 {
				busSlot := strings.Split(parts[2], ".")
				if len(busSlot) == 2 {
					address.setAttribute("domain", "0x"+parts[0])
					address.setAttribute("bus", "0x"+parts[1])
					address.setAttribute("slot", "0x"+busSlot[0])
					address.setAttribute("function", "0x"+busSlot[1])
				}
			}
			source.Children = append(source.Children, address)
		}
		hostdev.Children = append(hostdev.Children, source)
		devices.Children = append(devices.Children, hostdev)
	}

	var buffer bytes.Buffer
	buffer.WriteString(xml.Header)
	encoder := xml.NewEncoder(&buffer)
	encoder.Indent("", "  ")
	if err := root.encode(encoder); err != nil {
		return fmt.Errorf("encode domain XML: %w", err)
	}
	if err := encoder.Flush(); err != nil {
		return err
	}
	buffer.WriteString("\n")
	return l.UpdateVMXML(ctx, id, buffer.String())
}

func validateVMAdvanced(cfg model.VMAdvancedConfig) error {
	if cfg.VCPUs < 1 || cfg.VCPUs > 128 {
		return errors.New("vCPUs must be between 1 and 128")
	}
	if cfg.MemoryMB < 512 || cfg.MemoryMB > 1_048_576 {
		return errors.New("memory must be between 512 MiB and 1 TiB")
	}
	if cfg.CPUSet != "" && !vmCPUSetPattern.MatchString(cfg.CPUSet) {
		return errors.New("invalid CPU pinning set")
	}
	if len(cfg.Networks) < 1 || len(cfg.Networks) > 8 {
		return errors.New("between 1 and 8 network interfaces are required")
	}
	for _, network := range cfg.Networks {
		if !vmNetworkPattern.MatchString(network.Bridge) {
			return fmt.Errorf("invalid bridge %q", network.Bridge)
		}
		switch network.Model {
		case "virtio", "e1000", "rtl8139":
		default:
			return fmt.Errorf("unsupported network model %q", network.Model)
		}
		if network.MAC != "" && !vmMACPattern.MatchString(network.MAC) {
			return fmt.Errorf("invalid MAC address %q", network.MAC)
		}
	}
	switch cfg.SoundModel {
	case "", "ich9", "ich6", "ac97", "usb-audio":
	default:
		return fmt.Errorf("unsupported sound model %q", cfg.SoundModel)
	}
	if len(cfg.SharedFolders) > 16 {
		return errors.New("too many shared folders")
	}
	for _, folder := range cfg.SharedFolders {
		if !strings.HasPrefix(folder.Source, "/mnt/") || strings.Contains(folder.Source, "..") {
			return fmt.Errorf("invalid shared folder source %q", folder.Source)
		}
		if !vmShareTargetPattern.MatchString(folder.Target) {
			return fmt.Errorf("invalid shared folder target %q", folder.Target)
		}
	}
	if len(cfg.HostDevices) > 32 {
		return errors.New("too many host devices")
	}
	for _, device := range cfg.HostDevices {
		switch device.Type {
		case "pci":
			if !vmPCIAddressPattern.MatchString(device.Address) {
				return fmt.Errorf("invalid PCI address %q", device.Address)
			}
		case "usb":
			if len(device.Vendor) != 4 || len(device.Product) != 4 {
				return errors.New("USB vendor and product must be four hexadecimal digits")
			}
			if _, err := strconv.ParseUint(device.Vendor+device.Product, 16, 32); err != nil {
				return errors.New("invalid USB vendor or product")
			}
		default:
			return fmt.Errorf("unsupported host device type %q", device.Type)
		}
	}
	return nil
}

// AttachVMUSB hot-plugs a USB device into a running VM, or records it in the
// persistent domain definition when the VM is stopped.
func (l *Live) AttachVMUSB(ctx context.Context, id, vendor, product string) error {
	return l.vmUSBDevice(ctx, id, vendor, product, true)
}

func (l *Live) DetachVMUSB(ctx context.Context, id, vendor, product string) error {
	return l.vmUSBDevice(ctx, id, vendor, product, false)
}

func (l *Live) vmUSBDevice(ctx context.Context, id, vendor, product string, attach bool) error {
	vendor = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(vendor), "0x"))
	product = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(product), "0x"))
	if !usbIDPattern.MatchString(vendor) || !usbIDPattern.MatchString(product) {
		return errors.New("USB vendor and product must be four hexadecimal digits")
	}
	name, err := l.vmNameForID(ctx, id)
	if err != nil {
		return err
	}
	xmlData := fmt.Sprintf("<hostdev mode='subsystem' type='usb'><source><vendor id='0x%s'/><product id='0x%s'/></source></hostdev>", vendor, product)
	file, err := os.CreateTemp("", "aurora-usb-*.xml")
	if err != nil {
		return fmt.Errorf("create USB device XML: %w", err)
	}
	path := file.Name()
	defer os.Remove(path)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.WriteString(xmlData); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	action := "attach-device"
	if !attach {
		action = "detach-device"
	}
	args := []string{action, name, path}
	state, _ := exec.CommandContext(ctx, "virsh", "domstate", name).Output()
	if strings.Contains(strings.ToLower(string(state)), "running") {
		args = append(args, "--live")
	} else {
		args = append(args, "--config")
	}
	output, err := exec.CommandContext(ctx, "virsh", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("virsh %s USB: %w (%s)", action, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (l *Live) VMHostDevices(ctx context.Context) ([]model.VMHostDevice, error) {
	devices := []model.VMHostDevice{}
	pciNames := lspciNames(ctx)
	entries, _ := filepath.Glob("/sys/bus/pci/devices/*")
	sort.Strings(entries)
	for _, entry := range entries {
		address := filepath.Base(entry)
		class := strings.TrimSpace(readText(filepath.Join(entry, "class")))
		if strings.HasPrefix(class, "0x06") {
			continue
		}
		vendor := strings.TrimSpace(readText(filepath.Join(entry, "vendor")))
		product := strings.TrimSpace(readText(filepath.Join(entry, "device")))
		driver := ""
		if target, err := os.Readlink(filepath.Join(entry, "driver")); err == nil {
			driver = filepath.Base(target)
		}
		group := ""
		if target, err := os.Readlink(filepath.Join(entry, "iommu_group")); err == nil {
			group = filepath.Base(target)
		}
		name := pciNames[address]
		if name == "" {
			name = strings.TrimPrefix(vendor, "0x") + ":" + strings.TrimPrefix(product, "0x")
		}
		devices = append(devices, model.VMHostDevice{
			Type: "pci", Address: address, Vendor: strings.TrimPrefix(vendor, "0x"),
			Product: strings.TrimPrefix(product, "0x"), Name: name, Driver: driver,
			IOMMUGroup: group, Class: class,
		})
		if len(devices) >= 200 {
			break
		}
	}
	if output, err := exec.CommandContext(ctx, "lsusb").Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 6 || fields[4] != "ID" {
				continue
			}
			vendor, product, ok := strings.Cut(fields[5], ":")
			if !ok {
				continue
			}
			name := strings.Join(fields[6:], " ")
			devices = append(devices, model.VMHostDevice{
				Type: "usb", Vendor: vendor, Product: product, Name: name,
			})
		}
	}
	sort.SliceStable(devices, func(i, j int) bool {
		if devices[i].Type != devices[j].Type {
			return devices[i].Type < devices[j].Type
		}
		return strings.ToLower(devices[i].Name) < strings.ToLower(devices[j].Name)
	})
	return devices, nil
}

func (l *Live) CloneVM(ctx context.Context, id string, opts model.VMCloneOptions) (model.VM, error) {
	opts.Name = strings.TrimSpace(opts.Name)
	if !vmNamePattern.MatchString(opts.Name) || strings.Contains(opts.Name, "..") {
		return model.VM{}, errors.New("invalid VM name")
	}
	sourceName, err := l.vmNameForID(ctx, id)
	if err != nil {
		return model.VM{}, err
	}
	state, err := exec.CommandContext(ctx, "virsh", "domstate", sourceName).Output()
	if err != nil {
		return model.VM{}, fmt.Errorf("read VM state: %w", err)
	}
	if strings.HasPrefix(strings.TrimSpace(string(state)), "running") {
		return model.VM{}, errors.New("shut down the virtual machine before cloning it")
	}
	if err := exec.CommandContext(ctx, "virsh", "dominfo", opts.Name).Run(); err == nil {
		return model.VM{}, errors.New("a virtual machine with this name already exists")
	}
	raw, err := exec.CommandContext(ctx, "virsh", "dumpxml", sourceName).Output()
	if err != nil {
		return model.VM{}, fmt.Errorf("virsh dumpxml: %w", err)
	}
	root, err := parseXMLTree(raw)
	if err != nil {
		return model.VM{}, fmt.Errorf("parse domain XML: %w", err)
	}
	targetDir := filepath.Join("/mnt/user/domains", slug(opts.Name))
	if info, err := os.Stat(targetDir); err == nil && info.IsDir() {
		return model.VM{}, errors.New("the clone directory already exists")
	}
	if err := os.MkdirAll(targetDir, 0o775); err != nil {
		return model.VM{}, fmt.Errorf("create clone directory: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(targetDir)
		}
	}()

	root.setText("name", opts.Name)
	root.setText("uuid", newUUID())
	devices := root.child("devices")
	if devices == nil {
		return model.VM{}, errors.New("the VM has no devices section")
	}
	diskIndex := 0
	for _, child := range devices.Children {
		switch child.Name.Local {
		case "disk":
			source := child.child("source")
			if source == nil {
				continue
			}
			oldPath := source.attribute("file")
			if oldPath == "" {
				continue
			}
			clean := filepath.Clean(oldPath)
			if !strings.HasPrefix(clean, "/mnt/") {
				return model.VM{}, fmt.Errorf("disk %q is outside /mnt", oldPath)
			}
			diskIndex++
			newPath := filepath.Join(targetDir, fmt.Sprintf("vdisk%d.img", diskIndex))
			format := "qcow2"
			if driver := child.child("driver"); driver != nil && driver.attribute("type") != "" {
				format = driver.attribute("type")
			}
			if err := copyVMDisk(ctx, clean, newPath, format); err != nil {
				return model.VM{}, err
			}
			source.setAttribute("file", newPath)
		case "interface":
			mac := child.child("mac")
			if mac != nil {
				mac.setAttribute("address", newMAC())
			}
		case "nvram":
			oldPath := strings.TrimSpace(child.Text)
			if oldPath == "" {
				continue
			}
			clean := filepath.Clean(oldPath)
			if !strings.HasPrefix(clean, "/mnt/") {
				return model.VM{}, fmt.Errorf("NVRAM %q is outside /mnt", oldPath)
			}
			newPath := filepath.Join(targetDir, filepath.Base(clean))
			raw, err := os.ReadFile(clean)
			if err != nil {
				return model.VM{}, fmt.Errorf("read NVRAM: %w", err)
			}
			if err := os.WriteFile(newPath, raw, 0o600); err != nil {
				return model.VM{}, fmt.Errorf("copy NVRAM: %w", err)
			}
			child.Text = newPath
		}
	}
	if diskIndex == 0 {
		return model.VM{}, errors.New("the source VM has no file-backed disk")
	}

	var buffer bytes.Buffer
	buffer.WriteString(xml.Header)
	encoder := xml.NewEncoder(&buffer)
	encoder.Indent("", "  ")
	if err := root.encode(encoder); err != nil {
		return model.VM{}, err
	}
	if err := encoder.Flush(); err != nil {
		return model.VM{}, err
	}
	buffer.WriteString("\n")
	temp, err := os.CreateTemp("", "aurora-clone-*.xml")
	if err != nil {
		return model.VM{}, err
	}
	tempName := temp.Name()
	_ = temp.Close()
	defer os.Remove(tempName)
	if err := os.WriteFile(tempName, buffer.Bytes(), 0o600); err != nil {
		return model.VM{}, err
	}
	if output, err := exec.CommandContext(ctx, "virsh", "define", tempName).CombinedOutput(); err != nil {
		return model.VM{}, fmt.Errorf("define clone: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	if opts.Start {
		if output, err := exec.CommandContext(ctx, "virsh", "start", opts.Name).CombinedOutput(); err != nil {
			_, _ = exec.CommandContext(context.WithoutCancel(ctx), "virsh", "undefine", opts.Name, "--nvram").CombinedOutput()
			return model.VM{}, fmt.Errorf("start clone: %w (%s)", err, strings.TrimSpace(string(output)))
		}
	}
	cleanup = false
	l.statsMu.Lock()
	l.vmCacheAt = time.Time{}
	l.statsMu.Unlock()
	return l.vmDetails(ctx, opts.Name), nil
}

func (l *Live) ResizeVMDisk(ctx context.Context, id string, opts model.VMDiskResizeOptions) error {
	sizeGB := opts.SizeGB
	if sizeGB < 1 || sizeGB > 10_000 {
		return errors.New("disk size must be between 1 and 10000 GiB")
	}
	name, err := l.vmNameForID(ctx, id)
	if err != nil {
		return err
	}
	raw, err := exec.CommandContext(ctx, "virsh", "dumpxml", name).Output()
	if err != nil {
		return fmt.Errorf("virsh dumpxml: %w", err)
	}
	root, err := parseXMLTree(raw)
	if err != nil {
		return fmt.Errorf("parse domain XML: %w", err)
	}
	devices := root.child("devices")
	if devices == nil {
		return errors.New("the VM has no devices section")
	}
	var diskPath, targetDev string
	for _, child := range devices.Children {
		if child.Name.Local != "disk" || child.attribute("device") != "disk" {
			continue
		}
		if source := child.child("source"); source != nil {
			diskPath = source.attribute("file")
		}
		if target := child.child("target"); target != nil {
			targetDev = target.attribute("dev")
		}
		if diskPath != "" {
			break
		}
	}
	clean := filepath.Clean(diskPath)
	if diskPath == "" || !strings.HasPrefix(clean, "/mnt/") {
		return errors.New("the VM has no resizable file-backed disk")
	}
	infoRaw, err := exec.CommandContext(ctx, "qemu-img", "info", "--output=json", clean).Output()
	if err != nil {
		return fmt.Errorf("read disk size: %w", err)
	}
	var info struct {
		VirtualSize int64 `json:"virtual-size"`
	}
	if err := json.Unmarshal(infoRaw, &info); err != nil {
		return fmt.Errorf("decode disk size: %w", err)
	}
	targetSize := sizeGB * (1 << 30)
	if targetSize <= info.VirtualSize {
		return fmt.Errorf("the new size must be larger than the current %d GiB", info.VirtualSize/(1<<30))
	}
	state, _ := exec.CommandContext(ctx, "virsh", "domstate", name).Output()
	if strings.HasPrefix(strings.TrimSpace(string(state)), "running") {
		if targetDev == "" {
			return errors.New("the running disk has no target device")
		}
		output, err := exec.CommandContext(ctx, "virsh", "blockresize", name, targetDev, fmt.Sprintf("%dG", sizeGB)).CombinedOutput()
		if err != nil {
			return fmt.Errorf("resize running disk: %w (%s)", err, strings.TrimSpace(string(output)))
		}
	} else {
		output, err := exec.CommandContext(ctx, "qemu-img", "resize", clean, fmt.Sprintf("%dG", sizeGB)).CombinedOutput()
		if err != nil {
			return fmt.Errorf("resize disk: %w (%s)", err, strings.TrimSpace(string(output)))
		}
	}
	l.statsMu.Lock()
	l.vmCacheAt = time.Time{}
	l.statsMu.Unlock()
	return nil
}

func copyVMDisk(ctx context.Context, source, target, format string) error {
	if format == "" {
		format = "qcow2"
	}
	output, err := exec.CommandContext(ctx, "qemu-img", "convert", "-O", format, source, target).CombinedOutput()
	if err != nil {
		return fmt.Errorf("copy VM disk: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func memoryMB(value, unit string) int {
	number := atoi(value)
	switch strings.ToLower(unit) {
	case "kib", "kb":
		return number / 1024
	case "gib", "gb":
		return number * 1024
	case "tib", "tb":
		return number * 1024 * 1024
	default:
		return number
	}
}

func pciAddress(domain, bus, slot, function string) string {
	domain = strings.TrimPrefix(domain, "0x")
	bus = strings.TrimPrefix(bus, "0x")
	slot = strings.TrimPrefix(slot, "0x")
	function = strings.TrimPrefix(function, "0x")
	if len(domain) < 4 {
		domain = strings.Repeat("0", 4-len(domain)) + domain
	}
	if len(bus) < 2 {
		bus = "0" + bus
	}
	if len(slot) < 2 {
		slot = "0" + slot
	}
	return domain + ":" + bus + ":" + slot + "." + function
}

func trimHex(value string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "0x")
}

func removeAttribute(element *xmlElement, name string) {
	attrs := element.Attrs[:0]
	for _, attr := range element.Attrs {
		if attr.Name.Local != name {
			attrs = append(attrs, attr)
		}
	}
	element.Attrs = attrs
}

func readText(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func lspciNames(ctx context.Context) map[string]string {
	names := map[string]string{}
	output, err := exec.CommandContext(ctx, "lspci", "-Dnn").Output()
	if err != nil {
		return names
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		address := fields[0]
		name := strings.Join(fields[1:], " ")
		names[address] = name
	}
	return names
}

func newUUID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(raw[0:4]), hex.EncodeToString(raw[4:6]), hex.EncodeToString(raw[6:8]),
		hex.EncodeToString(raw[8:10]), hex.EncodeToString(raw[10:16]))
}

func newMAC() string {
	raw := make([]byte, 3)
	if _, err := rand.Read(raw); err != nil {
		raw = []byte{byte(time.Now().UnixNano()), byte(time.Now().UnixNano() >> 8), byte(time.Now().UnixNano() >> 16)}
	}
	return fmt.Sprintf("52:54:00:%02x:%02x:%02x", raw[0], raw[1], raw[2])
}
