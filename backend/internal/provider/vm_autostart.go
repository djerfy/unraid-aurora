package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"aurora/internal/model"
)

const (
	vmAutostartFile = "vm-autostart.json"
	vmStartTimeout  = 90 * time.Second
)

type vmAutostartConfig struct {
	Order      []string  `json:"order"`
	Managed    bool      `json:"managed"`
	ManagedVMs []string  `json:"managedVms"`
	UpdatedAt  time.Time `json:"updatedAt"`
	AppliedFor string    `json:"appliedFor,omitempty"`
}

func (c vmAutostartConfig) model() model.VMAutostartOrder {
	return model.VMAutostartOrder{
		Order: c.Order, Managed: c.Managed, ManagedVMs: c.ManagedVMs, UpdatedAt: c.UpdatedAt,
	}
}

func (l *Live) loadVMAutostartConfig(ctx context.Context) (vmAutostartConfig, error) {
	path := l.configPath(vmAutostartFile)
	if path != "" {
		raw, err := os.ReadFile(path)
		if err == nil {
			var config vmAutostartConfig
			if err := json.Unmarshal(raw, &config); err != nil {
				return config, fmt.Errorf("parse VM autostart order: %w", err)
			}
			return config, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return vmAutostartConfig{}, err
		}
	}
	return vmAutostartConfig{Order: []string{}, ManagedVMs: []string{}}, nil
}

func (l *Live) VMAutostartOrder(ctx context.Context) (model.VMAutostartOrder, error) {
	config, err := l.loadVMAutostartConfig(ctx)
	if err != nil {
		return model.VMAutostartOrder{}, err
	}
	// Until the first save, expose the VMs that already autostart. This is only
	// done for the UI; the boot watcher must not enumerate domains every tick.
	if path := l.configPath(vmAutostartFile); path != "" {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			if vms, err := l.VMs(ctx); err == nil {
				order := make([]string, 0, len(vms))
				for _, vm := range vms {
					if vm.Autostart {
						order = append(order, vm.Name)
					}
				}
				config.Order = order
			}
		}
	}
	return config.model(), nil
}

