package provider

import "testing"

func TestContainerMounts(t *testing.T) {
	got := containerMounts([]dockerMount{
		{Type: "bind", Source: "/mnt/user/media", Destination: "/media", RW: false},
		{Type: "volume", Source: "plex-config", Destination: "/config", RW: true},
		{Type: "bind", Source: "/mnt/user/appdata/plex", Destination: "/config", RW: true},
		{Type: "bind", Source: "", Destination: "/broken", RW: true},
	})
	if len(got) != 2 {
		t.Fatalf("got %d mounts, want 2: %+v", len(got), got)
	}
	if got[0].Destination != "/config" || got[0].Source != "/mnt/user/appdata/plex" || got[0].ReadOnly {
		t.Fatalf("unexpected first mount: %+v", got[0])
	}
	if got[1].Destination != "/media" || got[1].Source != "/mnt/user/media" || !got[1].ReadOnly {
		t.Fatalf("unexpected second mount: %+v", got[1])
	}
}
