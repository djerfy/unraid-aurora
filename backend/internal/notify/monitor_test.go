package notify

import (
	"testing"

	"aurora/internal/model"
)

func TestMonitorIgnoresBaselineAndReportsTransitions(t *testing.T) {
	monitor := &Monitor{}
	base := model.Snapshot{
		Containers: []model.Container{{Name: "plex", State: "running", Health: "healthy"}},
		Disks:      []model.Disk{{Name: "disk1", Smart: "passed", Status: "active"}},
		VMs:        []model.VM{{Name: "ubuntu", State: "running"}},
	}
	if messages := monitor.Observe(base); len(messages) != 0 {
		t.Fatalf("baseline produced alerts: %+v", messages)
	}
	changed := base
	changed.Containers = []model.Container{{Name: "plex", State: "exited", Health: ""}}
	changed.Disks = []model.Disk{{Name: "disk1", Smart: "warning", Status: "active"}}
	changed.VMs = []model.VM{{Name: "ubuntu", State: "shut off"}}
	messages := monitor.Observe(changed)
	if len(messages) != 3 {
		t.Fatalf("expected three alerts, got %+v", messages)
	}
	if messages[0].Source != "docker" || messages[1].Source != "disk" || messages[2].Source != "vm" {
		t.Fatalf("unexpected alert sources: %+v", messages)
	}
}