func (l *Live) SetVMAutostartOrder(ctx context.Context, order []string, managed bool) (model.VMAutostartOrder, error) {
	l.vmAutostartMu.Lock()
	defer l.vmAutostartMu.Unlock()
	vms, err := l.VMs(ctx)
	if err != nil {
		return model.VMAutostartOrder{}, err
	}
	byName := make(map[string]model.VM, len(vms))
	for _, vm := range vms {
		byName[vm.Name] = vm
	}
	normalized := make([]string, 0, len(order))
	seen := make(map[string]bool, len(order))
	for _, raw := range order {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if _, ok := byName[name]; !ok {
			return model.VMAutostartOrder{}, fmt.Errorf("unknown virtual machine: %s", name)
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		normalized = append(normalized, name)
	}

	previous, err := l.loadVMAutostartConfig(ctx)
	if err != nil {
		return model.VMAutostartOrder{}, err
	}
	managedVMs := append([]string(nil), previous.ManagedVMs...)
	managedSet := make(map[string]bool, len(managedVMs))
	for _, name := range managedVMs {
		managedSet[name] = true
	}
	desired := make(map[string]bool, len(normalized))
	for _, name := range normalized {
		desired[name] = true
	}

	// Stop managing VMs that left the order: give libvirt autostart back.
	if previous.Managed {
		kept := managedVMs[:0]
		for _, name := range managedVMs {
			if desired[name] {
				kept = append(kept, name)
				continue
			}
			if _, err := exec.CommandContext(ctx, "virsh", "autostart", name).CombinedOutput(); err != nil {
				return model.VMAutostartOrder{}, fmt.Errorf("re-enable libvirt autostart for %s: %w", name, err)
			}
		}
		managedVMs = kept
		managedSet = make(map[string]bool, len(managedVMs))
		for _, name := range managedVMs {
			managedSet[name] = true
		}
	}

	if managed {
		for _, name := range normalized {
			if managedSet[name] {
				continue
			}
			if !byName[name].Autostart {
				// Nothing to take over: libvirt already does not start it.
				continue
			}
			if output, err := exec.CommandContext(ctx, "virsh", "autostart", "--disable", name).CombinedOutput(); err != nil {
				return model.VMAutostartOrder{}, fmt.Errorf("disable libvirt autostart for %s: %w (%s)", name, err, strings.TrimSpace(string(output)))
			}
			managedVMs = append(managedVMs, name)
			managedSet[name] = true
		}
	} else if previous.Managed {
		for _, name := range managedVMs {
			if output, err := exec.CommandContext(ctx, "virsh", "autostart", name).CombinedOutput(); err != nil {
				return model.VMAutostartOrder{}, fmt.Errorf("restore libvirt autostart for %s: %w (%s)", name, err, strings.TrimSpace(string(output)))
			}
		}
		managedVMs = []string{}
	}

	config := vmAutostartConfig{
		Order: normalized, Managed: managed, ManagedVMs: managedVMs, UpdatedAt: time.Now().UTC(),
		AppliedFor: previous.AppliedFor,
	}
	if err := l.writeVMAutostartConfig(config); err != nil {
		return model.VMAutostartOrder{}, err
	}
	l.recordConfigEvent("vm.autostart", "Updated the VM autostart order")
	return config.model(), nil
}

func (l *Live) writeVMAutostartConfig(config vmAutostartConfig) error {
	raw, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	path := l.configPath(vmAutostartFile)
	if path == "" {
		return errors.New("configuration directory is unavailable")
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("write VM autostart order: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write VM autostart order: %w", err)
	}
	return nil
}

func (l *Live) markVMAutostartApplied(arrayStartedAt string) {
	config, err := l.loadVMAutostartConfig(context.Background())
	if err != nil || config.AppliedFor == arrayStartedAt {
		return
	}
	config.AppliedFor = arrayStartedAt
	_ = l.writeVMAutostartConfig(config)
}

func (l *Live) ApplyVMAutostartOrder(ctx context.Context) (model.VMAutostartResult, error) {
	l.vmAutostartMu.Lock()
	defer l.vmAutostartMu.Unlock()
	config, err := l.loadVMAutostartConfig(ctx)
	if err != nil {
		return model.VMAutostartResult{}, err
	}
	if len(config.Order) == 0 {
		return model.VMAutostartResult{}, errors.New("no autostart order is configured")
	}
	result := model.VMAutostartResult{
		Started: []string{}, Skipped: []string{}, Failed: []model.VMAutostartFailure{},
	}
	for _, name := range config.Order {
		state, err := vmDomainRunState(ctx, name)
		if err == nil && (state == "running" || state == "paused") {
			result.Skipped = append(result.Skipped, name)
			continue
		}
		startCtx, cancel := context.WithTimeout(ctx, vmStartTimeout)
		output, err := exec.CommandContext(startCtx, "virsh", "start", name).CombinedOutput()
		cancel()
		if err != nil {
			result.Failed = append(result.Failed, model.VMAutostartFailure{
				Name: name, Error: strings.TrimSpace(string(output)) + " " + err.Error(),
			})
			continue
		}
		result.Started = append(result.Started, name)
	}
	l.recordConfigEvent("vm.autostart.apply", fmt.Sprintf("Started %d virtual machines in order", len(result.Started)))
	return result, nil
}

// RunVMAutostart watches the array state and applies the managed order once
// per array start. AppliedFor makes this idempotent across Aurora restarts.
// It is opt-in: nothing happens while Managed is false.
func (l *Live) RunVMAutostart(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		if array, err := l.Array(ctx); err == nil && array.State == "STARTED" && array.StartedAt != "" {
			config, err := l.loadVMAutostartConfig(ctx)
			if err == nil && config.Managed && len(config.Order) > 0 && config.AppliedFor != array.StartedAt {
				if _, err := l.ApplyVMAutostartOrder(ctx); err == nil {
					l.markVMAutostartApplied(array.StartedAt)
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
