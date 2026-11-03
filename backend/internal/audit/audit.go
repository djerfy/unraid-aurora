// Package audit keeps a durable record of mutating API calls.
package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Entry struct {
	Time   time.Time `json:"time"`
	User   string    `json:"user"`
	Method string    `json:"method"`
	Path   string    `json:"path"`
	Status int       `json:"status"`
	Remote string    `json:"remote"`
}

type Log struct {
	mu      sync.Mutex
	path    string
	entries []Entry
}

func New(dir string) *Log {
	l := &Log{}
	if dir == "" {
		return l
	}
	l.path = filepath.Join(dir, "audit-log.json")
	l.load()
	return l
}

func (l *Log) load() {
	raw, err := os.ReadFile(l.path)
	if err != nil {
		return
	}
	_ = json.Unmarshal(raw, &l.entries)
}

func (l *Log) Record(entry Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
	if len(l.entries) > 1000 {
		l.entries = l.entries[len(l.entries)-1000:]
	}
	if l.path == "" {
		return
	}
	raw, err := json.MarshalIndent(l.entries, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o750); err != nil {
		return
	}
	tmp := l.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, l.path)
}

func (l *Log) List(limit int) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	if limit <= 0 || limit > len(l.entries) {
		limit = len(l.entries)
	}
	out := make([]Entry, limit)
	copy(out, l.entries[len(l.entries)-limit:])
	for left, right := 0, len(out)-1; left < right; left, right = left+1, right-1 {
		out[left], out[right] = out[right], out[left]
	}
	return out
}
