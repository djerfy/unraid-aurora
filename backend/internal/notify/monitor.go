package notify

import (
	"fmt"
	"time"

	"aurora/internal/model"
)

// Monitor turns state transitions into Aurora alerts. The first observation is
// a baseline so a restart does not replay every current condition.
type Monitor struct {
	initialized bool
	parity      bool
	containers  map[string]string
	disks       map[string]string
	vms         map[string]string
}

func (m *Monitor) Observe(snapshot model.Snapshot) []Message {
	if m.containers == nil {
		m.containers = map[string]string{}
		m.disks = map[string]string{}
		m.vms = map[string]string{}
	}
	if !m.initialized {
		m.initialized = true
		m.parity = snapshot.Array.Parity.Running
		for _, container := range snapshot.Containers {
			m.containers[container.Name] = container.Health + "|" + container.State
		}
		for _, disk := range snapshot.Disks {
			m.disks[disk.Name] = disk.Smart + "|" + disk.Status
		}
		for _, vm := range snapshot.VMs {
			m.vms[vm.Name] = vm.State
		}
		return nil
	}

	var messages []Message
	add := func(title, description, importance, source string) {
		messages = append(messages, Message{
			Title: title, Description: description, Importance: importance,
			Source: source, Timestamp: time.Now(),
		})
	}
	if !m.parity && snapshot.Array.Parity.Running {
		add("Parity check started", "A parity operation started on the array.", "normal", "array")
	} else if m.parity && !snapshot.Array.Parity.Running {
		add("Parity check finished", "The parity operation is no longer running.", "warning", "array")
	}
	m.parity = snapshot.Array.Parity.Running

	for _, container := range snapshot.Containers {
		previous, seen := m.containers[container.Name]
		current := container.Health + "|" + container.State
		m.containers[container.Name] = current
		if !seen {
			continue
		}
		if previous != current {
			state := container.State
			if container.Health == "unhealthy" {
				state = "unhealthy"
			}
			if state != "running" {
				add("Container "+container.Name+" is "+state, "Docker reported a state change for "+container.Name+".", "warning", "docker")
			}
		}
	}
	for _, disk := range snapshot.Disks {
		previous, seen := m.disks[disk.Name]
		current := disk.Smart + "|" + disk.Status
		m.disks[disk.Name] = current
		if !seen || previous == current {
			continue
		}
		if disk.Smart != "passed" || disk.Status == "missing" {
			add("Disk "+disk.Name+" needs attention", fmt.Sprintf("SMART: %s, status: %s", disk.Smart, disk.Status), "alert", "disk")
		}
	}
	for _, vm := range snapshot.VMs {
		previous, seen := m.vms[vm.Name]
		m.vms[vm.Name] = vm.State
		if seen && previous == "running" && vm.State != "running" && vm.State != "paused" {
			add("VM "+vm.Name+" stopped", "The virtual machine is no longer running.", "warning", "vm")
		}
	}
	return messages
}
