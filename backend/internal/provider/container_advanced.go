package provider

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"aurora/internal/model"
)

var (
	containerMacPattern     = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`)
	containerHostPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{0,62}$`)
	containerCapPattern     = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,63}$`)
	containerSysctlPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,127}$`)
	containerLabelPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
	containerUlimitPattern  = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
	containerCPUPattern     = regexp.MustCompile(`^[0-9]+(-[0-9]+)?(,[0-9]+(-[0-9]+)?)*$`)
	containerDNSNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{0,252}$`)
	containerTmpfsPattern   = regexp.MustCompile(`^[A-Za-z0-9=,._-]{0,128}$`)
)

func normalizeContainerAdvanced(advanced *model.ContainerAdvanced) {
	advanced.MacAddress = strings.TrimSpace(advanced.MacAddress)
	advanced.Hostname = strings.TrimSpace(advanced.Hostname)
	advanced.CPUSet = strings.TrimSpace(advanced.CPUSet)
	for index := range advanced.Devices {
		advanced.Devices[index].HostPath = strings.TrimSpace(advanced.Devices[index].HostPath)
		advanced.Devices[index].ContainerPath = strings.TrimSpace(advanced.Devices[index].ContainerPath)
		advanced.Devices[index].Permissions = strings.TrimSpace(advanced.Devices[index].Permissions)
		if advanced.Devices[index].Permissions == "" {
			advanced.Devices[index].Permissions = "rwm"
		}
	}
	advanced.CapAdd = normalizeCapabilities(advanced.CapAdd)
	advanced.CapDrop = normalizeCapabilities(advanced.CapDrop)
	if advanced.Sysctls == nil {
		advanced.Sysctls = map[string]string{}
	}
	advanced.Sysctls = trimStringMap(advanced.Sysctls)
	if advanced.Labels == nil {
		advanced.Labels = map[string]string{}
	}
	advanced.Labels = trimStringMap(advanced.Labels)
	advanced.ExtraHosts = trimStrings(advanced.ExtraHosts)
	advanced.Tmpfs = trimStrings(advanced.Tmpfs)
	advanced.DNS = trimStrings(advanced.DNS)
	advanced.DNSSearch = trimStrings(advanced.DNSSearch)
}

func validateContainerAdvanced(advanced model.ContainerAdvanced) error {
	if advanced.MacAddress != "" && !containerMacPattern.MatchString(advanced.MacAddress) {
		return errors.New("MAC address must use the AA:BB:CC:DD:EE:FF format")
	}
	if advanced.Hostname != "" && !containerHostPattern.MatchString(advanced.Hostname) {
		return errors.New("invalid hostname")
	}
	for _, device := range advanced.Devices {
		if !strings.HasPrefix(device.HostPath, "/dev/") || strings.Contains(device.HostPath, "..") {
			return fmt.Errorf("device %q must be an absolute path under /dev", device.HostPath)
		}
		if !strings.HasPrefix(device.ContainerPath, "/") || strings.Contains(device.ContainerPath, "..") {
			return fmt.Errorf("container device %q must be an absolute path", device.ContainerPath)
		}
		if !regexp.MustCompile(`^[rwm]{1,3}$`).MatchString(device.Permissions) {
			return fmt.Errorf("device permissions %q must contain r, w and/or m", device.Permissions)
		}
	}
	for _, capability := range append(append([]string{}, advanced.CapAdd...), advanced.CapDrop...) {
		if !containerCapPattern.MatchString(capability) {
			return fmt.Errorf("invalid capability %q", capability)
		}
		if capability == "ALL" {
			return errors.New("the ALL capability is blocked; use privileged mode explicitly if required")
		}
	}
	for key, value := range advanced.Sysctls {
		if !containerSysctlPattern.MatchString(key) {
			return fmt.Errorf("invalid sysctl key %q", key)
		}
		if len(value) > 512 || strings.ContainsAny(value, "\x00\n\r") {
			return fmt.Errorf("invalid sysctl value for %q", key)
		}
	}
	for _, ulimit := range advanced.Ulimits {
		if !containerUlimitPattern.MatchString(ulimit.Name) {
			return fmt.Errorf("invalid ulimit name %q", ulimit.Name)
		}
		if ulimit.Soft < 0 || ulimit.Hard < 0 || ulimit.Hard < ulimit.Soft {
			return fmt.Errorf("invalid ulimit values for %q", ulimit.Name)
		}
	}
	for key, value := range advanced.Labels {
		if !containerLabelPattern.MatchString(key) {
			return fmt.Errorf("invalid label key %q", key)
		}
		if strings.HasPrefix(strings.ToLower(key), "net.unraid.") {
			return fmt.Errorf("label %q is reserved by Unraid", key)
		}
		if len(value) > 1024 || strings.ContainsAny(value, "\x00\n\r") {
			return fmt.Errorf("invalid label value for %q", key)
		}
	}
	for _, host := range advanced.ExtraHosts {
		hostName, address, ok := strings.Cut(host, ":")
		if !ok || hostName == "" || address == "" || strings.ContainsAny(host, "\x00\n\r ") {
			return fmt.Errorf("extra host %q must use host:ip", host)
		}
		if address != "host-gateway" && net.ParseIP(address) == nil {
			return fmt.Errorf("extra host %q has an invalid IP address", host)
		}
	}
	for _, tmpfs := range advanced.Tmpfs {
		path, options, _ := strings.Cut(tmpfs, ":")
		if !strings.HasPrefix(path, "/") || strings.Contains(path, "..") {
			return fmt.Errorf("tmpfs %q must be an absolute path", tmpfs)
		}
		if options != "" && !containerTmpfsPattern.MatchString(options) {
			return fmt.Errorf("tmpfs %q has invalid options", tmpfs)
		}
	}
	for _, address := range advanced.DNS {
		if net.ParseIP(address) == nil {
			return fmt.Errorf("invalid DNS address %q", address)
		}
	}
	for _, name := range advanced.DNSSearch {
		if !containerDNSNamePattern.MatchString(name) {
			return fmt.Errorf("invalid DNS search domain %q", name)
		}
	}
	if advanced.MemoryLimit != 0 && (advanced.MemoryLimit < 6<<20 || advanced.MemoryLimit > 1<<40) {
		return errors.New("memory limit must be 0 or between 6 MiB and 1 TiB")
	}
	if advanced.NanoCPUs != 0 && (advanced.NanoCPUs < 100_000_000 || advanced.NanoCPUs > 1_000_000_000_000) {
		return errors.New("CPU limit must be 0 or between 0.1 and 1000 CPUs")
	}
	if advanced.CPUSet != "" && !containerCPUPattern.MatchString(advanced.CPUSet) {
		return errors.New("invalid CPU pinning set")
	}
	if advanced.PidsLimit < 0 || advanced.PidsLimit > 1_000_000 {
		return errors.New("PID limit must be between 0 and 1000000")
	}
	return nil
}

func normalizeCapabilities(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToUpper(strings.TrimSpace(value))
		value = strings.TrimPrefix(value, "CAP_")
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func trimStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func trimStringMap(values map[string]string) map[string]string {
	out := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func applyContainerAdvanced(config, host map[string]any, advanced model.ContainerAdvanced) {
	normalizeContainerAdvanced(&advanced)
	if advanced.Hostname != "" {
		config["Hostname"] = advanced.Hostname
	}
	if advanced.MacAddress != "" {
		config["MacAddress"] = advanced.MacAddress
	}
	if len(advanced.Labels) > 0 {
		labels := map[string]any{}
		for key, value := range childMap(config, "Labels") {
			labels[key] = value
		}
		for key, value := range advanced.Labels {
			labels[key] = value
		}
		config["Labels"] = labels
	}
	if len(advanced.Devices) > 0 {
		devices := make([]any, 0, len(advanced.Devices))
		for _, device := range advanced.Devices {
			devices = append(devices, map[string]any{
				"PathOnHost": device.HostPath, "PathInContainer": device.ContainerPath,
				"CgroupPermissions": device.Permissions,
			})
		}
		host["Devices"] = devices
	}
	if len(advanced.CapAdd) > 0 {
		host["CapAdd"] = advanced.CapAdd
	}
	if len(advanced.CapDrop) > 0 {
		host["CapDrop"] = advanced.CapDrop
	}
	if len(advanced.Sysctls) > 0 {
		host["Sysctls"] = advanced.Sysctls
	}
	if len(advanced.Ulimits) > 0 {
		ulimits := make([]any, 0, len(advanced.Ulimits))
		for _, ulimit := range advanced.Ulimits {
			ulimits = append(ulimits, map[string]any{"Name": ulimit.Name, "Soft": ulimit.Soft, "Hard": ulimit.Hard})
		}
		host["Ulimits"] = ulimits
	}
	if len(advanced.ExtraHosts) > 0 {
		host["ExtraHosts"] = advanced.ExtraHosts
	}
	if len(advanced.DNS) > 0 {
		host["Dns"] = advanced.DNS
	}
	if len(advanced.DNSSearch) > 0 {
		host["DnsSearch"] = advanced.DNSSearch
	}
	if len(advanced.Tmpfs) > 0 {
		tmpfs := map[string]string{}
		for _, value := range advanced.Tmpfs {
			path, options, _ := strings.Cut(value, ":")
			tmpfs[path] = options
		}
		host["Tmpfs"] = tmpfs
	}
	if advanced.MemoryLimit > 0 {
		host["Memory"] = advanced.MemoryLimit
	}
	if advanced.NanoCPUs > 0 {
		host["NanoCpus"] = advanced.NanoCPUs
	}
	if advanced.CPUSet != "" {
		host["CpusetCpus"] = advanced.CPUSet
	}
	if advanced.PidsLimit > 0 {
		host["PidsLimit"] = advanced.PidsLimit
	}
	if advanced.ReadOnlyRootfs {
		host["ReadonlyRootfs"] = true
	}
}

func parseContainerAdvanced(config, host map[string]any) model.ContainerAdvanced {
	advanced := model.ContainerAdvanced{
		MacAddress: stringValue(config["MacAddress"]), Hostname: stringValue(config["Hostname"]),
		Devices: []model.ContainerDevice{}, CapAdd: stringSlice(host["CapAdd"]), CapDrop: stringSlice(host["CapDrop"]),
		Sysctls: map[string]string{}, Ulimits: []model.ContainerUlimit{}, Labels: map[string]string{},
		ExtraHosts: stringSlice(host["ExtraHosts"]), Tmpfs: []string{}, DNS: stringSlice(host["Dns"]),
		DNSSearch: stringSlice(host["DnsSearch"]), CPUSet: stringValue(host["CpusetCpus"]),
		MemoryLimit: int64Value(host["Memory"]), NanoCPUs: int64Value(host["NanoCpus"]),
		PidsLimit: int64Value(host["PidsLimit"]), ReadOnlyRootfs: boolValue(host["ReadonlyRootfs"]),
	}
	for _, raw := range anySlice(host["Devices"]) {
		device := mapValue(raw)
		if device == nil {
			continue
		}
		advanced.Devices = append(advanced.Devices, model.ContainerDevice{
			HostPath: stringValue(device["PathOnHost"]), ContainerPath: stringValue(device["PathInContainer"]),
			Permissions: stringValue(device["CgroupPermissions"]),
		})
	}
	for key, value := range childMap(host, "Sysctls") {
		advanced.Sysctls[key] = stringValue(value)
	}
	for _, raw := range anySlice(host["Ulimits"]) {
		ulimit := mapValue(raw)
		if ulimit == nil {
			continue
		}
		advanced.Ulimits = append(advanced.Ulimits, model.ContainerUlimit{
			Name: stringValue(ulimit["Name"]), Soft: int64Value(ulimit["Soft"]), Hard: int64Value(ulimit["Hard"]),
		})
	}
	for key, value := range childMap(config, "Labels") {
		if strings.HasPrefix(strings.ToLower(key), "net.unraid.") {
			continue
		}
		advanced.Labels[key] = stringValue(value)
	}
	for path, options := range childMap(host, "Tmpfs") {
		if options := stringValue(options); options != "" {
			advanced.Tmpfs = append(advanced.Tmpfs, path+":"+options)
		} else {
			advanced.Tmpfs = append(advanced.Tmpfs, path)
		}
	}
	return advanced
}

func int64Value(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	default:
		return 0
	}
}

func containerAdvancedFromExtraParams(extra string) model.ContainerAdvanced {
	advanced := model.ContainerAdvanced{
		Devices: []model.ContainerDevice{}, CapAdd: []string{}, CapDrop: []string{},
		Sysctls: map[string]string{}, Ulimits: []model.ContainerUlimit{}, Labels: map[string]string{},
		ExtraHosts: []string{}, Tmpfs: []string{}, DNS: []string{}, DNSSearch: []string{},
	}
	tokens := strings.Fields(extra)
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		value := ""
		if key, rest, ok := strings.Cut(token, "="); ok {
			token, value = key, rest
		}
		if value == "" && needsSeparateValue(token) && index+1 < len(tokens) {
			value = tokens[index+1]
			index++
		}
		switch token {
		case "--device":
			parts := strings.Split(value, ":")
			if len(parts) >= 1 {
				device := model.ContainerDevice{HostPath: parts[0], ContainerPath: parts[0], Permissions: "rwm"}
				if len(parts) >= 2 {
					device.ContainerPath = parts[1]
				}
				if len(parts) >= 3 {
					device.Permissions = parts[2]
				}
				advanced.Devices = append(advanced.Devices, device)
			}
		case "--cap-add":
			advanced.CapAdd = append(advanced.CapAdd, strings.Split(value, ",")...)
		case "--cap-drop":
			advanced.CapDrop = append(advanced.CapDrop, strings.Split(value, ",")...)
		case "--sysctl":
			if key, item, ok := strings.Cut(value, "="); ok {
				advanced.Sysctls[key] = item
			}
		case "--ulimit":
			if name, values, ok := strings.Cut(value, "="); ok {
				if soft, hard, ok := strings.Cut(values, ":"); ok {
					advanced.Ulimits = append(advanced.Ulimits, model.ContainerUlimit{Name: name, Soft: int64(atoi(soft)), Hard: int64(atoi(hard))})
				}
			}
		case "--add-host":
			advanced.ExtraHosts = append(advanced.ExtraHosts, value)
		case "--tmpfs":
			advanced.Tmpfs = append(advanced.Tmpfs, value)
		case "--dns":
			advanced.DNS = append(advanced.DNS, value)
		case "--dns-search":
			advanced.DNSSearch = append(advanced.DNSSearch, value)
		case "--mac-address":
			advanced.MacAddress = value
		case "--hostname":
			advanced.Hostname = value
		case "--memory":
			advanced.MemoryLimit = parseDockerMemory(value)
		case "--cpus":
			advanced.NanoCPUs = parseNanoCPUs(value)
		case "--cpuset-cpus":
			advanced.CPUSet = value
		case "--pids-limit":
			advanced.PidsLimit = int64(atoi(value))
		case "--read-only":
			advanced.ReadOnlyRootfs = true
		case "--label":
			if key, item, ok := strings.Cut(value, "="); ok {
				advanced.Labels[key] = item
			}
		}
	}
	return advanced
}

func needsSeparateValue(token string) bool {
	switch token {
	case "--device", "--cap-add", "--cap-drop", "--sysctl", "--ulimit", "--add-host", "--tmpfs",
		"--dns", "--dns-search", "--mac-address", "--hostname", "--memory", "--cpus",
		"--cpuset-cpus", "--pids-limit", "--label":
		return true
	default:
		return false
	}
}

func mergeExtraParams(existing string, advanced model.ContainerAdvanced, restartPolicy string) string {
	managed := map[string]bool{
		"--device": true, "--cap-add": true, "--cap-drop": true, "--sysctl": true, "--ulimit": true,
		"--add-host": true, "--tmpfs": true, "--dns": true, "--dns-search": true, "--mac-address": true,
		"--hostname": true, "--memory": true, "--cpus": true, "--cpuset-cpus": true, "--pids-limit": true,
		"--read-only": true, "--label": true,
	}
	tokens := strings.Fields(existing)
	kept := make([]string, 0, len(tokens))
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		key := token
		if before, _, ok := strings.Cut(token, "="); ok {
			key = before
		}
		if managed[key] {
			if !strings.Contains(token, "=") && key != "--read-only" && index+1 < len(tokens) {
				index++
			}
			continue
		}
		kept = append(kept, token)
	}
	for _, device := range advanced.Devices {
		kept = append(kept, "--device="+device.HostPath+":"+device.ContainerPath+":"+device.Permissions)
	}
	for _, capability := range advanced.CapAdd {
		kept = append(kept, "--cap-add="+capability)
	}
	for _, capability := range advanced.CapDrop {
		kept = append(kept, "--cap-drop="+capability)
	}
	for key, value := range advanced.Sysctls {
		kept = append(kept, "--sysctl="+key+"="+value)
	}
	for _, ulimit := range advanced.Ulimits {
		kept = append(kept, fmt.Sprintf("--ulimit=%s=%d:%d", ulimit.Name, ulimit.Soft, ulimit.Hard))
	}
	for _, host := range advanced.ExtraHosts {
		kept = append(kept, "--add-host="+host)
	}
	for _, tmpfs := range advanced.Tmpfs {
		kept = append(kept, "--tmpfs="+tmpfs)
	}
	for _, address := range advanced.DNS {
		kept = append(kept, "--dns="+address)
	}
	for _, name := range advanced.DNSSearch {
		kept = append(kept, "--dns-search="+name)
	}
	if advanced.MacAddress != "" {
		kept = append(kept, "--mac-address="+advanced.MacAddress)
	}
	if advanced.Hostname != "" {
		kept = append(kept, "--hostname="+advanced.Hostname)
	}
	if advanced.NanoCPUs > 0 {
		kept = append(kept, fmt.Sprintf("--cpus=%g", float64(advanced.NanoCPUs)/1_000_000_000))
	}
	if advanced.PidsLimit > 0 {
		kept = append(kept, "--pids-limit="+strconv.FormatInt(advanced.PidsLimit, 10))
	}
	if advanced.ReadOnlyRootfs {
		kept = append(kept, "--read-only")
	}
	for key, value := range advanced.Labels {
		kept = append(kept, "--label="+key+"="+shellQuote(value))
	}
	return applyRestartPolicy(strings.Join(kept, " "), restartPolicy)
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " \t'\"\\$`") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func parseDockerMemory(value string) int64 {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return 0
	}
	multiplier := int64(1)
	switch value[len(value)-1] {
	case 'b':
		value = value[:len(value)-1]
	case 'k':
		multiplier = 1 << 10
		value = value[:len(value)-1]
	case 'm':
		multiplier = 1 << 20
		value = value[:len(value)-1]
	case 'g':
		multiplier = 1 << 30
		value = value[:len(value)-1]
	case 't':
		multiplier = 1 << 40
		value = value[:len(value)-1]
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || number < 0 {
		return 0
	}
	return int64(number * float64(multiplier))
}

func formatDockerMemory(value int64) string {
	if value <= 0 {
		return ""
	}
	if value%(1<<30) == 0 {
		return fmt.Sprintf("%dg", value/(1<<30))
	}
	if value%(1<<20) == 0 {
		return fmt.Sprintf("%dm", value/(1<<20))
	}
	return strconv.FormatInt(value, 10)
}

func parseNanoCPUs(value string) int64 {
	number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || number < 0 {
		return 0
	}
	return int64(number * 1_000_000_000)
}
