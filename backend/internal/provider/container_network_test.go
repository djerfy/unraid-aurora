package provider

import "testing"

func TestPickContainerNetwork(t *testing.T) {
	networks := map[string]containerNetwork{
		"proxynet": {IPAddress: "192.168.1.42"},
		"bridge":   {IPAddress: "172.17.0.5"},
	}
	name, ip := pickContainerNetwork(networks)
	if name != "bridge" || ip != "172.17.0.5" {
		t.Fatalf("bridge should win alphabetically: %q %q", name, ip)
	}

	networks = map[string]containerNetwork{
		"bridge":   {},
		"proxynet": {IPAddress: "192.168.1.42"},
	}
	name, ip = pickContainerNetwork(networks)
	if name != "proxynet" || ip != "192.168.1.42" {
		t.Fatalf("addressable network should win: %q %q", name, ip)
	}

	networks = map[string]containerNetwork{
		"v6net": {GlobalIPv6Address: "fd00::2"},
	}
	name, ip = pickContainerNetwork(networks)
	if name != "v6net" || ip != "fd00::2" {
		t.Fatalf("ipv6 fallback: %q %q", name, ip)
	}

	networks = map[string]containerNetwork{"custom": {}}
	name, ip = pickContainerNetwork(networks)
	if name != "custom" || ip != "" {
		t.Fatalf("network without address: %q %q", name, ip)
	}

	if name, ip := pickContainerNetwork(nil); name != "" || ip != "" {
		t.Fatalf("no networks: %q %q", name, ip)
	}
}
