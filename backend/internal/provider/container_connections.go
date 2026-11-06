package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"aurora/internal/model"
)

var ssProcessPattern = regexp.MustCompile(`\("([^"]+)",pid=(\d+)`)

type geoEntry struct {
	country string
	city    string
	at      time.Time
}

// ContainerConnections inspects the container network namespace with ss.
// Sockets in a private network namespace already belong to the container;
// host-network containers are filtered by their Docker process IDs.
func (l *Live) ContainerConnections(ctx context.Context, id string) (model.ContainerConnections, error) {
	return l.containerConnections(ctx, id, true)
}

func (l *Live) containerConnections(ctx context.Context, id string, enrichGeo bool) (model.ContainerConnections, error) {
	id = strings.TrimSpace(id)
	if id == "" || strings.ContainsAny(id, "/ \t\n\r\x00") {
		return model.ContainerConnections{}, errors.New("invalid container id")
	}
	ssPath := findCommand("ss", "/usr/sbin/ss", "/sbin/ss", "/usr/bin/ss", "/bin/ss")
	nsenterPath := findCommand("nsenter", "/usr/bin/nsenter", "/bin/nsenter", "/usr/sbin/nsenter")
	if ssPath == "" || nsenterPath == "" {
		return model.ContainerConnections{Available: false, Reason: "ss or nsenter is not available on this host"}, nil
	}

	raw, err := l.dockerDo(ctx, http.MethodGet, "/containers/"+url.PathEscape(id)+"/json", nil)
	if err != nil {
		return model.ContainerConnections{Available: false, Reason: "the container could not be inspected"}, nil
	}
	var info struct {
		State struct {
			Running bool `json:"Running"`
			Pid     int  `json:"Pid"`
		} `json:"State"`
		HostConfig struct {
			NetworkMode string `json:"NetworkMode"`
		} `json:"HostConfig"`
	}
	if err := json.Unmarshal(raw, &info); err != nil {
		return model.ContainerConnections{Available: false, Reason: "the container could not be inspected"}, nil
	}
	if !info.State.Running || info.State.Pid <= 0 {
		return model.ContainerConnections{Available: true, Reason: "the container is not running", Connections: []model.ContainerConnection{}}, nil
	}

	allowedPIDs := map[int]bool{}
	if info.HostConfig.NetworkMode == "host" {
		allowedPIDs = l.containerPIDs(ctx, id)
		if len(allowedPIDs) == 0 {
			return model.ContainerConnections{Available: true, Reason: "the container processes could not be mapped", Connections: []model.ContainerConnection{}}, nil
		}
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, nsenterPath, "-t", strconv.Itoa(info.State.Pid), "-n", "--", ssPath, "-tunap")
	out, err := cmd.Output()
	if err != nil {
		return model.ContainerConnections{Available: true, Reason: "ss could not inspect the container network namespace", Connections: []model.ContainerConnection{}}, nil
	}
	connections := parseSSOutput(string(out), allowedPIDs)
	if enrichGeo {
		l.enrichConnectionsGeo(ctx, connections)
	}
	return model.ContainerConnections{Available: true, Connections: connections}, nil
}

