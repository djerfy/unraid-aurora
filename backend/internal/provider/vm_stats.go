package provider

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"aurora/internal/model"
)

type vmCPUSample struct {
	at      time.Time
	cpuTime uint64 // nanoseconds
}

func (l *Live) vmDetails(ctx context.Context, name string) model.VM {
	vm := model.VM{
		ID: "vm-" + slug(name), Name: name, State: "stopped", OS: name,
		Machine: "q35", Color: colorFor(name), Icon: "server",
	}

	if out, err := exec.CommandContext(ctx, "virsh", "domstate", name).Output(); err == nil {
		state := strings.TrimSpace(string(out))
		switch {
		case strings.HasPrefix(state, "running"):
			vm.State = "running"
		case strings.HasPrefix(state, "paused"):
			vm.State = "paused"
		case strings.HasPrefix(state, "shut off"), strings.HasPrefix(state, "crashed"):
			vm.State = "stopped"
		default:
			vm.State = state
		}
	}

	info := virshColonInfo(ctx, "dominfo", name)
	if vcpus := firstInt(info["CPU(s)"]); vcpus > 0 {
		vm.VCPUs = vcpus
	}
	if maxKiB := firstInt(info["Max memory"]); maxKiB > 0 {
		vm.Memory = uint64(maxKiB) * 1024
	}
	if strings.EqualFold(info["Autostart"], "enable") {
		vm.Autostart = true
	}

	if vm.State == "running" {
		if console, err := virshGraphics(ctx, name); err == nil {
			vm.ConsoleProtocol = console.Protocol
			vm.ConsolePort = console.Port
			vm.ConsoleWebSocket = console.WebSocket
			vm.ConsoleKeyboard = console.Keyboard
		}
		l.applyVMRuntime(ctx, &vm)
	}
	vm.DiskSize, vm.DiskUsed = vmDiskUsage(ctx, name)
	return vm
}

// VMConsole resolves the VNC/SPICE websocket exposed by libvirt for a running
// domain. The client never supplies the port, so the proxy cannot be pointed at
// an arbitrary local service.
func (l *Live) VMConsole(ctx context.Context, id string) (model.VMConsole, error) {
	vms, err := l.VMs(ctx)
	if err != nil {
		return model.VMConsole{}, err
	}
	for _, vm := range vms {
		if vm.ID != id {
			continue
		}
		if vm.State != "running" {
			return model.VMConsole{}, errors.New("virtual machine is not running")
		}
		console, err := virshGraphics(ctx, vm.Name)
		if err != nil {
			return model.VMConsole{}, err
		}
		// Autoport domains can report port=-1 in the XML; virsh knows the
		// display allocated at runtime.
		if console.Port < 1 {
			if address, port := virshVNCAddress(ctx, vm.Name); port > 0 {
				console.Port = port
				if console.Address == "" {
					console.Address = address
				}
			}
		}
		if console.WebSocket < 1 && console.Port < 1 {
			return model.VMConsole{}, errors.New("this virtual machine does not expose a console")
		}
		return console, nil
	}
	return model.VMConsole{}, errors.New("virtual machine not found")
}

// virshVNCAddress parses `virsh vncdisplay`, which reports the runtime display
// as "host:display" even when the XML uses autoport.
func virshVNCAddress(ctx context.Context, name string) (string, int) {
	out, err := exec.CommandContext(ctx, "virsh", "vncdisplay", name).Output()
	if err != nil {
		return "", 0
	}
	value := strings.TrimSpace(string(out))
	if value == "" {
		return "", 0
	}
	host := ""
	if index := strings.LastIndex(value, ":"); index >= 0 {
		host = value[:index]
		value = value[index+1:]
	}
	display, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || display < 0 {
		return "", 0
	}
	return host, 5900 + display
}

func virshGraphics(ctx context.Context, name string) (model.VMConsole, error) {
	out, err := exec.CommandContext(ctx, "virsh", "dumpxml", name).Output()
	if err != nil {
		return model.VMConsole{}, fmt.Errorf("virsh dumpxml: %w", err)
	}
	return parseDomainGraphics(out, name)
}

