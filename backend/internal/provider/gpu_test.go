package provider

import "testing"

func TestParseNvidiaSMI(t *testing.T) {
	raw := `0, GPU-abc, NVIDIA GeForce RTX 4080 SUPER, 580.95.05, 16376, 4096, 37, 56, 120.5, 320, 42, 2550, 11200
1, GPU-def, NVIDIA GeForce RTX 3060, 580.95.05, 12288, 0, 0, 39, 12, 170, 0, 210, 7000
`
	gpus, byUUID := parseNvidiaSMI(raw)
	if len(gpus) != 2 || gpus[0].Name != "NVIDIA GeForce RTX 4080 SUPER" {
		t.Fatalf("gpus = %+v", gpus)
	}
	if gpus[0].MemoryTotal != 16376*1024*1024 || gpus[0].Utilization != 37 || gpus[0].Power != 120.5 {
		t.Fatalf("first gpu = %+v", gpus[0])
	}
	if byUUID["GPU-def"] != 1 || gpus[0].Vendor != "NVIDIA" {
		t.Fatalf("uuid map = %+v", byUUID)
	}
}

func TestParseNvidiaProcesses(t *testing.T) {
	raw := "GPU-abc, 4242, /usr/lib/plexmediaserver/Plex Transcoder, 1024\n"
	processes := parseNvidiaProcesses(raw)
	if len(processes) != 1 || processes[0].Process.PID != 4242 || processes[0].Process.Memory != 1024*1024*1024 {
		t.Fatalf("processes = %+v", processes)
	}
}
