package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestVMDetailsFromVirsh(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
case "$1" in
  list) echo "Windows 11" ;;
  domstate) echo "running" ;;
  dominfo) printf 'CPU(s):         4\nMax memory:     4194304 KiB\nAutostart:      enable\n' ;;
  dommemstat) printf 'actual 4194304\nunused 1048576\nrss 2097152\n' ;;
  domstats) printf 'state.state=1\ncpu.time=1000000000\nballoon.maximum=4194304\n' ;;
  domblklist) printf ' Type   Device   Target   Source\n file   disk     vda      /tmp/disk.img\n' ;;
  domblkinfo) printf 'Capacity:       1000\nAllocation:     400\nPhysical:       400\n' ;;
esac
`
	path := filepath.Join(dir, "virsh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	live := &Live{vmCPU: map[string]vmCPUSample{}}
	vm := live.vmDetails(context.Background(), "Windows 11")
	if vm.State != "running" || vm.VCPUs != 4 || !vm.Autostart {
		t.Fatalf("unexpected vm: %+v", vm)
	}
	if vm.Memory != 4194304*1024 {
		t.Fatalf("memory = %d", vm.Memory)
	}
	if vm.MemoryUsage != 50 {
		t.Fatalf("memory usage = %v", vm.MemoryUsage)
	}
	if vm.DiskSize != 1000 || vm.DiskUsed != 400 {
		t.Fatalf("disk = %d/%d", vm.DiskSize, vm.DiskUsed)
	}
}

func TestParseDomainGraphics(t *testing.T) {
	raw := []byte(`<domain type='kvm'>
  <name>win11</name>
  <devices>
    <graphics type='vnc' port='5901' autoport='yes' websocket='5701' listen='0.0.0.0'/>
  </devices>
</domain>`)
	console, err := parseDomainGraphics(raw, "win11")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if console.Name != "win11" || console.Protocol != "vnc" || console.Port != 5901 || console.WebSocket != 5701 {
		t.Fatalf("console = %+v", console)
	}

	raw = []byte(`<domain><devices><graphics type='spice' port='5902'/></devices></domain>`)
	console, err = parseDomainGraphics(raw, "linux")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if console.Protocol != "spice" || console.WebSocket != 0 {
		t.Fatalf("spice fallback = %+v", console)
	}
}