func parseDomainGraphics(raw []byte, name string) (model.VMConsole, error) {
	var domain struct {
		Devices struct {
			Graphics []struct {
				Type      string `xml:"type,attr"`
				Port      int    `xml:"port,attr"`
				WebSocket int    `xml:"websocket,attr"`
				Listen    string `xml:"listen,attr"`
				Addresses []struct {
					Address string `xml:"address,attr"`
				} `xml:"listen"`
			} `xml:"graphics"`
			Inputs []struct {
				Type string `xml:"type,attr"`
			} `xml:"input"`
		} `xml:"devices"`
	}
	if err := xml.Unmarshal(raw, &domain); err != nil {
		return model.VMConsole{}, fmt.Errorf("parse domain xml: %w", err)
	}
	hasKeyboard := false
	for _, input := range domain.Devices.Inputs {
		if strings.EqualFold(input.Type, "keyboard") {
			hasKeyboard = true
			break
		}
	}
	var fallback model.VMConsole
	for _, graphics := range domain.Devices.Graphics {
		protocol := strings.ToLower(graphics.Type)
		if protocol != "vnc" && protocol != "spice" {
			continue
		}
		console := model.VMConsole{Name: name, Protocol: protocol, Port: graphics.Port, WebSocket: graphics.WebSocket, Keyboard: hasKeyboard}
		console.Address = graphics.Listen
		if console.Address == "" && len(graphics.Addresses) > 0 {
			console.Address = graphics.Addresses[0].Address
		}
		if console.WebSocket > 0 {
			return console, nil
		}
		if fallback.Protocol == "" {
			fallback = console
		}
	}
	return fallback, nil
}

func (l *Live) applyVMRuntime(ctx context.Context, vm *model.VM) {
	memStats := virshSpaceInfo(ctx, "dommemstat", vm.Name)
	usedKiB := firstInt(memStats["rss"])
	if usedKiB <= 0 {
		actual := firstInt(memStats["actual"])
		unused := firstInt(memStats["unused"])
		if actual > unused {
			usedKiB = actual - unused
		}
	}
	if vm.Memory > 0 && usedKiB > 0 {
		used := uint64(usedKiB) * 1024
		vm.MemoryUsage = round1(clampFloat(float64(used)/float64(vm.Memory)*100, 0, 100))
	}

	stats := virshEqualsInfo(ctx, "domstats", vm.Name, "--state", "--cpu-total", "--balloon")
	if cpuTime := parseUint(stats["cpu.time"]); cpuTime > 0 {
		now := time.Now()
		l.statsMu.Lock()
		previous, ok := l.vmCPU[vm.Name]
		l.vmCPU[vm.Name] = vmCPUSample{at: now, cpuTime: cpuTime}
		l.statsMu.Unlock()
		if ok && now.After(previous.at) && cpuTime > previous.cpuTime {
			seconds := now.Sub(previous.at).Seconds()
			vcpus := vm.VCPUs
			if vcpus < 1 {
				vcpus = 1
			}
			used := float64(cpuTime-previous.cpuTime) / 1e9
			vm.CPUUsage = round1(clampFloat(used/seconds/float64(vcpus)*100, 0, 100))
		}
	}
}

func vmDiskUsage(ctx context.Context, name string) (uint64, uint64) {
	out, err := exec.CommandContext(ctx, "virsh", "domblklist", name, "--details").Output()
	if err != nil {
		return 0, 0
	}
	var total, used uint64
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[1] != "disk" {
			continue
		}
		target := fields[2]
		if target == "" {
			continue
		}
		infoOut, err := exec.CommandContext(ctx, "virsh", "domblkinfo", name, target).Output()
		if err != nil {
			continue
		}
		info := virshColonInfoFrom(string(infoOut))
		total += uint64(firstInt(info["Capacity"]))
		used += uint64(firstInt(info["Allocation"]))
	}
	return total, used
}

func virshColonInfo(ctx context.Context, command, name string) map[string]string {
	out, err := exec.CommandContext(ctx, "virsh", command, name).Output()
	if err != nil {
		return map[string]string{}
	}
	return virshColonInfoFrom(string(out))
}

func virshColonInfoFrom(text string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		out[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return out
}

func virshSpaceInfo(ctx context.Context, command, name string, args ...string) map[string]string {
	cmdArgs := append([]string{command, name}, args...)
	out, err := exec.CommandContext(ctx, "virsh", cmdArgs...).Output()
	if err != nil {
		return map[string]string{}
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		values[fields[0]] = fields[1]
	}
	return values
}

func virshEqualsInfo(ctx context.Context, command, name string, args ...string) map[string]string {
	cmdArgs := append([]string{command, name}, args...)
	out, err := exec.CommandContext(ctx, "virsh", cmdArgs...).Output()
	if err != nil {
		return map[string]string{}
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return values
}

func firstInt(value string) int {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(fields[0])
	return n
}

func parseUint(value string) uint64 {
	n, _ := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	return n
}

func clampFloat(value, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, value))
}
