package provider

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
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
