package metrics

import (
	"testing"
	"time"

	"aurora/internal/model"
)

func TestHistoryRangePrefersArchiveForLongSpans(t *testing.T) {
	now := time.Now().Unix()
	h := &Hub{
		history: []model.MetricPoint{{T: now - 60, CPU: 10}, {T: now - 7200, CPU: 20}},
		archive: []model.MetricPoint{{T: now - 3600, CPU: 30}, {T: now - 48*3600, CPU: 40}},
	}
	short := h.HistoryRange(time.Hour)
	if len(short) != 1 || short[0].CPU != 10 {
		t.Fatalf("short range = %+v", short)
	}
	long := h.HistoryRange(7 * 24 * time.Hour)
	if len(long) != 2 || long[0].CPU != 30 {
		t.Fatalf("long range = %+v", long)
	}
}
