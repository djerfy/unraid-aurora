package provider

import (
	"testing"

	"aurora/internal/model"
)

func TestBuildContainerNetworkMap(t *testing.T) {
	containers := []model.Container{
		{ID: "web", Name: "web", IP: "172.18.0.2", Network: "br0", State: "running"},
		{ID: "db", Name: "db", IP: "172.18.0.3", Network: "br0", State: "running"},
	}
	connections := map[string]model.ContainerConnections{
		"web": {Available: true, Connections: []model.ContainerConnection{
			{Protocol: "tcp", State: "ESTAB", RemoteAddress: "172.18.0.3", RemotePort: 5432, LocalPort: 40000, Process: "web"},
			{Protocol: "tcp", State: "ESTAB", RemoteAddress: "172.18.0.3", RemotePort: 5432, LocalPort: 40000, Process: "web"},
			{Protocol: "tcp", State: "ESTAB", RemoteAddress: "1.1.1.1", RemotePort: 443, LocalPort: 40001},
			{Protocol: "tcp", State: "ESTAB", RemoteAddress: "127.0.0.1", RemotePort: 8080, LocalPort: 40002},
		}},
		"db": {Available: true, Connections: []model.ContainerConnection{
			{Protocol: "udp", State: "ESTAB", RemoteAddress: "8.8.8.8", RemotePort: 53, LocalPort: 40003},
		}},
	}

	out := buildContainerNetworkMap(containers, connections)
	if !out.Available || len(out.Containers) != 2 {
		t.Fatalf("unexpected containers: %+v", out)
	}
	if len(out.Edges) != 3 {
		t.Fatalf("expected 3 deduplicated edges, got %d: %+v", len(out.Edges), out.Edges)
	}
	if len(out.Peers) != 3 {
		t.Fatalf("expected 3 peers, got %d: %+v", len(out.Peers), out.Peers)
	}
	if out.Peers[0].Scope != "container" || out.Peers[0].ContainerName != "db" {
		t.Fatalf("container peers must come first: %+v", out.Peers)
	}
	if out.Peers[0].Connections != 1 || len(out.Peers[0].Ports) != 1 || out.Peers[0].Ports[0] != 5432 {
		t.Fatalf("unexpected container peer: %+v", out.Peers[0])
	}
	for _, peer := range out.Peers {
		if peer.Address == "127.0.0.1" {
			t.Fatalf("loopback peers must be excluded: %+v", peer)
		}
	}
}

func TestClassifyNetworkMapPeer(t *testing.T) {
	byIP := map[string]model.Container{"172.18.0.3": {ID: "db", Name: "db"}}
	if scope, id, name := classifyNetworkMapPeer("172.18.0.3", byIP); scope != "container" || id != "db" || name != "db" {
		t.Fatalf("unexpected container classification: %s %s %s", scope, id, name)
	}
	if scope, _, _ := classifyNetworkMapPeer("192.168.1.20", byIP); scope != "lan" {
		t.Fatalf("expected lan, got %s", scope)
	}
	if scope, _, _ := classifyNetworkMapPeer("1.1.1.1", byIP); scope != "internet" {
		t.Fatalf("expected internet, got %s", scope)
	}
	if scope, _, _ := classifyNetworkMapPeer("not-an-ip", byIP); scope != "other" {
		t.Fatalf("expected other, got %s", scope)
	}
}