func (l *Live) containerPIDs(ctx context.Context, id string) map[int]bool {
	raw, err := l.dockerDo(ctx, http.MethodGet, "/containers/"+url.PathEscape(id)+"/top?ps_args="+url.QueryEscape("-eo pid"), nil)
	if err != nil {
		return nil
	}
	var top struct {
		Processes [][]string `json:"Processes"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil
	}
	pids := map[int]bool{}
	for _, row := range top.Processes {
		if len(row) == 0 {
			continue
		}
		if pid, err := strconv.Atoi(strings.TrimSpace(row[0])); err == nil && pid > 0 {
			pids[pid] = true
		}
	}
	return pids
}

func parseSSOutput(output string, allowedPIDs map[int]bool) []model.ContainerConnection {
	listening := parseSSListeningPorts(output, allowedPIDs)
	seen := map[string]bool{}
	out := []model.ContainerConnection{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		netid := fields[0]
		if netid != "tcp" && netid != "udp" && netid != "tcp6" && netid != "udp6" {
			continue
		}
		state := fields[1]
		if state == "LISTEN" || state == "UNCONN" {
			continue
		}
		localAddress, localPort := parseSSAddress(fields[4])
		remoteAddress, remotePort := parseSSAddress(fields[5])
		if remoteAddress == "" || remotePort == 0 {
			continue
		}
		ip := net.ParseIP(remoteAddress)
		if !isRemoteAddress(ip) {
			continue
		}
		process, pid := parseSSProcess(strings.Join(fields[6:], " "))
		if len(allowedPIDs) > 0 && (pid == 0 || !allowedPIDs[pid]) {
			continue
		}
		key := fmt.Sprintf("%s|%s|%d|%s|%d|%d", netid, localAddress, localPort, remoteAddress, remotePort, pid)
		if seen[key] {
			continue
		}
		seen[key] = true
		direction := "outbound"
		if listening[localPort] {
			direction = "inbound"
		}
		out = append(out, model.ContainerConnection{
			Protocol: netid, State: state, Direction: direction,
			LocalAddress: localAddress, LocalPort: localPort,
			RemoteAddress: remoteAddress, RemotePort: remotePort,
			Process: process, PID: pid,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RemoteAddress != out[j].RemoteAddress {
			return out[i].RemoteAddress < out[j].RemoteAddress
		}
		if out[i].RemotePort != out[j].RemotePort {
			return out[i].RemotePort < out[j].RemotePort
		}
		return out[i].Process < out[j].Process
	})
	return out
}

// parseSSListeningPorts records the local ports the container accepts traffic
// on, so established sockets can be split between inbound and outbound.
func parseSSListeningPorts(output string, allowedPIDs map[int]bool) map[int]bool {
	ports := map[int]bool{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		netid := fields[0]
		if netid != "tcp" && netid != "udp" && netid != "tcp6" && netid != "udp6" {
			continue
		}
		state := fields[1]
		if state != "LISTEN" && state != "UNCONN" {
			continue
		}
		_, port := parseSSAddress(fields[4])
		if port == 0 {
			continue
		}
		if len(allowedPIDs) > 0 {
			_, pid := parseSSProcess(strings.Join(fields[6:], " "))
			if pid == 0 || !allowedPIDs[pid] {
				continue
			}
		}
		ports[port] = true
	}
	return ports
}

func parseSSAddress(value string) (string, int) {
	value = strings.TrimSpace(value)
	if host, port, err := net.SplitHostPort(value); err == nil {
		number, _ := strconv.Atoi(port)
		return stripZone(strings.Trim(host, "[]")), number
	}
	i := strings.LastIndexByte(value, ':')
	if i < 0 {
		return "", 0
	}
	host := stripZone(strings.Trim(value[:i], "[]"))
	port, _ := strconv.Atoi(value[i+1:])
	return host, port
}

func stripZone(host string) string {
	if i := strings.IndexByte(host, '%'); i >= 0 {
		return host[:i]
	}
	return host
}

func parseSSProcess(line string) (string, int) {
	match := ssProcessPattern.FindStringSubmatch(line)
	if len(match) < 3 {
		return "", 0
	}
	pid, _ := strconv.Atoi(match[2])
	return match[1], pid
}

// isRemoteAddress keeps every real peer, including private ranges and
// loopback. Only listening, unspecified, link-local, multicast and broadcast
// addresses are not meaningful connections.
func isRemoteAddress(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return false
	}
	return !ip.Equal(net.IPv4bcast)
}

func isPublicAddress(ip net.IP) bool {
	return isRemoteAddress(ip) && !ip.IsLoopback() && !ip.IsPrivate()
}

func (l *Live) enrichConnectionsGeo(ctx context.Context, connections []model.ContainerConnection) {
	path := findCommand("geoiplookup", "/usr/bin/geoiplookup", "/usr/local/bin/geoiplookup")
	if path == "" || len(connections) == 0 {
		return
	}
	geoCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	seen := map[string]bool{}
	for i := range connections {
		ip := connections[i].RemoteAddress
		if seen[ip] || !isPublicAddress(net.ParseIP(ip)) {
			continue
		}
		if len(seen) >= 32 || geoCtx.Err() != nil {
			break
		}
		seen[ip] = true
		country, city := l.geoFor(geoCtx, path, ip)
		connections[i].Country = country
		connections[i].City = city
	}
}

func (l *Live) geoFor(ctx context.Context, path, ip string) (string, string) {
	l.geoMu.Lock()
	entry, ok := l.geoCache[ip]
	l.geoMu.Unlock()
	if ok && time.Since(entry.at) < 24*time.Hour {
		return entry.country, entry.city
	}
	if ctx.Err() != nil {
		return "", ""
	}
	args := []string{ip}
	if database := geoCityDatabase(ip); database != "" {
		args = []string{"-f", database, ip}
	}
	out, err := exec.CommandContext(ctx, path, args...).Output()
	if err != nil {
		return "", ""
	}
	country, city := parseGeoLookup(string(out))
	l.geoMu.Lock()
	if l.geoCache == nil {
		l.geoCache = map[string]geoEntry{}
	}
	l.geoCache[ip] = geoEntry{country: country, city: city, at: time.Now()}
	l.geoMu.Unlock()
	return country, city
}

func geoCityDatabase(ip string) string {
	paths := []string{
		"/usr/share/GeoIP/GeoLiteCity.dat",
		"/usr/share/GeoIP/GeoIPCity.dat",
	}
	if strings.Contains(ip, ":") {
		paths = []string{
			"/usr/share/GeoIP/GeoLiteCityv6.dat",
			"/usr/share/GeoIP/GeoIPCityv6.dat",
		}
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func parseGeoLookup(output string) (string, string) {
	line := strings.TrimSpace(strings.SplitN(output, "\n", 2)[0])
	if line == "" || strings.Contains(strings.ToLower(line), "not found") {
		return "", ""
	}
	_, rest, ok := strings.Cut(line, ": ")
	if !ok {
		return "", ""
	}
	parts := strings.Split(rest, ",")
	if len(parts) < 2 {
		return "", ""
	}
	code := strings.TrimSpace(parts[0])
	if len(code) != 2 {
		code = ""
	}
	city := ""
	if len(parts) >= 3 {
		city = strings.TrimSpace(parts[2])
	}
	return code, city
}
