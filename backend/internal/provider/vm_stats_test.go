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
