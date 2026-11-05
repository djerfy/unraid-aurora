package provider

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"aurora/internal/model"
)

var networkInterfacesDir = "/sys/class/net"

// ErrNetworkLockout means the new static configuration would leave the client
// that submitted it outside of the server subnet.
var ErrNetworkLockout = errors.New("the new configuration would make this client unreachable")

var (
	networkKVPattern  = regexp.MustCompile(`^\s*([A-Za-z][A-Za-z0-9_]*(?:\[\d+\]|:\d+)?)="(.*)"\s*$`)
	networkNameFormat = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,14}$`)
	networkIndexedKey = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_]*)\[(\d+)\]$`)
)

type networkLine struct {
	raw   string
	key   string
	value string
	isKV  bool
}

// networkNICSource is a variable so tests do not depend on the host NICs.
var networkNICSource = sortedNetworkNICs

// NetworkConfig reads every interface stored in network.cfg.
func (l *Live) NetworkConfig(ctx context.Context) (model.NetworkConfig, error) {
	_ = ctx
	path := l.networkConfigPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return model.NetworkConfig{}, fmt.Errorf("read network configuration: %w", err)
	}
	return decodeNetworkConfig(parseNetworkLines(string(raw))), nil
}

func decodeNetworkConfig(lines []networkLine) model.NetworkConfig {
	indexed := networkIsIndexed(lines)
	cfg := model.NetworkConfig{AvailableNICs: networkNICSource(), DNS: networkDNS(lines)}
	if indexed {
		cfg.Format = "indexed"
		for _, index := range networkIndices(lines) {
			iface := networkInterfaceAt(lines, index, true)
			cfg.Interfaces = append(cfg.Interfaces, iface)
			if iface.Bonding && iface.BondName != "" && !containsString(cfg.AvailableNICs, iface.BondName) {
				cfg.AvailableNICs = append(cfg.AvailableNICs, iface.BondName)
			}
		}
	} else {
		cfg.Format = "legacy"
		cfg.Interfaces = []model.NetworkInterfaceConfig{networkInterfaceAt(lines, 0, false)}
	}
	if len(cfg.Interfaces) == 0 {
		cfg.Interfaces = []model.NetworkInterfaceConfig{networkInterfaceAt(lines, 0, indexed)}
	}
	sort.Slice(cfg.Interfaces, func(i, j int) bool { return cfg.Interfaces[i].Index < cfg.Interfaces[j].Index })
	sort.Strings(cfg.AvailableNICs)
	return cfg
}

