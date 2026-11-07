package provider

import (
	"fmt"
	"sort"
	"strings"
)

// recordContainerImageChange stores the resolved image digest difference in
// History. It is informational: the entry has no file backup, so rollback is
// disabled for it.
func (l *Live) recordContainerImageChange(name, image, oldID, newID string, oldDigests, newDigests []string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	oldText := containerImageSnapshot(image, oldID, oldDigests)
	newText := containerImageSnapshot(image, newID, newDigests)
	if oldText == newText {
		return nil
	}
	path := "docker://" + name + "/image"
	return l.recordHistoryEvent(
		"container.image.update",
		"Updated container "+name+" ("+image+")",
		[]historyFile{{
			Path: path, Existed: true, Virtual: true,
			Diff: renderConfigDiff(path, []byte(oldText), []byte(newText), false),
			Size: int64(len(oldText)),
		}},
	)
}

func containerImageSnapshot(image, id string, digests []string) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "image: %s\n", image)
	if id != "" {
		fmt.Fprintf(&builder, "image-id: %s\n", id)
	}
	sorted := append([]string(nil), digests...)
	sort.Strings(sorted)
	if len(sorted) == 0 {
		builder.WriteString("digest: (unknown)\n")
		return builder.String()
	}
	for _, digest := range sorted {
		fmt.Fprintf(&builder, "digest: %s\n", digest)
	}
	return builder.String()
}
