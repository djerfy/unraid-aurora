package provider

import (
	"context"
	"testing"
)

func TestInstallPluginRejectsInvalidURL(t *testing.T) {
	live := &Live{}
	for _, target := range []string{"file:///etc/passwd", "ftp://example.com/x.plg", "", "not a url"} {
		if err := live.InstallPlugin(context.Background(), target, nil); err == nil {
			t.Fatalf("%q should be rejected", target)
		}
	}
}

func TestRemovePluginUnknown(t *testing.T) {
	live := &Live{bootDir: t.TempDir()}
	if err := live.RemovePlugin(context.Background(), "missing", nil); err == nil {
		t.Fatal("unknown plugin should fail")
	}
}
