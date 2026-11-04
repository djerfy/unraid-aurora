package provider

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// streamServer serves the given lines with delay between them. When the
// request context is cancelled the handler returns immediately.
func streamServer(t *testing.T, lines []string, delay time.Duration) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		for index, line := range lines {
			if index > 0 {
				select {
				case <-r.Context().Done():
					return
				case <-time.After(delay):
				}
			}
			_, _ = io.WriteString(w, line+"\n")
			flusher.Flush()
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func dockerTestClient(server *httptest.Server, timeout time.Duration) *http.Client {
	transport := &http.Transport{DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		var dialer net.Dialer
		return dialer.DialContext(ctx, network, server.Listener.Addr().String())
	}}
	return &http.Client{Timeout: timeout, Transport: transport}
}

func TestPullImageOutlivesShortClientTimeout(t *testing.T) {
	server := streamServer(t, []string{
		`{"status":"Pulling from linuxserver/jackett","id":"latest"}`,
		`{"status":"Download complete","id":"abc","progressDetail":{"current":10,"total":10}}`,
	}, 150*time.Millisecond)
	live := &Live{client: dockerTestClient(server, 50*time.Millisecond)}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var messages []string
	err := live.pullImageWithProgress(ctx, "linuxserver/jackett:latest", func(message string, _ float64) {
		messages = append(messages, message)
	})
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if len(messages) == 0 || messages[len(messages)-1] != "Image pulled" {
		t.Fatalf("unexpected progress messages: %v", messages)
	}
}

func TestPullImageAbortsWhenDaemonGoesSilent(t *testing.T) {
	server := streamServer(t, []string{
		`{"status":"Pulling from linuxserver/jackett","id":"latest"}`,
		`{"status":"Download complete","id":"abc"}`,
	}, 5*time.Second)
	live := &Live{
		client:    dockerTestClient(server, 0),
		pullStall: 100 * time.Millisecond,
	}
	start := time.Now()
	err := live.pullImage(context.Background(), "linuxserver/jackett:latest")
	if err == nil || !strings.Contains(err.Error(), "no progress from the Docker daemon") {
		t.Fatalf("expected a stall error, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("stall watchdog was too slow: %s", elapsed)
	}
}

func TestDockerPullStallValidation(t *testing.T) {
	cases := map[string]time.Duration{
		"":      10 * time.Minute,
		"5m":    5 * time.Minute,
		"0":     0,
		"90s":   90 * time.Second,
		"bogus": 10 * time.Minute,
		"-1m":   10 * time.Minute,
	}
	for value, want := range cases {
		t.Setenv("AURORA_DOCKER_PULL_STALL", value)
		if got := dockerPullStall(); got != want {
			t.Fatalf("%q: got %s, want %s", value, got, want)
		}
	}
}

func TestDockerLongClientKeepsTransport(t *testing.T) {
	transport := http.DefaultTransport
	live := &Live{client: &http.Client{Timeout: time.Second, Transport: transport}}
	long := live.dockerLongClient()
	if long.Timeout != 0 {
		t.Fatalf("timeout = %v, want 0", long.Timeout)
	}
	if long.Transport != transport {
		t.Fatal("long client must reuse the Docker transport")
	}
	if live.client.Timeout != time.Second {
		t.Fatal("the original client must not be mutated")
	}
}

func TestPullImageReportsDockerError(t *testing.T) {
	live := &Live{client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{"error":"manifest unknown"}` + "\n")),
			Header:     make(http.Header),
		}, nil
	})}}
	err := live.pullImage(context.Background(), "linuxserver/jackett:latest")
	if err == nil || !strings.Contains(err.Error(), "manifest unknown") {
		t.Fatalf("expected the daemon error, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
