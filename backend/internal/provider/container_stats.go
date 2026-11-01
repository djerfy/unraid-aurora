package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sync"
	"time"

	"aurora/internal/model"
)

const containerStatsTTL = 10 * time.Second

type cachedContainerStats struct {
	at     time.Time
	cpu    float64
	memory uint64
	limit  uint64
	rx     uint64
	tx     uint64
}

type dockerContainerStats struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage  uint64   `json:"total_usage"`
			PercpuUsage []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs     uint32 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64            `json:"usage"`
		Limit uint64            `json:"limit"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
}

func (l *Live) enrichContainerStats(ctx context.Context, containers []model.Container) {
	cache := l.containerStatsSnapshot()
	now := time.Now()
	need := make([]string, 0, len(containers))
	for _, container := range containers {
		if container.State != "running" {
			continue
		}
		if cached, ok := cache[container.ID]; ok && now.Sub(cached.at) < containerStatsTTL {
			continue
		}
		need = append(need, container.ID)
	}

	fetched := map[string]cachedContainerStats{}
	if len(need) > 0 {
		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, 6)
		for _, id := range need {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				stats, err := l.fetchContainerStats(ctx, id)
				if err != nil {
					return
				}
				mu.Lock()
				fetched[id] = stats
				mu.Unlock()
			}(id)
		}
		wg.Wait()
	}

	l.statsMu.Lock()
	if l.containerStats == nil {
		l.containerStats = map[string]cachedContainerStats{}
	}
	for id, stats := range fetched {
		l.containerStats[id] = stats
	}
	merged := make(map[string]cachedContainerStats, len(cache)+len(fetched))
	for id, stats := range cache {
		merged[id] = stats
	}
	for id, stats := range fetched {
		merged[id] = stats
	}
	l.statsMu.Unlock()

	for i := range containers {
		if stats, ok := merged[containers[i].ID]; ok {
			containers[i].CPU = stats.cpu
			containers[i].Memory = stats.memory
			containers[i].MemoryLimit = stats.limit
			containers[i].NetworkRx = stats.rx
			containers[i].NetworkTx = stats.tx
		}
	}
}

func (l *Live) containerStatsSnapshot() map[string]cachedContainerStats {
	l.statsMu.Lock()
	defer l.statsMu.Unlock()
	out := make(map[string]cachedContainerStats, len(l.containerStats))
	for id, stats := range l.containerStats {
		out[id] = stats
	}
	return out
}

func (l *Live) fetchContainerStats(ctx context.Context, id string) (cachedContainerStats, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	path := "/containers/" + url.PathEscape(id) + "/stats?stream=false&one-shot=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker"+path, nil)
	if err != nil {
		return cachedContainerStats{}, err
	}
	client := l.client
	if client.Timeout > 0 {
		clone := *client
		clone.Timeout = 0
		client = &clone
	}
	resp, err := client.Do(req)
	if err != nil {
		return cachedContainerStats{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return cachedContainerStats{}, fmt.Errorf("docker stats returned %s", resp.Status)
	}
	var raw dockerContainerStats
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&raw); err != nil {
		return cachedContainerStats{}, fmt.Errorf("decode docker stats: %w", err)
	}

	cpu := 0.0
	if raw.CPUStats.SystemCPUUsage > raw.PreCPUStats.SystemCPUUsage &&
		raw.CPUStats.CPUUsage.TotalUsage > raw.PreCPUStats.CPUUsage.TotalUsage {
		cpuDelta := float64(raw.CPUStats.CPUUsage.TotalUsage - raw.PreCPUStats.CPUUsage.TotalUsage)
		systemDelta := float64(raw.CPUStats.SystemCPUUsage - raw.PreCPUStats.SystemCPUUsage)
		online := raw.CPUStats.OnlineCPUs
		if online == 0 {
			online = uint32(len(raw.CPUStats.CPUUsage.PercpuUsage))
		}
		if online == 0 {
			online = 1
		}
		cpu = cpuDelta / systemDelta * float64(online) * 100
	}
	memory := raw.MemoryStats.Usage
	cache := raw.MemoryStats.Stats["cache"]
	if inactive := raw.MemoryStats.Stats["inactive_file"]; inactive > cache {
		cache = inactive
	}
	if cache > 0 && memory > cache {
		memory -= cache
	}
	var rx, tx uint64
	for _, network := range raw.Networks {
		rx += network.RxBytes
		tx += network.TxBytes
	}
	return cachedContainerStats{
		at:     time.Now(),
		cpu:    math.Round(cpu*10) / 10,
		memory: memory,
		limit:  raw.MemoryStats.Limit,
		rx:     rx,
		tx:     tx,
	}, nil
}
