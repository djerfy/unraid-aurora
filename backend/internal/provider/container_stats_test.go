package provider

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const statsFixture = `{
  "cpu_stats": {
    "cpu_usage": {"total_usage": 300000000, "percpu_usage": [1,1,1,1]},
    "system_cpu_usage": 1000000000,
    "online_cpus": 4
  },
  "precpu_stats": {
    "cpu_usage": {"total_usage": 100000000},
    "system_cpu_usage": 600000000
  },
  "memory_stats": {
    "usage": 104857600,
    "limit": 1073741824,
    "stats": {"cache": 10485760}
  },
  "networks": {
    "eth0": {"rx_bytes": 1200, "tx_bytes": 340}
  },
  "blkio_stats": {
    "io_service_bytes_recursive": [
      {"op": "Read", "value": 4096},
      {"op": "Write", "value": 8192},
      {"op": "Total", "value": 12288}
    ]
  }
}`

type statsTransport struct{}

func (statsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(statsFixture)),
		Header:     make(http.Header),
	}, nil
}

func TestFetchContainerStats(t *testing.T) {
	live := &Live{client: &http.Client{Transport: statsTransport{}}}
	stats, err := live.fetchContainerStats(context.Background(), "abc")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	// cpu delta 200ms / system delta 400ms * 4 CPUs = 200%.
	if stats.cpu != 200 {
		t.Fatalf("cpu = %v, want 200", stats.cpu)
	}
	if stats.memory != 90*1024*1024 {
		t.Fatalf("memory = %d", stats.memory)
	}
	if stats.limit != 1024*1024*1024 {
		t.Fatalf("limit = %d", stats.limit)
	}
	if stats.rx != 1200 || stats.tx != 340 {
		t.Fatalf("network = %d/%d", stats.rx, stats.tx)
	}
	if stats.read != 4096 || stats.write != 8192 {
		t.Fatalf("block io = %d/%d", stats.read, stats.write)
	}
}

func TestWithContainerRates(t *testing.T) {
	start := time.Unix(100, 0)
	got := withContainerRates(
		cachedContainerStats{at: start, rx: 1000, tx: 2000, read: 3000, write: 4000},
		cachedContainerStats{at: start.Add(2 * time.Second), rx: 1400, tx: 2400, read: 3000, write: 5000},
	)
	if got.rxRate != 200 || got.txRate != 200 || got.readRate != 0 || got.writeRate != 500 {
		t.Fatalf("rates = %d/%d/%d/%d", got.rxRate, got.txRate, got.readRate, got.writeRate)
	}

	counter := withContainerRates(
		cachedContainerStats{at: start, rx: 5000},
		cachedContainerStats{at: start.Add(time.Second), rx: 10},
	)
	if counter.rxRate != 0 {
		t.Fatalf("counter reset rate = %d, want 0", counter.rxRate)
	}
}

func TestVirshParsers(t *testing.T) {
	colon := virshColonInfoFrom("CPU(s):    12\nMax memory:  25165824 KiB\nAutostart:      enable")
	if firstInt(colon["CPU(s)"]) != 12 || firstInt(colon["Max memory"]) != 25165824 {
		t.Fatalf("colon parser: %+v", colon)
	}
	values := map[string]string{}
	for _, line := range strings.Split("actual 4194304\nrss 2097152\nstate.state=1\ncpu.time=123456", "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			values[fields[0]] = fields[1]
		}
	}
	if firstInt(values["rss"]) != 2097152 || parseUint("123456") != 123456 {
		t.Fatalf("value parser: %+v", values)
	}
}

func TestDockerStatusUptime(t *testing.T) {
	cases := []struct {
		status string
		want   int64
	}{
		{"Up 12 seconds", 12},
		{"Up 5 minutes", 300},
		{"Up 2 hours", 7200},
		{"Up 3 days", 259200},
		{"Up 2 weeks", 1209600},
		{"Up 4 months", 10368000},
		{"Up 1 year", 31536000},
		{"Up About a minute", 60},
		{"Up About an hour", 3600},
		{"Up Less than a second", 1},
		{"Up 3 days (Paused)", 259200},
		{"Up 3 days (unhealthy)", 259200},
		{"Exited (0) 2 days ago", 0},
		{"Created", 0},
		{"", 0},
	}
	for _, tc := range cases {
		if got := dockerStatusUptime(tc.status); got != tc.want {
			t.Errorf("dockerStatusUptime(%q) = %d, want %d", tc.status, got, tc.want)
		}
	}
}