// SaveNetworkConfig validates and writes network.cfg without touching the live
// interfaces. Applying requires an explicit separate action.
func (l *Live) SaveNetworkConfig(ctx context.Context, cfg model.NetworkConfig, clientIP string, acknowledge bool) (model.NetworkConfig, error) {
	_ = ctx
	if err := validateNetworkConfig(cfg); err != nil {
		return cfg, err
	}
	path := l.networkConfigPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read network configuration: %w", err)
	}
	lines := parseNetworkLines(string(raw))
	indexed := networkIsIndexed(lines)
	if !indexed && len(cfg.Interfaces) > 1 {
		return cfg, errors.New("this flash drive uses the legacy single-interface format")
	}
	oldConfig := decodeNetworkConfig(lines)
	anyRisk := false
	for index := range cfg.Interfaces {
		oldInterface := findNetworkInterface(oldConfig.Interfaces, cfg.Interfaces[index].Index)
		cfg.Interfaces[index].LockoutRisk = networkInterfaceLockoutRisk(oldInterface, cfg.Interfaces[index], clientIP)
		if cfg.Interfaces[index].LockoutRisk {
			anyRisk = true
		}
	}
	if anyRisk && !acknowledge {
		return cfg, ErrNetworkLockout
	}

	for index := range cfg.Interfaces {
		iface := &cfg.Interfaces[index]
		name := networkInterfaceName(*iface)
		iface.Interface = name
		set := func(base, value string) {
			key := base
			if indexed {
				key = fmt.Sprintf("%s[%d]", base, iface.Index)
			}
			lines = setNetworkValue(lines, key, value)
		}
		if indexed {
			set("IFNAME", name)
		}
		set("USE_DHCP", yesNo(iface.UseDHCP))
		set("IPADDR", strings.TrimSpace(iface.IPAddr))
		set("NETMASK", strings.TrimSpace(iface.Netmask))
		set("GATEWAY", strings.TrimSpace(iface.Gateway))
		set("USE_MTU", yesNo(iface.UseMTU))
		set("MTU", strconv.Itoa(iface.MTU))
		set("BONDING", yesNo(iface.Bonding))
		if iface.Bonding {
			set("BONDNAME", firstNonEmpty(iface.BondName, "bond0"))
			set("BONDNICS", strings.Join(iface.BondNICs, ","))
			set("BONDING_MODE", firstNonEmpty(iface.BondMode, "1"))
			set("BONDING_MIIMON", strconv.Itoa(iface.BondMIIMon))
		}
		set("BRIDGING", yesNo(iface.Bridging))
		if iface.Bridging {
			set("BRNAME", firstNonEmpty(iface.BridgeName, "br0"))
			set("BRSTP", boolNumber(iface.BridgeSTP))
			set("BRFD", strconv.Itoa(iface.BridgeFD))
			if indexed {
				set("BRNICS", strings.Join(iface.BridgeNICs, ","))
			} else if _, ok := networkValue(lines, "BRNIC"); ok {
				set("BRNIC", strings.Join(iface.BridgeNICs, " "))
				lines = removeNetworkKey(lines, "BRNICS")
			} else {
				set("BRNICS", strings.Join(iface.BridgeNICs, " "))
				lines = removeNetworkKey(lines, "BRNIC")
			}
		}
	}

	// Unraid resolves DHCP DNS automatically; keep the static values so users
	// can switch back without retyping them.
	if _, ok := networkValue(lines, "DNS_SERVER1"); ok || !hasNetworkKey(lines, "DNS1") {
		for index := 1; index <= 3; index++ {
			value := ""
			if index-1 < len(cfg.DNS) {
				value = strings.TrimSpace(cfg.DNS[index-1])
			}
			lines = setNetworkValue(lines, "DNS_SERVER"+strconv.Itoa(index), value)
		}
	} else {
		for index := 1; index <= 2; index++ {
			value := ""
			if index-1 < len(cfg.DNS) {
				value = strings.TrimSpace(cfg.DNS[index-1])
			}
			lines = setNetworkValue(lines, "DNS"+strconv.Itoa(index), value)
		}
	}
	if indexed {
		maxIndex := 0
		for _, iface := range cfg.Interfaces {
			if iface.Index > maxIndex {
				maxIndex = iface.Index
			}
		}
		if current := networkSysNICs(lines); maxIndex+1 > current {
			lines = setNetworkValue(lines, "SYSNICS", strconv.Itoa(maxIndex+1))
		}
	}

	backup, err := l.backupNetworkConfig(raw)
	if err != nil {
		return cfg, err
	}
	updated := renderNetworkLines(lines)
	if err := writeFileAtomic(path, []byte(updated), 0o644); err != nil {
		return cfg, fmt.Errorf("write network configuration: %w", err)
	}
	cfg.Backup = backup
	cfg.Format = map[bool]string{true: "indexed", false: "legacy"}[indexed]
	for index := range cfg.Interfaces {
		cfg.Interfaces[index].State = readNetworkInterfaceState(cfg.Interfaces[index].Interface)
	}
	return cfg, nil
}

// RevertNetworkConfig restores the most recent Aurora backup.
func (l *Live) RevertNetworkConfig(ctx context.Context) (string, error) {
	_ = ctx
	backups, err := filepath.Glob(l.networkConfigPath() + ".aurora-*.bak")
	if err != nil {
		return "", err
	}
	if len(backups) == 0 {
		return "", errors.New("no network configuration backup is available")
	}
	sort.Strings(backups)
	latest := backups[len(backups)-1]
	raw, err := os.ReadFile(latest)
	if err != nil {
		return "", err
	}
	if err := writeFileAtomic(l.networkConfigPath(), raw, 0o644); err != nil {
		return "", err
	}
	return filepath.Base(latest), nil
}

