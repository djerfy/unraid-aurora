package provider

import (
	"context"
	"testing"
)

func TestValidImageID(t *testing.T) {
	for _, id := range []string{"sha256:abcdef123456", "abcdef123456"} {
		if !validImageID(id) {
			t.Fatalf("%q should be valid", id)
		}
	}
	for _, id := range []string{"", "bad/id", "bad id", "bad\tid", "..", "sha256:xyz"} {
		if validImageID(id) {
			t.Fatalf("%q should be invalid", id)
		}
	}
}

func TestBuildContainerImages(t *testing.T) {
	images := []dockerImageInfo{
		{ID: "sha256:used", RepoTags: []string{"repo:latest"}, Size: 100, Created: 10},
		{ID: "sha256:free", RepoTags: nil, Size: 200, Created: 20},
	}
	containers := []dockerContainerImageRef{
		{ID: "container1", Names: []string{"/plex"}, ImageID: "sha256:used"},
		{ID: "container2", Names: []string{"/plex2"}, ImageID: "sha256:used"},
	}
	out := buildContainerImages(images, containers)
	if len(out) != 2 {
		t.Fatalf("unexpected images: %+v", out)
	}
	if out[0].ID != "sha256:free" || out[0].InUse || !out[0].Dangling {
		t.Fatalf("unused dangling image should come first: %+v", out[0])
	}
	if out[1].ID != "sha256:used" || !out[1].InUse || len(out[1].UsedBy) != 2 {
		t.Fatalf("used image should list its containers: %+v", out[1])
	}
}

func TestRemoveContainerImageRejectsInvalidID(t *testing.T) {
	live := &Live{}
	if err := live.RemoveContainerImage(context.Background(), "../etc/passwd"); err == nil {
		t.Fatal("invalid image id must be rejected")
	}
}
