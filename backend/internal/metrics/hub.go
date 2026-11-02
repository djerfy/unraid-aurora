// Package metrics samples the provider and fans the results out to SSE clients.
package metrics

import (
	"context"
	"sync"
	"time"

	"aurora/internal/model"
	"aurora/internal/provider"
)

type Hub struct {
	provider provider.Provider

	mu      sync.RWMutex
	latest  model.Metrics
	history []model.MetricPoint
	subs    map[chan model.Metrics]struct{}
}

func NewHub(p provider.Provider) *Hub {
	h := &Hub{
		provider: p,
		subs:     make(map[chan model.Metrics]struct{}),
	}
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
	h.history = append(h.history, model.MetricPoint{
		T: m.T, CPU: m.CPU, Memory: m.Memory, NetRx: m.NetRx,
		NetTx: m.NetTx, DiskRead: m.DiskRead, DiskWrite: m.DiskWrite, Temp: m.Temp,
	})
	if len(h.history) > 1800 {
		h.history = h.history[len(h.history)-1800:]
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
