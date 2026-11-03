// Package metrics samples the provider and fans the results out to SSE clients.
package metrics

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"aurora/internal/model"
	"aurora/internal/provider"
)

type Hub struct {
	provider provider.Provider

	mu          sync.RWMutex
	latest      model.Metrics
	history     []model.MetricPoint
	archive     []model.MetricPoint
	archivePath string
	lastArchive time.Time
	subs        map[chan model.Metrics]struct{}
}

func NewHub(p provider.Provider, archivePath string) *Hub {
	h := &Hub{
		provider: p, archivePath: archivePath,
		subs: make(map[chan model.Metrics]struct{}),
	}
	h.loadArchive()
	if history, err := p.History(context.Background()); err == nil {
		h.history = history
		if len(history) > 0 {
			last := history[len(history)-1]
			h.latest = model.Metrics{
				T: last.T, CPU: last.CPU, Memory: last.Memory, NetRx: last.NetRx,
				NetTx: last.NetTx, DiskRead: last.DiskRead, DiskWrite: last.DiskWrite,
				Temp: last.Temp,
			}
		}
	}
	return h
}

// Run samples every interval until the context is cancelled.
func (h *Hub) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	h.sample(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.sample(ctx)
		}
	}
}

func (h *Hub) sample(ctx context.Context) {
	m, err := h.provider.Metrics(ctx)
	if err != nil {
		return
	}
	h.mu.Lock()
	h.latest = m
	point := model.MetricPoint{
		T: m.T, CPU: m.CPU, Memory: m.Memory, NetRx: m.NetRx,
		NetTx: m.NetTx, DiskRead: m.DiskRead, DiskWrite: m.DiskWrite, Temp: m.Temp,
	}
	h.history = append(h.history, point)
	if len(h.history) > 1800 {
		h.history = h.history[len(h.history)-1800:]
	}
	if h.archivePath != "" && (h.lastArchive.IsZero() || time.Since(h.lastArchive) >= time.Minute) {
		h.archive = append(h.archive, point)
		if len(h.archive) > 44640 { // 31 days at one sample per minute.
			h.archive = h.archive[len(h.archive)-44640:]
		}
		h.lastArchive = time.Now()
		h.saveArchive()
	}
	subs := make([]chan model.Metrics, 0, len(h.subs))
	for ch := range h.subs {
		subs = append(subs, ch)
	}
	h.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- m:
		default:
			// Slow client: skip a frame instead of blocking the ticker.
		}
	}
}

func (h *Hub) Subscribe() (<-chan model.Metrics, func()) {
	ch := make(chan model.Metrics, 4)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	latest := h.latest
	h.mu.Unlock()
	if latest.T > 0 {
		ch <- latest
	}
	cancel := func() {
		h.mu.Lock()
		if _, ok := h.subs[ch]; ok {
			delete(h.subs, ch)
			close(ch)
		}
		h.mu.Unlock()
	}
	return ch, cancel
}

func (h *Hub) Latest() model.Metrics {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.latest
}

func (h *Hub) History() []model.MetricPoint {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]model.MetricPoint(nil), h.history...)
}

func (h *Hub) HistoryRange(span time.Duration) []model.MetricPoint {
	h.mu.RLock()
	defer h.mu.RUnlock()
	cutoff := time.Now().Add(-span).Unix()
	source := h.history
	if span > 2*time.Hour && len(h.archive) > 0 {
		source = h.archive
	}
	out := make([]model.MetricPoint, 0, len(source))
	for _, point := range source {
		if point.T >= cutoff {
			out = append(out, point)
		}
	}
	return out
}

func (h *Hub) loadArchive() {
	if h.archivePath == "" {
		return
	}
	raw, err := os.ReadFile(h.archivePath)
	if err != nil {
		return
	}
	_ = json.Unmarshal(raw, &h.archive)
}

func (h *Hub) saveArchive() {
	if h.archivePath == "" {
		return
	}
	raw, err := json.Marshal(h.archive)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(h.archivePath), 0o750); err != nil {
		return
	}
	tmp := h.archivePath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, h.archivePath)
}
