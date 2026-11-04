package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"aurora/internal/model"
)

const shareUsageTTL = 6 * time.Hour

// shareUserRoot is a variable so tests can point the scanner at a temp tree.
var shareUserRoot = "/mnt/user"

type shareUsageEntry struct {
	Used     uint64    `json:"used"`
	Updated  time.Time `json:"updated"`
	Trigger  string    `json:"trigger,omitempty"`
	Duration int64     `json:"duration,omitempty"`
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

func (l *Live) cachedShareUsed(name string) (uint64, time.Time) {
	l.shareUsageMu.Lock()
	defer l.shareUsageMu.Unlock()
	entry := l.shareUsage[name]
	return entry.Used, entry.Updated
}

func (l *Live) applyShareUsage(share *model.Share) {
	used := share.Used
	updated := time.Time{}
	if used == 0 {
		used, updated = l.cachedShareUsed(share.Name)
	}
	if used == 0 {
		return
	}
	share.Used = used
	if !updated.IsZero() {
		value := updated
		share.UsageUpdated = &value
	}
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

// RefreshShareUsage forces a background scan instead of waiting for the TTL.
// An empty name refreshes every share.
func (l *Live) RefreshShareUsage(ctx context.Context, name string) (int, error) {
	_ = ctx
	names := []string{}
	if name != "" {
		name = strings.TrimSpace(name)
		if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, "\x00\n\r") {
			return 0, errors.New("invalid share name")
		}
		ini, err := parseINI(filepath.Join(l.stateDir, "shares.ini"))
		if err != nil {
			return 0, err
		}
		if _, ok := ini[name]; !ok {
			return 0, errors.New("share not found")
		}
		names = []string{name}
	} else {
		ini, err := parseINI(filepath.Join(l.stateDir, "shares.ini"))
		if err != nil {
			return 0, err
		}
		for shareName := range ini {
			names = append(names, shareName)
		}
		sort.Strings(names)
	}
	started := 0
	for _, shareName := range names {
		l.shareUsageMu.Lock()
		if l.shareUsageRefreshing[shareName] {
			l.shareUsageMu.Unlock()
			continue
		}
		l.shareUsageRefreshing[shareName] = true
		l.shareUsageStarted[shareName] = time.Now()
		l.shareUsageTrigger[shareName] = "manual"
		l.shareUsageMu.Unlock()
		started++
		go l.refreshShareUsage(shareName)
	}
	return started, nil
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
		l.shareUsageStarted[share.Name] = time.Now()
		l.shareUsageTrigger[share.Name] = "automatic"
		l.shareUsageMu.Unlock()
		go l.refreshShareUsage(share.Name)
	}
}

func (l *Live) refreshShareUsage(name string) {
	started := time.Now()
	l.shareUsageSem <- struct{}{}
	defer func() { <-l.shareUsageSem }()
	defer func() {
		l.shareUsageMu.Lock()
		delete(l.shareUsageRefreshing, name)
		delete(l.shareUsageStarted, name)
		delete(l.shareUsageTrigger, name)
		delete(l.shareUsagePID, name)
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
		command := exec.CommandContext(ctx, "du", "-s", "-k", "-x", path)
		var output bytes.Buffer
		command.Stdout = &output
		if err := command.Start(); err != nil {
			continue
		}
		l.shareUsageMu.Lock()
		l.shareUsagePID[name] = command.Process.Pid
		l.shareUsageMu.Unlock()
		waitErr := command.Wait()
		l.shareUsageMu.Lock()
		delete(l.shareUsagePID, name)
		l.shareUsageMu.Unlock()
		if waitErr != nil {
			continue
		}
		fields := strings.Fields(output.String())
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
		entry.Trigger = l.shareUsageTrigger[name]
		entry.Duration = int64(time.Since(started).Seconds())
		l.shareUsage[name] = entry
		l.shareUsageErrors[name] = "no readable share path"
		l.saveShareUsageLocked()
		l.shareUsageMu.Unlock()
		return
	}
	l.shareUsage[name] = shareUsageEntry{
		Used: total, Updated: time.Now(), Trigger: l.shareUsageTrigger[name],
		Duration: int64(time.Since(started).Seconds()),
	}
	delete(l.shareUsageErrors, name)
	l.saveShareUsageLocked()
	l.shareUsageMu.Unlock()
}

func (l *Live) sharePaths(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, "\x00\n\r") {
		return nil
	}
	// Shares are the /mnt/user view: it merges every data disk, cache pool and
	// secondary pool that contributes to the share. Scanning /mnt/diskX would
	// only see one isolated disk and miss data stored elsewhere.
	path := filepath.Join(shareUserRoot, name)
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return nil
	}
	return []string{path}
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
