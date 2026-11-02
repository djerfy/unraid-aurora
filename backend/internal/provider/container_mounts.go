package provider

import (
	"sort"

	"aurora/internal/model"
)

type dockerMount struct {
	Type        string `json:"Type"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
	RW          bool   `json:"RW"`
}

// containerMounts keeps bind mounts only: named volumes and tmpfs mounts do
// not expose a host path, so they are noise in the container list.
func containerMounts(mounts []dockerMount) []model.ContainerMount {
	out := make([]model.ContainerMount, 0, len(mounts))
	for _, mount := range mounts {
		if mount.Type != "bind" || mount.Source == "" || mount.Destination == "" {
			continue
		}
		out = append(out, model.ContainerMount{
			Type:        mount.Type,
			Source:      mount.Source,
			Destination: mount.Destination,
			ReadOnly:    !mount.RW,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Destination == out[j].Destination {
			return out[i].Source < out[j].Source
		}
		return out[i].Destination < out[j].Destination
	})
	return out
}