// ApplyNetworkConfig restarts networking in the background. A short delay lets
// the HTTP response reach the browser before the interface goes down.
func (l *Live) ApplyNetworkConfig(ctx context.Context) error {
	_ = ctx
	script := "/etc/rc.d/rc.inet1"
	if info, err := os.Stat(script); err != nil || info.IsDir() {
		return errors.New("the Unraid network service is not available")
	}
	// #nosec G204 -- script is the fixed rc.inet1 path; the name is validated elsewhere.
	command := exec.Command("/bin/sh", "-c", "sleep 3; "+script+" restart >/tmp/aurora-network-apply.log 2>&1")
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

func (l *Live) networkConfigPath() string {
	return filepath.Join(l.bootPath(), "config", "network.cfg")
}

func (l *Live) backupNetworkConfig(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	name := fmt.Sprintf("network.cfg.aurora-%s.bak", time.Now().Format("20060102-150405"))
	path := filepath.Join(l.bootPath(), "config", name)
	if err := writeFileAtomic(path, raw, 0o644); err != nil {
		return "", fmt.Errorf("back up network configuration: %w", err)
	}
	backups, _ := filepath.Glob(l.networkConfigPath() + ".aurora-*.bak")
	sort.Strings(backups)
	for len(backups) > 10 {
		_ = os.Remove(backups[0])
		backups = backups[1:]
	}
	return name, nil
}

func networkIsIndexed(lines []networkLine) bool {
	for _, line := range lines {
		if line.isKV && (line.key == "SYSNICS" || strings.HasPrefix(line.key, "IFNAME[")) {
			return true
		}
	}
	return false
}

func networkSysNICs(lines []networkLine) int {
	if value, ok := networkValue(lines, "SYSNICS"); ok {
		count, _ := strconv.Atoi(strings.TrimSpace(value))
		return count
	}
	return 0
}

func networkIndices(lines []networkLine) []int {
	seen := map[int]bool{}
	for _, line := range lines {
		if !line.isKV {
			continue
		}
		if matches := networkIndexedKey.FindStringSubmatch(line.key); len(matches) == 3 && matches[1] == "IFNAME" {
			if index, err := strconv.Atoi(matches[2]); err == nil && index >= 0 {
				seen[index] = true
			}
		}
	}
	if len(seen) == 0 {
		// Older indexed files may omit IFNAME; fall back to every indexed key,
		// then to SYSNICS so the primary interface is still editable.
		for _, line := range lines {
			if !line.isKV {
				continue
			}
			if matches := networkIndexedKey.FindStringSubmatch(line.key); len(matches) == 3 {
				if index, err := strconv.Atoi(matches[2]); err == nil && index >= 0 {
					seen[index] = true
				}
			}
		}
	}
	if len(seen) == 0 {
		count := networkSysNICs(lines)
		if count == 0 {
			count = 1
		}
		for index := 0; index < count; index++ {
			seen[index] = true
		}
	}
	indices := make([]int, 0, len(seen))
	for index := range seen {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	return indices
}

func networkInterfaceAt(lines []networkLine, index int, indexed bool) model.NetworkInterfaceConfig {
	get := func(base string) string {
		if indexed {
			for _, key := range []string{fmt.Sprintf("%s[%d]", base, index), fmt.Sprintf("%s:%d", base, index)} {
				if value, ok := networkValue(lines, key); ok {
					return strings.TrimSpace(value)
				}
			}
			// Indexed entries must not inherit another interface's value.
			return ""
		}
		value, _ := networkValue(lines, base)
		return strings.TrimSpace(value)
	}
	cfg := model.NetworkInterfaceConfig{Index: index}
	cfg.UseMTU = parseYes(get("USE_MTU"))
	cfg.MTU, _ = strconv.Atoi(get("MTU"))
	if cfg.MTU == 0 {
		cfg.MTU = 1500
	}
	if raw := get("USE_MTU"); raw == "" {
		cfg.UseMTU = cfg.MTU > 0 && cfg.MTU != 1500
	}
	cfg.Bonding = parseYes(get("BONDING"))
	cfg.BondName = get("BONDNAME")
	cfg.BondNICs = splitNICList(get("BONDNICS"))
	cfg.BondMode = get("BONDING_MODE")
	if cfg.BondMode == "" {
		cfg.BondMode = "1"
	}
	cfg.BondMIIMon, _ = strconv.Atoi(get("BONDING_MIIMON"))
	if cfg.BondMIIMon == 0 {
		cfg.BondMIIMon = 100
	}
	cfg.Bridging = parseYes(get("BRIDGING"))
	cfg.BridgeName = get("BRNAME")
	cfg.BridgeNICs = splitNICList(get("BRNICS"))
	if len(cfg.BridgeNICs) == 0 {
		cfg.BridgeNICs = splitNICList(get("BRNIC"))
	}
	rawSTP := get("BRSTP")
	cfg.BridgeSTP = parseYes(rawSTP) || rawSTP == "1"
	cfg.BridgeFD, _ = strconv.Atoi(get("BRFD"))
	cfg.IPAddr = get("IPADDR")
	cfg.Netmask = get("NETMASK")
	cfg.Gateway = get("GATEWAY")
	rawDHCP := get("USE_DHCP")
	cfg.UseDHCP = parseYes(rawDHCP)
	if rawDHCP == "" {
		cfg.UseDHCP = cfg.IPAddr == ""
	}
	cfg.Interface = get("IFNAME")
	if cfg.Interface == "" {
		switch {
		case cfg.Bridging && cfg.BridgeName != "":
			cfg.Interface = cfg.BridgeName
		case cfg.Bonding && cfg.BondName != "":
			cfg.Interface = cfg.BondName
		default:
			cfg.Interface = fmt.Sprintf("eth%d", index)
		}
	}
	if len(cfg.BondNICs) > 0 {
		cfg.PhysicalNIC = cfg.BondNICs[0]
	} else if len(cfg.BridgeNICs) > 0 {
		cfg.PhysicalNIC = cfg.BridgeNICs[0]
	} else if strings.HasPrefix(cfg.Interface, "eth") {
		cfg.PhysicalNIC = cfg.Interface
	}
	cfg.State = readNetworkInterfaceState(networkInterfaceName(cfg))
	return cfg
}

func networkDNS(lines []networkLine) []string {
	out := make([]string, 0, 3)
	for index := 1; index <= 3; index++ {
		if value, ok := networkValue(lines, "DNS_SERVER"+strconv.Itoa(index)); ok && strings.TrimSpace(value) != "" {
			out = append(out, strings.TrimSpace(value))
		}
	}
	if len(out) == 0 {
		for index := 1; index <= 2; index++ {
			if value, ok := networkValue(lines, "DNS"+strconv.Itoa(index)); ok && strings.TrimSpace(value) != "" {
				out = append(out, strings.TrimSpace(value))
			}
		}
	}
	return out
}

func validateNetworkConfig(cfg model.NetworkConfig) error {
	if len(cfg.Interfaces) == 0 {
		return errors.New("no network interface is configured")
	}
	if cfg.Format == "legacy" && len(cfg.Interfaces) > 1 {
		return errors.New("this flash drive uses the legacy single-interface format")
	}
	seenIndex := map[int]bool{}
	seenName := map[string]bool{}
	for _, iface := range cfg.Interfaces {
		if seenIndex[iface.Index] {
			return fmt.Errorf("duplicate interface index %d", iface.Index)
		}
		seenIndex[iface.Index] = true
		if err := validateNetworkInterface(cfg, iface); err != nil {
			return fmt.Errorf("%s: %w", networkInterfaceName(iface), err)
		}
		name := networkInterfaceName(iface)
		if seenName[name] {
			return fmt.Errorf("duplicate interface name %q", name)
		}
		seenName[name] = true
	}
	for _, server := range cfg.DNS {
		server = strings.TrimSpace(server)
		if server == "" {
			continue
		}
		if net.ParseIP(server) == nil {
			return fmt.Errorf("DNS server %q is invalid", server)
		}
	}
	return nil
}

func validateNetworkInterface(cfg model.NetworkConfig, iface model.NetworkInterfaceConfig) error {
	if !iface.UseDHCP {
		address := net.ParseIP(strings.TrimSpace(iface.IPAddr)).To4()
		if address == nil || !usableIPv4(address) {
			return errors.New("IP address must be a valid IPv4 address")
		}
		maskIP := net.ParseIP(strings.TrimSpace(iface.Netmask)).To4()
		if maskIP == nil {
			return errors.New("subnet mask must be a valid IPv4 mask")
		}
		mask := net.IPMask(maskIP)
		if ones, bits := mask.Size(); ones == 0 || bits != 32 {
			return errors.New("subnet mask must be contiguous")
		}
		if gateway := strings.TrimSpace(iface.Gateway); gateway != "" {
			parsed := net.ParseIP(gateway).To4()
			if parsed == nil || !usableIPv4(parsed) {
				return errors.New("gateway must be a valid IPv4 address")
			}
			if !address.Mask(mask).Equal(parsed.Mask(mask)) {
				return errors.New("gateway must belong to the same subnet as the server")
			}
		}
	}
	if iface.UseMTU && (iface.MTU < 576 || iface.MTU > 9000) {
		return errors.New("MTU must be between 576 and 9000")
	}
	if iface.Bonding {
		if strings.TrimSpace(iface.BondName) == "" {
			return errors.New("a bond name is required when bonding is enabled")
		}
		if !networkNameFormat.MatchString(iface.BondName) {
			return errors.New("bond name is invalid")
		}
		if len(iface.BondNICs) == 0 {
			return errors.New("select at least one interface for the bond")
		}
	}
	if iface.Bridging {
		if strings.TrimSpace(iface.BridgeName) == "" {
			return errors.New("a bridge name is required when bridging is enabled")
		}
		if !networkNameFormat.MatchString(iface.BridgeName) {
			return errors.New("bridge name is invalid")
		}
		if len(iface.BridgeNICs) == 0 {
			return errors.New("select at least one interface for the bridge")
		}
	}
	seen := map[string]bool{}
	for _, nic := range append(append([]string{}, iface.BondNICs...), iface.BridgeNICs...) {
		nic = strings.TrimSpace(nic)
		if nic == "" || seen[nic] {
			continue
		}
		seen[nic] = true
		if !networkNameFormat.MatchString(nic) {
			return fmt.Errorf("interface name %q is invalid", nic)
		}
		if iface.Bridging && nic == iface.BridgeName {
			return errors.New("the bridge cannot contain itself")
		}
		if len(cfg.AvailableNICs) > 0 && !containsString(cfg.AvailableNICs, nic) && !(iface.Bonding && nic == iface.BondName) {
			return fmt.Errorf("interface %q does not exist on this server", nic)
		}
	}
	if iface.Bonding && iface.Bridging && !containsString(iface.BridgeNICs, iface.BondName) && len(iface.BridgeNICs) > 0 {
		return errors.New("the bridge must include the bond interface when both are enabled")
	}
	return nil
}

// SetNetworkInterfaceState brings a configured interface up or down. Down is
// delayed so the HTTP response can leave the host before the link disappears.
func (l *Live) SetNetworkInterfaceState(ctx context.Context, name string, up bool) error {
	name = strings.TrimSpace(name)
	if !networkNameFormat.MatchString(name) {
		return errors.New("invalid interface name")
	}
	cfg, err := l.NetworkConfig(ctx)
	if err != nil {
		return err
	}
	configured := false
	for _, iface := range cfg.Interfaces {
		if networkInterfaceName(iface) == name {
			configured = true
			break
		}
	}
	if !configured {
		return fmt.Errorf("interface %q is not configured", name)
	}
	if _, err := net.InterfaceByName(name); err != nil {
		return fmt.Errorf("interface %q is not present on this server", name)
	}
	if up {
		runCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		output, err := exec.CommandContext(runCtx, "ip", "link", "set", "dev", name, "up").CombinedOutput()
		if err != nil {
			return fmt.Errorf("bring %s up: %w (%s)", name, err, strings.TrimSpace(string(output)))
		}
		return nil
	}
	// #nosec G204 -- name is restricted by networkNameFormat above.
	command := exec.Command("/bin/sh", "-c", "sleep 2; ip link set dev "+name+" down >/tmp/aurora-interface.log 2>&1")
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

func readNetworkInterfaceState(name string) string {
	if name == "" {
		return "unknown"
	}
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return "unknown"
	}
	if raw, err := os.ReadFile(filepath.Join(networkInterfacesDir, name, "operstate")); err == nil {
		switch strings.TrimSpace(string(raw)) {
		case "up":
			return "up"
		case "down":
			return "down"
		}
	}
	if iface.Flags&net.FlagUp != 0 {
		return "up"
	}
	return "down"
}

func networkInterfaceLockoutRisk(old, updated model.NetworkInterfaceConfig, clientIP string) bool {
	if updated.UseDHCP {
		return false
	}
	client := net.ParseIP(strings.TrimSpace(clientIP)).To4()
	if client == nil || client.IsLoopback() {
		return false
	}
	address := net.ParseIP(strings.TrimSpace(updated.IPAddr)).To4()
	maskIP := net.ParseIP(strings.TrimSpace(updated.Netmask)).To4()
	if address == nil || maskIP == nil {
		return true
	}
	mask := net.IPMask(maskIP)
	if ones, bits := mask.Size(); ones == 0 || bits != 32 {
		return true
	}
	if address.Mask(mask).Equal(client.Mask(mask)) {
		return false
	}
	// The client is only at risk if the previous interface already served its
	// subnet, or if this is the primary interface carrying the default route.
	oldAddress := net.ParseIP(strings.TrimSpace(old.IPAddr)).To4()
	oldMaskIP := net.ParseIP(strings.TrimSpace(old.Netmask)).To4()
	if oldAddress != nil && oldMaskIP != nil {
		oldMask := net.IPMask(oldMaskIP)
		if ones, bits := oldMask.Size(); ones != 0 && bits == 32 {
			return oldAddress.Mask(oldMask).Equal(client.Mask(oldMask))
		}
	}
	return updated.Index == 0
}

func networkInterfaceName(iface model.NetworkInterfaceConfig) string {
	switch {
	case iface.Bridging && iface.BridgeName != "":
		return iface.BridgeName
	case iface.Bonding && iface.BondName != "":
		return iface.BondName
	case iface.PhysicalNIC != "":
		return iface.PhysicalNIC
	case iface.Interface != "":
		return iface.Interface
	default:
		return fmt.Sprintf("eth%d", iface.Index)
	}
}

func findNetworkInterface(interfaces []model.NetworkInterfaceConfig, index int) model.NetworkInterfaceConfig {
	for _, iface := range interfaces {
		if iface.Index == index {
			return iface
		}
	}
	return model.NetworkInterfaceConfig{Index: index}
}

func parseNetworkLines(raw string) []networkLine {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	lines := make([]networkLine, 0, strings.Count(raw, "\n")+1)
	for _, text := range strings.Split(raw, "\n") {
		line := networkLine{raw: text}
		if matches := networkKVPattern.FindStringSubmatch(text); len(matches) == 3 {
			line.key, line.value, line.isKV = matches[1], matches[2], true
		}
		lines = append(lines, line)
	}
	return lines
}

func renderNetworkLines(lines []networkLine) string {
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		if line.isKV {
			rendered = append(rendered, fmt.Sprintf(`%s="%s"`, line.key, line.value))
		} else {
			rendered = append(rendered, line.raw)
		}
	}
	for len(rendered) > 0 && rendered[len(rendered)-1] == "" {
		rendered = rendered[:len(rendered)-1]
	}
	return strings.Join(rendered, "\n") + "\n"
}

func networkValue(lines []networkLine, key string) (string, bool) {
	for _, line := range lines {
		if line.isKV && line.key == key {
			return line.value, true
		}
	}
	return "", false
}

func hasNetworkKey(lines []networkLine, key string) bool {
	_, ok := networkValue(lines, key)
	return ok
}

func setNetworkValue(lines []networkLine, key, value string) []networkLine {
	updated := false
	for index := range lines {
		if lines[index].isKV && lines[index].key == key {
			lines[index].value = value
			lines[index].raw = fmt.Sprintf(`%s="%s"`, key, value)
			updated = true
		}
	}
	if updated {
		return lines
	}
	newLine := networkLine{key: key, value: value, isKV: true, raw: fmt.Sprintf(`%s="%s"`, key, value)}
	for index, line := range lines {
		if line.isKV && line.key == "SYSNICS" {
			lines = append(lines[:index], append([]networkLine{newLine}, lines[index:]...)...)
			return lines
		}
	}
	return append(lines, newLine)
}

func removeNetworkKey(lines []networkLine, key string) []networkLine {
	out := lines[:0]
	for _, line := range lines {
		if line.isKV && line.key == key {
			continue
		}
		out = append(out, line)
	}
	return out
}

func writeFileAtomic(path string, raw []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".aurora-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err := temporary.Write(raw); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func sortedNetworkNICs() []string {
	entries, err := os.ReadDir(networkInterfacesDir)
	if err != nil {
		return nil
	}
	out := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "eth") || strings.HasPrefix(name, "bond") {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func splitNICList(raw string) []string {
	fields := strings.FieldsFunc(raw, func(char rune) bool {
		return char == ',' || char == ' ' || char == '\t'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if field = strings.TrimSpace(field); field != "" {
			out = append(out, field)
		}
	}
	return out
}

func parseYes(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "yes", "on", "true", "1":
		return true
	default:
		return false
	}
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func boolNumber(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

func usableIPv4(address net.IP) bool {
	if address == nil || address.IsLoopback() || address.IsUnspecified() || address.IsMulticast() || address.IsLinkLocalUnicast() {
		return false
	}
	return address[0] != 0
}
