package provider

import (
	"net"
	"testing"
)

func TestParseSSOutput(t *testing.T) {
	raw := `Netid State  Recv-Q Send-Q Local Address:Port Peer Address:Port Process
tcp   ESTAB  0      0      172.17.0.2:54544  142.250.75.238:443 users:(("node",pid=1234,fd=22))
tcp   ESTAB  0      0      172.17.0.2:54545  192.168.1.10:8080 users:(("node",pid=1234,fd=23))
tcp   LISTEN 0      4096   0.0.0.0:8080       0.0.0.0:*         users:(("python",pid=99,fd=3))
udp   ESTAB  0      0      172.17.0.2:50000  1.1.1.1:53 users:(("app",pid=77,fd=4))
tcp6  ESTAB  0      0      [::ffff:172.17.0.2]:54546 [2606:4700:4700::1111]:443 users:(("app",pid=88,fd=5))
tcp   ESTAB  0      0      172.17.0.2:54547  8.8.8.8:53 users:(("app",pid=66,fd=6))`
	connections := parseSSOutput(raw, nil)
	if len(connections) != 5 {
		t.Fatalf("connections = %+v, want 5 remote connections", connections)
	}
	first := connections[0]
	if first.RemoteAddress != "1.1.1.1" || first.RemotePort != 53 || first.Process != "app" || first.PID != 77 {
		t.Fatalf("unexpected first connection: %+v", first)
	}
}

func TestParseSSOutputFiltersHostPIDs(t *testing.T) {
	raw := `tcp ESTAB 0 0 172.17.0.2:54544 142.250.75.238:443 users:(("node",pid=1234,fd=22))
tcp ESTAB 0 0 172.17.0.2:54547 8.8.8.8:53 users:(("other",pid=66,fd=6))`
	connections := parseSSOutput(raw, map[int]bool{1234: true})
	if len(connections) != 1 || connections[0].PID != 1234 {
		t.Fatalf("PID filter failed: %+v", connections)
	}
}

func TestParseSSAddress(t *testing.T) {
	cases := []struct {
		input string
		host  string
		port  int
	}{
		{"172.17.0.2:54544", "172.17.0.2", 54544},
		{"172.17.0.2%eth0:54544", "172.17.0.2", 54544},
		{"[2606:4700:4700::1111]:443", "2606:4700:4700::1111", 443},
		{"*:22", "*", 22},
	}
	for _, tc := range cases {
		host, port := parseSSAddress(tc.input)
		if host != tc.host || port != tc.port {
			t.Errorf("parseSSAddress(%q) = %q:%d, want %q:%d", tc.input, host, port, tc.host, tc.port)
		}
	}
}

func TestIsRemoteAddress(t *testing.T) {
	cases := map[string]bool{
		"8.8.8.8":         true,
		"1.1.1.1":         true,
		"2606:4700::1111": true,
		"10.0.0.5":        true,
		"172.16.4.2":      true,
		"192.168.1.10":    true,
		"127.0.0.1":       true,
		"::1":             true,
		"169.254.1.1":     false,
		"0.0.0.0":         false,
		"fe80::1":         false,
		"ff02::1":         false,
	}
	for value, want := range cases {
		if got := isRemoteAddress(net.ParseIP(value)); got != want {
			t.Errorf("isRemoteAddress(%s) = %v, want %v", value, got, want)
		}
	}
}

func TestIsPublicAddress(t *testing.T) {
	for _, value := range []string{"8.8.8.8", "2606:4700::1111"} {
		if !isPublicAddress(net.ParseIP(value)) {
			t.Errorf("%s should be public", value)
		}
	}
	for _, value := range []string{"10.0.0.5", "192.168.1.10", "127.0.0.1", "::1"} {
		if isPublicAddress(net.ParseIP(value)) {
			t.Errorf("%s should not be public", value)
		}
	}
}

func TestParseGeoLookup(t *testing.T) {
	country, city := parseGeoLookup("GeoIP City Edition, Rev 1: US, California, San Francisco, 94107, 37.7749, -122.4194, 807, 0\n")
	if country != "US" || city != "San Francisco" {
		t.Fatalf("city lookup = %q/%q", country, city)
	}
	country, city = parseGeoLookup("GeoIP Country Edition: DE, Germany\n")
	if country != "DE" || city != "" {
		t.Fatalf("country lookup = %q/%q", country, city)
	}
	country, city = parseGeoLookup("GeoIP Country Edition: IP Address not found\n")
	if country != "" || city != "" {
		t.Fatalf("missing lookup = %q/%q", country, city)
	}
}

func TestParseSSProcess(t *testing.T) {
	name, pid := parseSSProcess(`users:(("node",pid=1234,fd=22))`)
	if name != "node" || pid != 1234 {
		t.Fatalf("parseSSProcess = %q/%d", name, pid)
	}
}

func TestParseSSOutputDirection(t *testing.T) {
	raw := `tcp   ESTAB  0 0 172.17.0.2:8080  203.0.113.4:55000 users:(("server",pid=1,fd=1))
tcp   LISTEN 0 4096 0.0.0.0:8080 0.0.0.0:* users:(("server",pid=1,fd=2))
tcp   ESTAB  0 0 172.17.0.2:55001 1.1.1.1:443 users:(("app",pid=2,fd=3))`
	connections := parseSSOutput(raw, nil)
	if len(connections) != 2 {
		t.Fatalf("connections = %+v, want 2", connections)
	}
	byAddress := map[string]string{}
	for _, connection := range connections {
		byAddress[connection.RemoteAddress] = connection.Direction
	}
	if byAddress["203.0.113.4"] != "inbound" {
		t.Fatalf("expected inbound direction for the listening port: %+v", connections)
	}
	if byAddress["1.1.1.1"] != "outbound" {
		t.Fatalf("expected outbound direction for the ephemeral port: %+v", connections)
	}
}

func TestParseSSListeningPortsFiltersHostPIDs(t *testing.T) {
	raw := `tcp LISTEN 0 4096 0.0.0.0:8080 0.0.0.0:* users:(("mine",pid=1,fd=2))
tcp LISTEN 0 4096 0.0.0.0:9090 0.0.0.0:* users:(("other",pid=2,fd=2))`
	ports := parseSSListeningPorts(raw, map[int]bool{1: true})
	if !ports[8080] || ports[9090] {
		t.Fatalf("unexpected listening ports: %+v", ports)
	}
}
