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
