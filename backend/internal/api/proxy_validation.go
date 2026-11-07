package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"aurora/internal/model"
)

const (
	maxProxyConfigs = 4
	maxProxyName    = 64
	maxProxyURL     = 2048
	maxProxyUser    = 256
	maxProxyPass    = 1024
)

func newProxyID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return ""
	}
	return "proxy-" + hex.EncodeToString(buf[:])
}

// normalizeProxySettings trims, validates and assigns stable IDs so the UI can
// save partial drafts without breaking the active selection.
func normalizeProxySettings(settings *model.Settings) error {
	settings.ProxyActiveID = strings.TrimSpace(settings.ProxyActiveID)
	if len(settings.Proxies) > maxProxyConfigs {
		return fmt.Errorf("at most %d proxy configurations are allowed", maxProxyConfigs)
	}
	proxies := make([]model.ProxyConfig, 0, len(settings.Proxies))
	seen := map[string]bool{}
	for _, proxy := range settings.Proxies {
		proxy.ID = strings.TrimSpace(proxy.ID)
		proxy.Name = strings.TrimSpace(proxy.Name)
		proxy.URL = strings.TrimSpace(proxy.URL)
		proxy.Username = strings.TrimSpace(proxy.Username)
		if proxy.Name == "" && proxy.URL == "" {
			continue
		}
		if proxy.Name == "" {
			return fmt.Errorf("proxy name is required")
		}
		if len(proxy.Name) > maxProxyName {
			return fmt.Errorf("proxy name %q is too long", proxy.Name)
		}
		if proxy.URL == "" {
			return fmt.Errorf("proxy URL is required")
		}
		if len(proxy.URL) > maxProxyURL {
			return fmt.Errorf("proxy URL is too long")
		}
		parsed, err := url.Parse(proxy.URL)
		if err != nil || parsed.Host == "" {
			return fmt.Errorf("proxy %q has an invalid URL", proxy.Name)
		}
		switch strings.ToLower(parsed.Scheme) {
		case "http", "https", "socks5", "socks5h":
		default:
			return fmt.Errorf("proxy %q must use http, https, socks5 or socks5h", proxy.Name)
		}
		if len(proxy.Username) > maxProxyUser {
			return fmt.Errorf("proxy %q username is too long", proxy.Name)
		}
		if len(proxy.Password) > maxProxyPass {
			return fmt.Errorf("proxy %q password is too long", proxy.Name)
		}
		if proxy.Password != "" && proxy.Username == "" {
			return fmt.Errorf("proxy %q password requires a username", proxy.Name)
		}
		if proxy.ID == "" || seen[proxy.ID] {
			proxy.ID = newProxyID()
			if proxy.ID == "" {
				return fmt.Errorf("cannot generate a proxy identifier")
			}
		}
		seen[proxy.ID] = true
		proxies = append(proxies, proxy)
	}
	if settings.ProxyEnabled && len(proxies) == 0 {
		return fmt.Errorf("add a proxy configuration before enabling the outbound proxy")
	}
	if len(proxies) == 0 {
		settings.ProxyActiveID = ""
	} else if !seen[settings.ProxyActiveID] {
		settings.ProxyActiveID = proxies[0].ID
	}
	settings.Proxies = proxies
	return nil
}
