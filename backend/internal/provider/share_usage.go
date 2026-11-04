package provider

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"aurora/internal/model"
)

const shareUsageTTL = 30 * time.Minute

type shareUsageEntry struct {
	Used    uint64    `json:"used"`
	Updated time.Time `json:"updated"`
}

func (l *Live) loadShareUsage() {
	raw, err := l.readConfigFile("share-usage.json")
	if err != nil {
		return
	}
	var file map[string]shareUsageEntry
	if json.Unmarshal(raw, &file) == nil {
		l.shareUsageMu.Lock()
		for name, entry := range file {
			l.shareUsage[name] = entry
		}
		l.shareUsageMu.Unlock()
	}
}

func (l *Live) saveShareUsageLocked() {
	raw, err := json.MarshalIndent(l.shareUsage, "", "  ")
	if err != nil {
		return
	}
	_ = l.saveConfigFile("share-usage.json", raw)
}

func (l *Live) cachedShareUsed(name string) uint64 {
	l.shareUsageMu.Lock()
	defer l.shareUsageMu.Unlock()
	return l.shareUsage[name].Used
}

func (l *Live) applyShareUsage(share *model.Share) {
	used := share.Used
	if used == 0 {
		used = l.cachedShareUsed(share.Name)
	}
	if used == 0 {
		return
	}
	share.Used = used
	size := share.Size
	if size == 0 {
		size = l.shareCapacity(share.CacheMode, share.Pool)
	}
	if size >= used {
		share.Size = size
		share.Free = size - used
		share.Usage = round1(float64(used) / float64(size) * 100)
	}
}

func (l *Live) scheduleShareUsage(shares []model.Share) {
	now := time.Now()
	for _, share := range shares {
		l.shareUsageMu.Lock()
		entry, ok := l.shareUsage[share.Name]
		stale := !ok || now.Sub(entry.Updated) >= shareUsageTTL
		if !stale || l.shareUsageRefreshing[share.Name] {
			l.shareUsageMu.Unlock()
			continue
		}
		l.shareUsageRefreshing[share.Name] = true
		l.shareUsageMu.Unlock()
		go l.refreshShareUsage(share.Name)
	}
}

func (l *Live) refreshShareUsage(name string) {
	l.shareUsageSem <- struct{}{}
	defer func() { <-l.shareUsageSem }()
	defer func() {
		l.shareUsageMu.Lock()
		delete(l.shareUsageRefreshing, name)
		l.shareUsageMu.Unlock()
	}()
	paths := l.sharePaths(name)
	if len(paths) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var total uint64
	scanned := false
	for _, path := range paths {
		out, err := exec.CommandContext(ctx, "du", "-s", "-k", "-x", path).Output()
		if err != nil {
			continue
		}
		fields := strings.Fields(string(out))
		if len(fields) == 0 {
			continue
		}
		kib, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		total += kib * 1024
		scanned = true
	}
	l.shareUsageMu.Lock()
	if !scanned {
		// Back off for the normal TTL after a failed scan instead of retrying
		// on every bootstrap.
		entry := l.shareUsage[name]
		entry.Updated = time.Now()
		l.shareUsage[name] = entry
		l.saveShareUsageLocked()
		l.shareUsageMu.Unlock()
		return
	}
	l.shareUsage[name] = shareUsageEntry{Used: total, Updated: time.Now()}
	l.saveShareUsageLocked()
	l.shareUsageMu.Unlock()
}

func (l *Live) sharePaths(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, "\x00\n\r") {
		return nil
	}
	ini, err := parseINI(filepath.Join(l.stateDir, "disks.ini"))
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	paths := []string{}
	add := func(path string) {
		if seen[path] {
			return
		}
		seen[path] = true
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			paths = append(paths, path)
		}
	}
	for diskName, disk := range ini {
		switch disk["type"] {
		case "Data":
			if strings.HasPrefix(diskName, "disk") {
				add(filepath.Join("/mnt", diskName, name))
			}
		case "Cache":
			if pool := strings.TrimSpace(disk["pool"]); pool != "" {
				add(filepath.Join("/mnt", pool, name))
			}
		}
	}
	add(filepath.Join("/mnt/cache", name))
	sort.Strings(paths)
	return paths
}

func (l *Live) shareCapacity(useCache, pool string) uint64 {
	ini, err := parseINI(filepath.Join(l.stateDir, "disks.ini"))
	if err != nil {
		return 0
	}
	var data uint64
	pools := map[string]uint64{}
	for _, disk := range ini {
		size := parseKiB(disk["fsSize"])
		if size == 0 {
			size = parseKiB(disk["size"])
		}
		switch disk["type"] {
		case "Data":
			data += size
		case "Cache":
			name := strings.TrimSpace(disk["pool"])
			if name != "" && size > pools[name] {
				pools[name] = size
			}
		}
	}
	poolName := strings.TrimSpace(pool)
	poolSize := pools[poolName]
	if poolSize == 0 && poolName != "" {
		if size, _, ok := statfsUsage(filepath.Join("/mnt", poolName)); ok {
			poolSize = size
		}
	}
	switch strings.ToLower(useCache) {
	case "only":
		if poolSize > 0 {
			return poolSize
		}
		return data
	case "yes", "prefer":
		return data + poolSize
	default:
		return data
	}
}
