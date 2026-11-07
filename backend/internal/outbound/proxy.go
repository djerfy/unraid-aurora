// Package outbound centralizes the HTTP transport used for Aurora's internet
// traffic so a user-configured proxy applies to every outbound request.
package outbound

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var (
	mu      sync.RWMutex
	current *url.URL

	transportOnce sync.Once
	transport     *http.Transport
)

// Configure sets the proxy used by outbound clients. Passing enabled=false
// clears it. The URL accepts http, https, socks5 and socks5h schemes.
func Configure(enabled bool, rawURL, username, password string) error {
	var parsed *url.URL
	if enabled {
		candidate, err := url.Parse(strings.TrimSpace(rawURL))
		if err != nil {
			return fmt.Errorf("parse proxy URL: %w", err)
		}
		switch strings.ToLower(candidate.Scheme) {
		case "http", "https", "socks5", "socks5h":
		default:
			return errors.New("proxy URL must use http, https, socks5 or socks5h")
		}
		if candidate.Host == "" {
			return errors.New("proxy URL must include a host")
		}
		if username = strings.TrimSpace(username); username != "" {
			candidate.User = url.UserPassword(username, password)
		}
		parsed = candidate
	}
	mu.Lock()
	current = parsed
	mu.Unlock()
	return nil
}

// Current returns a copy of the active proxy URL, or nil when disabled.
func Current() *url.URL {
	mu.RLock()
	defer mu.RUnlock()
	if current == nil {
		return nil
	}
	clone := *current
	return &clone
}

// Env returns proxy variables for child processes that download data (Trivy).
func Env() []string {
	proxy := Current()
	if proxy == nil {
		return nil
	}
	value := proxy.String()
	return []string{
		"HTTP_PROXY=" + value, "HTTPS_PROXY=" + value,
		"http_proxy=" + value, "https_proxy=" + value,
	}
}

func proxyForRequest(*http.Request) (*url.URL, error) {
	return Current(), nil
}

// Transport returns a shared transport that resolves the proxy at request
// time, so enabling or changing a proxy takes effect without a restart.
func Transport() *http.Transport {
	transportOnce.Do(func() {
		if base, ok := http.DefaultTransport.(*http.Transport); ok {
			transport = base.Clone()
		} else {
			transport = &http.Transport{}
		}
		transport.Proxy = proxyForRequest
	})
	return transport
}

// Client builds an HTTP client on the shared outbound transport.
func Client(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: Transport()}
}
