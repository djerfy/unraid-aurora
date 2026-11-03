package provider

import "testing"

func TestParseZFSPools(t *testing.T) {
	raw := "tank\t1000000000000\t400000000000\t600000000000\tONLINE\nbackup\t500000000000\t100000000000\t400000000000\tDEGRADED\n"
	pools := parseZFSPools(raw)
	if len(pools) != 2 || pools[0].Name != "tank" || pools[1].Health != "DEGRADED" {
		t.Fatalf("unexpected pools: %+v", pools)
	}
	if pools[0].Size != 1_000_000_000_000 || pools[0].Allocated != 400_000_000_000 {
		t.Fatalf("unexpected pool sizes: %+v", pools[0])
	}
}

func TestParseZFSDatasets(t *testing.T) {
	raw := "tank/data\t1000\t2000\t3000\t/mnt/user/data\tfilesystem\ntank/data@snap\t100\t0\t0\t-\tsnapshot\n"
	datasets := parseZFSDatasets(raw)
	if len(datasets) != 2 || datasets[0].Name != "tank/data" || datasets[0].Mountpoint != "/mnt/user/data" {
		t.Fatalf("unexpected datasets: %+v", datasets)
	}
}
