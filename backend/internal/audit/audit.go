// Package audit keeps a durable record of mutating API calls.
package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultMaxEntries = 5000

// Filter narrows the audit list. Zero values mean "no filter".
type Filter struct {
	Method      string
	User        string
	Status      int
	StatusClass int
}

type Entry struct {
	Time   time.Time `json:"time"`
	User   string    `json:"user"`
	Method string    `json:"method"`
	Path   string    `json:"path"`
	Status int       `json:"status"`
	Remote string    `json:"remote"`
}

type Log struct {
	mu         sync.Mutex
	path       string
	entries    []Entry
	maxEntries int
}

func New(dir string) *Log {
	l := &Log{maxEntries: defaultMaxEntries}
	if value := os.Getenv("AURORA_AUDIT_LIMIT"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 100 && parsed <= 50000 {
			l.maxEntries = parsed
		}
	}
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
	if len(l.entries) > l.maxEntries {
		l.entries = l.entries[len(l.entries)-l.maxEntries:]
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
	return l.ListFiltered(limit, Filter{})
}

func (l *Log) ListFiltered(limit int, filter Filter) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	if limit <= 0 || limit > l.maxEntries {
		limit = l.maxEntries
	}
	out := make([]Entry, 0, min(limit, len(l.entries)))
	for index := len(l.entries) - 1; index >= 0 && len(out) < limit; index-- {
		entry := l.entries[index]
		if filter.Method != "" && !strings.EqualFold(filter.Method, entry.Method) {
			continue
		}
		if filter.User != "" && !strings.EqualFold(filter.User, entry.User) {
			continue
		}
		if filter.Status != 0 && filter.Status != entry.Status {
			continue
		}
		if filter.StatusClass != 0 && entry.Status/100 != filter.StatusClass {
			continue
		}
		out = append(out, entry)
	}
	return out
}
