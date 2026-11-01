package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"aurora/internal/model"
)

func TestBuildArrayStartForm(t *testing.T) {
	values, err := buildArrayStartForm(model.ArrayStartOptions{KeyType: "none"})
	if err != nil || values.Get("cmdStart") != "Start" {
		t.Fatalf("start form: %v %v", values, err)
	}
	values, err = buildArrayStartForm(model.ArrayStartOptions{KeyType: "none", Mode: "maintenance"})
	if err != nil || values.Get("startMode") != "Maintenance" {
		t.Fatalf("maintenance form: %v %v", values, err)
	}
	values, err = buildArrayStartForm(model.ArrayStartOptions{KeyType: "passphrase", Key: "secret pass"})
	if err != nil || values.Get("luksKey") != "c2VjcmV0IHBhc3M=" {
		t.Fatalf("passphrase form: %v %v", values, err)
	}
	if _, err := buildArrayStartForm(model.ArrayStartOptions{KeyType: "passphrase", Key: "bad\nkey"}); err == nil {
		t.Fatal("control characters should be rejected")
	}
	if _, err := buildArrayStartForm(model.ArrayStartOptions{KeyType: "keyfile"}); err == nil {
		t.Fatal("empty keyfile should be rejected")
	}
}

func TestMoverStatusParsing(t *testing.T) {
	dir := t.TempDir()
	content := `TotalSize=1000
RemainSize=250
Throughput=1024
ETA=60
ShareTotal=500
ShareRemain=100
ShareThroughput=512
ShareETA=30
MoverPid=123
File="/mnt/cache/media/movie.mkv"
Action="Moving to array"
Filepercent=42.5
`
	if err := os.WriteFile(filepath.Join(dir, "mover.ini"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	live := &Live{stateDir: dir}
	status, err := live.Mover(context.Background())
	if err != nil {
		t.Fatalf("mover: %v", err)
	}
	if status.Running {
		t.Fatal("mover should not be running without a script")
	}
	if status.Progress != 75 || status.Total != 1000 || status.Remain != 250 {
		t.Fatalf("unexpected mover status: %+v", status)
	}
	if status.File != "/mnt/cache/media/movie.mkv" || status.FilePercent != 42.5 || status.ETA != 60 {
		t.Fatalf("unexpected mover details: %+v", status)
	}
}
