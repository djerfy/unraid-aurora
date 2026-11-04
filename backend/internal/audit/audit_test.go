package audit

import (
	"testing"
	"time"
)

func TestRecordAndList(t *testing.T) {
	dir := t.TempDir()
	log := New(dir)
	log.Record(Entry{Time: time.Now(), User: "root", Method: "POST", Path: "/api/v1/actions", Status: 200})
	log.Record(Entry{Time: time.Now(), User: "root", Method: "PUT", Path: "/api/v1/settings", Status: 200})
	entries := log.List(10)
	if len(entries) != 2 || entries[0].Path != "/api/v1/settings" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	reloaded := New(dir)
	if got := reloaded.List(10); len(got) != 2 {
		t.Fatalf("audit log was not persisted: %+v", got)
	}
}

func TestFilteredList(t *testing.T) {
	log := New("")
	log.Record(Entry{Time: time.Now(), User: "root", Method: "GET", Path: "/a", Status: 200})
	log.Record(Entry{Time: time.Now(), User: "token:ops", Method: "POST", Path: "/b", Status: 403})
	log.Record(Entry{Time: time.Now(), User: "root", Method: "PUT", Path: "/c", Status: 500})
	if got := log.ListFiltered(10, Filter{StatusClass: 4}); len(got) != 1 || got[0].Path != "/b" {
		t.Fatalf("status class filter: %+v", got)
	}
	if got := log.ListFiltered(10, Filter{Method: "put"}); len(got) != 1 || got[0].Path != "/c" {
		t.Fatalf("method filter: %+v", got)
	}
	if got := log.ListFiltered(10, Filter{User: "root"}); len(got) != 2 {
		t.Fatalf("user filter: %+v", got)
	}
}

func TestAuditRetention(t *testing.T) {
	t.Setenv("AURORA_AUDIT_LIMIT", "100")
	log := New("")
	for i := 0; i < 150; i++ {
		log.Record(Entry{Time: time.Now(), Method: "POST", Path: "/x", Status: 200})
	}
	if got := len(log.List(0)); got != 100 {
		t.Fatalf("retention = %d, want 100", got)
	}
}
