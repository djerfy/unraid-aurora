package provider

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurora/internal/model"
)

const (
	networkMapScopeContainer = "container"
	networkMapScopeLAN       = "lan"
	networkMapScopeInternet  = "internet"
	networkMapScopeOther     = "other"

	networkMapConcurrency = 6
	networkMapTimeout     = 10 * time.Second
	networkMapMaxEdges    = 1000
)

// ContainerNetworkMap aggregates live sockets across running containers into a
// node/edge graph. Loopback peers are dropped because they describe in-container
// wiring rather than a conversation with the outside.
func (l *Live) ContainerNetworkMap(ctx context.Context, containerID string) (model.ContainerNetworkMap, error) {
	containers, err := l.listContainers(ctx)
	if err != nil {
		return model.ContainerNetworkMap{}, err
	}
	running := make([]model.Container, 0, len(containers))
	for _, container := range containers {
		if container.State == "running" {
			running = append(running, container)
		}
	}
	inspected := running
	if containerID = strings.TrimSpace(containerID); containerID != "" {
		selected := -1
		for index := range running {
			if running[index].ID == containerID {
				selected = index
				break
			}
		}
		if selected < 0 {
			return model.ContainerNetworkMap{}, fmt.Errorf("container %q is not running", containerID)
		}
		inspected = []model.Container{running[selected]}
	}
	connections := l.gatherContainerConnections(ctx, inspected)
	result := buildContainerNetworkMap(running, inspected, connections)

	available := 0
	reason := ""
	for _, container := range running {
		connectionResult, ok := connections[container.ID]
		if !ok {
			continue
		}
		if connectionResult.Available {
			available++
		} else if reason == "" {
			reason = connectionResult.Reason
		}
	}
	if len(running) > 0 && available == 0 && reason != "" {
		result.Available = false
		result.Reason = reason
	}

	l.enrichNetworkMapPeers(ctx, result.Peers)
	return result, nil
}

func (l *Live) gatherContainerConnections(ctx context.Context, containers []model.Container) map[string]model.ContainerConnections {
	ctx, cancel := context.WithTimeout(ctx, networkMapTimeout)
	defer cancel()

	results := make(map[string]model.ContainerConnections, len(containers))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, networkMapConcurrency)

	for _, container := range containers {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			result, err := l.containerConnections(ctx, id, false)
			if err != nil {
				return
			}
			mu.Lock()
			results[id] = result
			mu.Unlock()
		}(container.ID)
	}
	wg.Wait()
	return results
}

func buildContainerNetworkMap(allContainers, inspected []model.Container, connections map[string]model.ContainerConnections) model.ContainerNetworkMap {
	out := model.ContainerNetworkMap{
		Available:  true,
		CapturedAt: time.Now().UTC(),
		Containers: make([]model.NetworkMapContainer, 0, len(inspected)),
		Peers:      []model.NetworkMapPeer{},
		Edges:      []model.NetworkMapEdge{},
	}
	byIP := make(map[string]model.Container, len(allContainers))
	for _, container := range allContainers {
		if container.IP != "" {
			byIP[container.IP] = container
		}
	}
	for _, container := range inspected {
		out.Containers = append(out.Containers, model.NetworkMapContainer{
			ID: container.ID, Name: container.Name, Image: container.Image,
			State: container.State, Network: container.Network, IP: container.IP,
		})
	}

	peerIndex := map[string]int{}
	edgeSeen := map[string]bool{}
	for _, container := range inspected {
		for _, connection := range connections[container.ID].Connections {
			remote := strings.TrimSpace(connection.RemoteAddress)
			if remote == "" {
				continue
			}
			if ip := net.ParseIP(remote); ip != nil && ip.IsLoopback() {
				continue
			}
			edgeKey := strings.Join([]string{
				container.ID, remote, connection.Protocol, connection.Direction,
				strconv.Itoa(connection.LocalPort), strconv.Itoa(connection.RemotePort), connection.State,
			}, "|")
			if edgeSeen[edgeKey] {
				continue
			}
			edgeSeen[edgeKey] = true
			if len(out.Edges) >= networkMapMaxEdges {
				continue
			}

			scope, peerID, peerName := classifyNetworkMapPeer(remote, byIP)
			index, ok := peerIndex[remote]
			if !ok {
				index = len(out.Peers)
				peerIndex[remote] = index
				out.Peers = append(out.Peers, model.NetworkMapPeer{
					Address: remote, Scope: scope,
					ContainerID: peerID, ContainerName: peerName,
				})
			}
			peer := &out.Peers[index]
			peer.Connections++
			port := connection.RemotePort
			if connection.Direction == "inbound" {
				port = connection.LocalPort
			}
			peer.Ports = appendUniqueInt(peer.Ports, port)
			peer.Protocols = appendUniqueString(peer.Protocols, connection.Protocol)

			out.Edges = append(out.Edges, model.NetworkMapEdge{
				ContainerID: container.ID, ContainerName: container.Name,
				PeerAddress: remote, PeerContainerID: peerID,
				Protocol: connection.Protocol, Direction: connection.Direction,
				LocalPort: connection.LocalPort, RemotePort: connection.RemotePort,
				State: connection.State, Process: connection.Process,
			})
		}
	}

	sort.Slice(out.Peers, func(i, j int) bool {
		if scopeRank(out.Peers[i].Scope) != scopeRank(out.Peers[j].Scope) {
			return scopeRank(out.Peers[i].Scope) < scopeRank(out.Peers[j].Scope)
		}
		if out.Peers[i].Connections != out.Peers[j].Connections {
			return out.Peers[i].Connections > out.Peers[j].Connections
		}
		return out.Peers[i].Address < out.Peers[j].Address
	})
	sort.Slice(out.Edges, func(i, j int) bool {
		if out.Edges[i].ContainerName != out.Edges[j].ContainerName {
			return out.Edges[i].ContainerName < out.Edges[j].ContainerName
		}
		return out.Edges[i].PeerAddress < out.Edges[j].PeerAddress
	})
	return out
}

func classifyNetworkMapPeer(address string, byIP map[string]model.Container) (string, string, string) {
	if container, ok := byIP[address]; ok {
		return networkMapScopeContainer, container.ID, container.Name
	}
	ip := net.ParseIP(address)
	switch {
	case ip == nil:
		return networkMapScopeOther, "", ""
	case ip.IsPrivate():
		return networkMapScopeLAN, "", ""
	default:
		return networkMapScopeInternet, "", ""
	}
}

func scopeRank(scope string) int {
	switch scope {
	case networkMapScopeContainer:
		return 0
	case networkMapScopeLAN:
		return 1
	case networkMapScopeInternet:
		return 2
	default:
		return 3
	}
}

func appendUniqueInt(values []int, value int) []int {
	if value == 0 {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func appendUniqueString(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func (l *Live) enrichNetworkMapPeers(ctx context.Context, peers []model.NetworkMapPeer) {
	path := findCommand("geoiplookup", "/usr/bin/geoiplookup", "/usr/local/bin/geoiplookup")
	if path == "" || len(peers) == 0 {
		return
	}
	geoCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	seen := 0
	for index := range peers {
		if peers[index].Scope != networkMapScopeInternet {
			continue
		}
		if seen >= 32 || geoCtx.Err() != nil {
			break
		}
		seen++
		country, city := l.geoFor(geoCtx, path, peers[index].Address)
		peers[index].Country = country
		peers[index].City = city
	}
}
