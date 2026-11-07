package provider

import (
	"context"
	"strings"
	"testing"
)

func TestRecordContainerImageChange(t *testing.T) {
	live := &Live{bootDir: t.TempDir()}
	if err := live.recordContainerImageChange("plex", "plex:latest", "sha256:111", "sha256:222",
		[]string{"sha256:aaa"}, []string{"sha256:bbb"}); err != nil {
		t.Fatal(err)
	}
	history, err := live.ConfigHistory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Action != "container.image.update" {
		t.Fatalf("unexpected history: %+v", history)
	}
	detail, err := live.ConfigHistoryDetail(context.Background(), history[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Files) != 1 {
		t.Fatalf("unexpected files: %+v", detail.Files)
	}
	file := detail.Files[0]
	if !file.Virtual || file.Path != "docker://plex/image" {
		t.Fatalf("unexpected virtual change: %+v", file)
	}
	if !strings.Contains(file.Diff, "-digest: sha256:aaa") || !strings.Contains(file.Diff, "+digest: sha256:bbb") {
		t.Fatalf("image digest diff is missing:\n%s", file.Diff)
	}
	if !strings.Contains(file.Diff, "-image-id: sha256:111") || !strings.Contains(file.Diff, "+image-id: sha256:222") {
		t.Fatalf("image id diff is missing:\n%s", file.Diff)
	}
	if err := live.RollbackConfig(context.Background(), detail.ID); err == nil {
		t.Fatal("virtual-only history entries must not be rollbackable")
	}
}

func TestRecordContainerImageChangeSkipsUnchangedDigest(t *testing.T) {
	live := &Live{bootDir: t.TempDir()}
	if err := live.recordContainerImageChange("plex", "plex:latest", "sha256:111", "sha256:111",
		[]string{"sha256:aaa"}, []string{"sha256:aaa"}); err != nil {
		t.Fatal(err)
	}
	history, err := live.ConfigHistory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Fatalf("unchanged image should not create history: %+v", history)
	}
}
