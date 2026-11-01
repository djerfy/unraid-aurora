package provider

import (
	"context"
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
		l.applyVMRuntime(ctx, &vm)
	}
	vm.DiskSize, vm.DiskUsed = vmDiskUsage(ctx, name)
	return vm
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
