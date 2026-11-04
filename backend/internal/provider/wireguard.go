package provider

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"aurora/internal/model"
)

const wireGuardConfigPath = "/boot/config/wireguard/wg0.cfg"

// wireGuard reads the Unraid WireGuard tunnel without mutating it. Peer names
// come from the comments in wg0.cfg; counters and handshakes come from `wg`.
func (l *Live) wireGuard(ctx context.Context) model.WireGuard {
	tunnel := model.WireGuard{Peers: []model.WireGuardPeer{}}
	file, err := os.Open(wireGuardConfigPath)
	if err != nil {
		return tunnel
	}
	defer file.Close()
	names := map[string]string{}
	publicKey := ""
	peerName := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(line, "#"):
			peerName = strings.TrimSpace(strings.TrimPrefix(line, "#"))
		case strings.HasPrefix(strings.ToLower(line), "listenport"):
			value := strings.TrimSpace(strings.SplitN(line, "=", 2)[1])
			tunnel.Port = atoi(value)
		case strings.EqualFold(line, "[peer]"):
			publicKey = ""
		case strings.HasPrefix(strings.ToLower(line), "publickey"):
			publicKey = strings.TrimSpace(strings.SplitN(line, "=", 2)[1])
			if peerName != "" && publicKey != "" {
				names[publicKey] = peerName
			}
		}
	}
	if tunnel.Port == 0 {
		if out, err := exec.CommandContext(ctx, "wg", "show", "wg0", "listen-port").Output(); err == nil {
			tunnel.Port = atoi(strings.TrimSpace(string(out)))
		}
	}
	out, err := exec.CommandContext(ctx, "wg", "show", "wg0", "dump").Output()
	if err != nil {
		return tunnel
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 8 {
			continue
		}
		handshake := int64(atoi(fields[4]))
		peer := model.WireGuardPeer{
			Name:       names[fields[0]],
			PublicKey:  fields[0],
			Endpoint:   strings.TrimSpace(fields[2]),
			AllowedIPs: splitList(fields[3]),
			Rx:         uint64(parseInt(fields[5])),
			Tx:         uint64(parseInt(fields[6])),
		}
		if peer.Name == "" {
			peer.Name = shortKey(peer.PublicKey)
		}
		if handshake > 0 {
			peer.Latest = time.Unix(handshake, 0)
			peer.Online = time.Since(peer.Latest) < 3*time.Minute
		}
		tunnel.Peers = append(tunnel.Peers, peer)
	}
	tunnel.Enabled = tunnel.Port > 0 || len(tunnel.Peers) > 0
	return tunnel
}

func splitList(value string) []string {
	out := []string{}
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func shortKey(key string) string {
	if len(key) <= 12 {
		return key
	}
	return key[:6] + "…" + key[len(key)-4:]
}
