package provider

import (
	"net"
	"os"
	"sort"
	"strings"
)

type containerNetwork struct {
	IPAddress         string `json:"IPAddress"`
	GlobalIPv6Address string `json:"GlobalIPv6Address"`
}

// pickContainerNetwork selects the address shown in the UI. Docker can attach a
// container to several networks; the alphabetically first one with an address
// wins so the result stays stable between snapshots.
func pickContainerNetwork(networks map[string]containerNetwork) (string, string) {
	names := make([]string, 0, len(networks))
	for name := range networks {
		names = append(names, name)
	}
	sort.Strings(names)

	fallback := ""
	for _, name := range names {
		settings := networks[name]
		if settings.IPAddress != "" {
			return name, settings.IPAddress
		}
		if fallback == "" && settings.GlobalIPv6Address != "" {
			fallback = name
		}
	}
	if fallback != "" {
		return fallback, networks[fallback].GlobalIPv6Address
	}
	if len(names) > 0 {
		if names[0] == "host" {
			return names[0], hostIPv4()
		}
		return names[0], ""
	}
	return "", ""
}

// hostIPv4 resolves the address of the default route interface. Containers
// using the host network have no address of their own, so the host address is
// the one users expect to see.
func hostIPv4() string {
	route, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return firstGlobalIPv4()
	}
	for _, line := range strings.Split(string(route), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[1] != "00000000" {
			continue
		}
		if address := interfaceIPv4(fields[0]); address != "" {
			return address
		}
	}
	return firstGlobalIPv4()
}

func interfaceIPv4(name string) string {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return ""
	}
	addresses, err := iface.Addrs()
	if err != nil {
		return ""
	}
	for _, address := range addresses {
		ip, _, err := net.ParseCIDR(address.String())
		if err != nil || ip.To4() == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			continue
		}
		return ip.String()
	}
	return ""
}

func firstGlobalIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if address := interfaceIPv4(iface.Name); address != "" {
			return address
		}
	}
	return ""
}
