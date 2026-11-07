package provider

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"aurora/internal/model"
	"aurora/internal/outbound"
)

const (
	nvidiaRepo          = "unraid/unraid-nvidia-driver"
	nvidiaAPIBase       = "https://api.github.com/repos/" + nvidiaRepo
	nvidiaDownloadBase  = "https://github.com/" + nvidiaRepo + "/releases/download"
	nvidiaVersionsURL   = "https://raw.githubusercontent.com/" + nvidiaRepo + "/master/versions.json"
	nvidiaVersionsTTL   = 5 * time.Minute
	nvidiaDownloadLimit = 60 * time.Minute
)

var nvidiaLegacyMajors = []string{"470", "390", "340"}

func (l *Live) nvidiaPluginDir() string {
	if l.bootDir == "" {
		return ""
	}
	return filepath.Join(l.bootDir, "config", "plugins", "nvidia-driver")
}

func (l *Live) nvidiaSettingsPath() string {
	dir := l.nvidiaPluginDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "settings.cfg")
}

func (l *Live) nvidiaPackagesDir(kernel string) string {
	dir := l.nvidiaPluginDir()
	if dir == "" || kernel == "" {
		return ""
	}
	return filepath.Join(dir, "packages", nvidiaKernelBase(kernel))
}

func nvidiaKernelBase(kernel string) string {
	base, _, _ := strings.Cut(kernel, "-")
	return base
}

// Overridden by tests to simulate a specific Unraid kernel.
var nvidiaKernel = func(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "uname", "-r").Output()
	if err != nil {
		return "", fmt.Errorf("read kernel version: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (l *Live) nvidiaPluginInstalled() bool {
	if path := l.nvidiaSettingsPath(); path != "" {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return true
		}
	}
	if info, err := os.Stat("/usr/local/emhttp/plugins/nvidia-driver"); err == nil && info.IsDir() {
		return true
	}
	return false
}

func nvidiaBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "yes", "1":
		return true
	}
	return false
}

func parseNvidiaModinfo(output string) (version, license string) {
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "version":
			if version == "" {
				version = value
			}
		case "license":
			if license == "" {
				license = value
			}
		}
	}
	return version, license
}

func nvidiaLicenseName(license string) string {
	switch {
	case strings.TrimSpace(license) == "":
		return "none"
	case strings.EqualFold(strings.TrimSpace(license), "NVIDIA"):
		return "proprietary"
	default:
		return "opensource"
	}
}

// NvidiaStatus reports the official Nvidia Driver plugin state without
// installing anything: the running module version and any package staged for
// the next boot.
func (l *Live) NvidiaStatus(ctx context.Context) (model.NvidiaStatus, error) {
	status := model.NvidiaStatus{
		PluginInstalled: l.nvidiaPluginInstalled(),
		SelectedVersion: "latest",
		UpdateCheck:     true,
	}
	kernel, err := nvidiaKernel(ctx)
	if err == nil {
		status.Kernel = kernel
	}
	if path := l.nvidiaSettingsPath(); path != "" {
		if ini, readErr := parseINI(path); readErr == nil {
			values := ini.section("")
			status.SelectedVersion = configValue(values, "driver_version", "latest")
			status.UpdateCheck = nvidiaBool(configValue(values, "update_check", "true"))
		} else if !errors.Is(readErr, os.ErrNotExist) {
			return status, fmt.Errorf("read nvidia-driver settings: %w", readErr)
		}
	}

	gpus, _ := l.GPUs(ctx)
	for _, gpu := range gpus {
		if !strings.EqualFold(gpu.Vendor, "NVIDIA") {
			continue
		}
		status.GPUs = append(status.GPUs, model.NvidiaGPU{Index: gpu.Index, Name: gpu.Name, UUID: gpu.UUID})
		if status.InstalledVersion == "" && gpu.Driver != "" {
			status.InstalledVersion = gpu.Driver
		}
	}
	if out, infoErr := exec.CommandContext(ctx, "modinfo", "nvidia").Output(); infoErr == nil {
		version, license := parseNvidiaModinfo(string(out))
		if status.InstalledVersion == "" {
			status.InstalledVersion = version
		}
		status.License = nvidiaLicenseName(license)
	}
	status.DriverInstalled = status.InstalledVersion != "" || len(status.GPUs) > 0
	if status.License == "" {
		status.License = "none"
	}
	if kernel != "" {
		dir := l.nvidiaPackagesDir(kernel)
		if entries, readErr := os.ReadDir(dir); readErr == nil {
			newest := ""
			var newestAt time.Time
			for _, entry := range entries {
				name := entry.Name()
				if entry.IsDir() || !strings.HasSuffix(name, ".txz") {
					continue
				}
				info, statErr := entry.Info()
				if statErr != nil {
					continue
				}
				if newest == "" || info.ModTime().After(newestAt) {
					newest, newestAt = name, info.ModTime()
				}
			}
			if newest != "" {
				status.DownloadedPackage = newest
				status.DownloadedVersion = nvidiaVersionFromPackage(newest, kernel)
			}
		}
	}

	l.nvidiaMu.Lock()
	download := l.nvidiaDownload
	l.nvidiaMu.Unlock()
	if download.Active || download.Done || download.Error != "" {
		status.Download = &download
	}
	return status, nil
}

// NvidiaVersions lists the packages published for the running kernel. The
// result is cached briefly because GitHub rate-limits unauthenticated calls.
func (l *Live) NvidiaVersions(ctx context.Context, refresh bool) (model.NvidiaVersions, error) {
	kernel, err := nvidiaKernel(ctx)
	if err != nil {
		return model.NvidiaVersions{}, err
	}
	l.nvidiaMu.Lock()
	if !refresh && l.nvidiaVersionsKernel == kernel && time.Since(l.nvidiaVersionsAt) < nvidiaVersionsTTL {
		cached := l.nvidiaVersions
		l.nvidiaMu.Unlock()
		return cached, nil
	}
	l.nvidiaMu.Unlock()

	names, err := fetchNvidiaReleaseAssets(ctx, kernel)
	if err != nil {
		return model.NvidiaVersions{}, err
	}
	branches, branchErr := fetchNvidiaBranches(ctx)
	versions := buildNvidiaVersions(kernel, nvidiaAssetsFromNames(names, kernel), branches)
	if branchErr != nil {
		// Release assets still make latest and open source selectable.
		versions.BranchError = branchErr.Error()
	}

	l.nvidiaMu.Lock()
	l.nvidiaVersions = versions
	l.nvidiaVersionsKernel = kernel
	l.nvidiaVersionsAt = time.Now()
	l.nvidiaMu.Unlock()
	return versions, nil
}

// DownloadNvidiaDriver writes the selected version to the plugin settings and
// stages the matching package for the next boot. The running module is never
// replaced while the server is online.
func (l *Live) DownloadNvidiaDriver(ctx context.Context, selection string) (model.NvidiaStatus, error) {
	selection = strings.TrimSpace(selection)
	if selection == "" {
		return model.NvidiaStatus{}, errors.New("choose a driver version")
	}
	if !l.nvidiaPluginInstalled() {
		return model.NvidiaStatus{}, errors.New("the Nvidia Driver plugin is not installed")
	}
	versions, err := l.NvidiaVersions(ctx, false)
	if err != nil {
		return model.NvidiaStatus{}, err
	}
	version, openSource, err := resolveNvidiaSelection(versions, selection)
	if err != nil {
		return model.NvidiaStatus{}, err
	}
	kernel, err := nvidiaKernel(ctx)
	if err != nil {
		return model.NvidiaStatus{}, err
	}
	packageName := nvidiaPackageName(version, kernel, openSource)

	l.nvidiaMu.Lock()
	if l.nvidiaDownload.Active {
		l.nvidiaMu.Unlock()
		return model.NvidiaStatus{}, errors.New("a driver download is already running")
	}
	l.nvidiaDownload = model.NvidiaDownload{Active: true, Version: version, Package: packageName}
	l.nvidiaMu.Unlock()

	if err := l.saveNvidiaSelection(ctx, selection); err != nil {
		l.finishNvidiaDownload(err)
		return model.NvidiaStatus{}, err
	}
	go l.runNvidiaDownload(kernel, version, packageName)
	return l.NvidiaStatus(ctx)
}

func (l *Live) saveNvidiaSelection(ctx context.Context, selection string) error {
	path := l.nvidiaSettingsPath()
	if path == "" {
		return errors.New("boot directory is unavailable")
	}
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read nvidia-driver settings: %w", err)
	}
	content := string(raw)
	if strings.TrimSpace(content) == "" {
		content = "driver_version=latest\ndisable_xconfig=false\nupdate_check=true\n"
	}
	updates := map[string]string{"driver_version": selection}
	if selection != "latest" && selection != "latest_prb" && selection != "latest_nfb" {
		updates["update_check"] = "false"
		removeNvidiaUpdateCron(ctx)
	}
	return l.writeConfigFile("nvidia.settings", "Selected Nvidia driver "+selection, path, rewriteConfigValues(content, updates, false))
}

// removeNvidiaUpdateCron mirrors the plugin's update_version helper: picking a
// pinned or open source driver stops the daily branch auto-download.
func removeNvidiaUpdateCron(ctx context.Context) {
	if _, err := os.Stat("/usr/local/emhttp/plugins/nvidia-driver/include/update-check.sh"); err != nil {
		return
	}
	cronCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cronCtx, "crontab", "-l").Output()
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	filtered := make([]string, 0, len(lines))
	removed := false
	for _, line := range lines {
		if strings.Contains(line, "/usr/local/emhttp/plugins/nvidia-driver/include/update-check.sh") {
			removed = true
			continue
		}
		filtered = append(filtered, line)
	}
	if !removed {
		return
	}
	cmd := exec.CommandContext(cronCtx, "crontab", "-")
	cmd.Stdin = strings.NewReader(strings.Join(filtered, "\n") + "\n")
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("remove Nvidia update cron: %v (%s)", err, strings.TrimSpace(string(output)))
	}
}

func (l *Live) runNvidiaDownload(kernel, version, packageName string) {
	ctx, cancel := context.WithTimeout(context.Background(), nvidiaDownloadLimit)
	defer cancel()
	err := l.downloadNvidiaPackage(ctx, kernel, packageName)
	l.finishNvidiaDownload(err)
}

func (l *Live) finishNvidiaDownload(err error) {
	l.nvidiaMu.Lock()
	defer l.nvidiaMu.Unlock()
	l.nvidiaDownload.Active = false
	if err != nil {
		l.nvidiaDownload.Error = err.Error()
		return
	}
	l.nvidiaDownload.Done = true
	l.nvidiaDownload.Progress = 100
}

func (l *Live) setNvidiaProgress(progress int) {
	l.nvidiaMu.Lock()
	l.nvidiaDownload.Progress = progress
	l.nvidiaMu.Unlock()
}

func (l *Live) downloadNvidiaPackage(ctx context.Context, kernel, packageName string) error {
	dir := l.nvidiaPackagesDir(kernel)
	if dir == "" {
		return errors.New("boot directory is unavailable")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create nvidia package directory: %w", err)
	}
	target := filepath.Join(dir, packageName)
	partial := target + ".part"
	_ = os.Remove(partial)

	url := nvidiaDownloadBase + "/" + kernel + "/" + packageName
	client := outbound.Client(0)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Aurora")
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", packageName, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: GitHub returned %s", packageName, response.Status)
	}

	file, err := os.OpenFile(partial, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", partial, err)
	}
	hasher := md5.New()
	var written atomic.Int64
	progress := &nvidiaProgressWriter{written: &written, total: response.ContentLength, onProgress: l.setNvidiaProgress}
	if response.ContentLength <= 0 {
		l.setNvidiaProgress(-1)
	}
	_, copyErr := io.Copy(io.MultiWriter(file, hasher, progress), response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(partial)
		return fmt.Errorf("download %s: %w", packageName, copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(partial)
		return fmt.Errorf("write %s: %w", partial, closeErr)
	}

	expected, err := fetchNvidiaChecksum(ctx, client, url+".md5")
	if err != nil {
		_ = os.Remove(partial)
		return err
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(expected, actual) {
		_ = os.Remove(partial)
		return fmt.Errorf("checksum mismatch for %s", packageName)
	}
	if err := os.Rename(partial, target); err != nil {
		_ = os.Remove(partial)
		return fmt.Errorf("store %s: %w", packageName, err)
	}
	if err := os.WriteFile(target+".md5", []byte(expected+"\n"), 0o644); err != nil {
		return fmt.Errorf("write checksum: %w", err)
	}
	l.cleanNvidiaPackages(dir, packageName)
	return nil
}

func (l *Live) cleanNvidiaPackages(dir, keep string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == keep || name == keep+".md5" {
			continue
		}
		_ = os.Remove(filepath.Join(dir, name))
	}
}

type nvidiaProgressWriter struct {
	written    *atomic.Int64
	total      int64
	last       int
	onProgress func(int)
}

func (w *nvidiaProgressWriter) Write(p []byte) (int, error) {
	written := w.written.Add(int64(len(p)))
	if w.total > 0 {
		progress := int(written * 100 / w.total)
		if progress > w.last {
			w.last = progress
			w.onProgress(progress)
		}
	}
	return len(p), nil
}

func fetchNvidiaReleaseAssets(ctx context.Context, kernel string) ([]string, error) {
	target := nvidiaAPIBase + "/releases/tags/" + kernel
	client := outbound.Client(20 * time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Aurora")
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("contact GitHub releases: %w", err)
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, fmt.Errorf("no Nvidia driver package is available for kernel %s", kernel)
	case http.StatusForbidden, http.StatusTooManyRequests:
		return nil, errors.New(nvidiaRateLimitMessage(response))
	default:
		return nil, fmt.Errorf("GitHub releases returned %s", response.Status)
	}
	var payload struct {
		Assets []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode GitHub release: %w", err)
	}
	names := make([]string, 0, len(payload.Assets))
	for _, asset := range payload.Assets {
		names = append(names, asset.Name)
	}
	return names, nil
}

func fetchNvidiaBranches(ctx context.Context) (map[string][]string, error) {
	client := outbound.Client(20 * time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, nvidiaVersionsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Aurora")
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("contact driver branch list: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("driver branch list returned %s", response.Status)
	}
	var payload struct {
		Branches map[string]json.RawMessage `json:"branches"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode driver branches: %w", err)
	}
	out := map[string][]string{}
	for name, raw := range payload.Branches {
		out[name] = nvidiaBranchValues(raw)
	}
	return out, nil
}

func nvidiaBranchValues(raw json.RawMessage) []string {
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return list
	}
	var object map[string]string
	if json.Unmarshal(raw, &object) != nil {
		return nil
	}
	values := make([]string, 0, len(object))
	for _, value := range object {
		if strings.TrimSpace(value) != "" {
			values = append(values, value)
		}
	}
	return values
}

func nvidiaRateLimitMessage(response *http.Response) string {
	if reset := response.Header.Get("X-RateLimit-Reset"); reset != "" {
		if epoch, err := strconv.ParseInt(reset, 10, 64); err == nil {
			return "GitHub API rate limit reached; resets at " + time.Unix(epoch, 0).Local().Format(time.RFC3339)
		}
	}
	return "GitHub API rate limit reached; try again later"
}

type nvidiaAsset struct {
	Version    string
	Name       string
	OpenSource bool
}

func nvidiaAssetsFromNames(names []string, kernel string) []nvidiaAsset {
	out := make([]nvidiaAsset, 0, len(names))
	suffix := "-" + kernel + "-1.txz"
	for _, name := range names {
		var prefix string
		switch {
		case strings.HasPrefix(name, "nvidia-"):
			prefix = "nvidia-"
		case strings.HasPrefix(name, "nvos-"):
			prefix = "nvos-"
		default:
			continue
		}
		if !strings.HasSuffix(name, suffix) {
			continue
		}
		version := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
		if version == "" {
			continue
		}
		out = append(out, nvidiaAsset{Version: version, Name: name, OpenSource: prefix == "nvos-"})
	}
	return out
}

func buildNvidiaVersions(kernel string, assets []nvidiaAsset, branches map[string][]string) model.NvidiaVersions {
	proprietary := map[string]bool{}
	openSource := map[string]bool{}
	for _, asset := range assets {
		if asset.OpenSource {
			openSource[asset.Version] = true
		} else {
			proprietary[asset.Version] = true
		}
	}
	versions := nvidiaSortedVersions(proprietary)
	out := model.NvidiaVersions{Kernel: kernel, Versions: versions}
	if len(versions) > 0 {
		out.Latest = versions[0]
	}
	out.Production = nvidiaPickBranch(proprietary, branches["production"])
	out.NewFeature = nvidiaPickBranch(proprietary, branches["newfeature"])
	if open := nvidiaSortedVersions(openSource); len(open) > 0 {
		out.OpenSource = open[0]
	}
	return out
}

func nvidiaSortedVersions(set map[string]bool) []string {
	versions := make([]string, 0, len(set))
	for version := range set {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool {
		return compareOSVersions(versions[i], versions[j]) > 0
	})
	// Keep the newest versions plus the best package of each legacy major so
	// old cards can still pick a compatible driver.
	seenLegacy := map[string]bool{}
	filtered := make([]string, 0, len(versions))
	for _, version := range versions {
		major, _, _ := strings.Cut(version, ".")
		if containsString(nvidiaLegacyMajors, major) {
			if seenLegacy[major] {
				continue
			}
			seenLegacy[major] = true
		}
		filtered = append(filtered, version)
	}
	return filtered
}

func nvidiaPickBranch(available map[string]bool, candidates []string) string {
	best := ""
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if !available[candidate] {
			continue
		}
		if best == "" || compareOSVersions(candidate, best) > 0 {
			best = candidate
		}
	}
	return best
}

func nvidiaVersionFromPackage(name, kernel string) string {
	return strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(name, "nvidia-"), "nvos-"), "-"+kernel+"-1.txz")
}

func nvidiaPackageName(version, kernel string, openSource bool) string {
	prefix := "nvidia-"
	if openSource {
		prefix = "nvos-"
	}
	return prefix + version + "-" + kernel + "-1.txz"
}

func resolveNvidiaSelection(versions model.NvidiaVersions, selection string) (string, bool, error) {
	switch selection {
	case "latest":
		if versions.Latest == "" {
			return "", false, errors.New("no Nvidia driver package is available for this kernel")
		}
		return versions.Latest, false, nil
	case "latest_prb":
		if versions.Production == "" {
			return "", false, errors.New("no Production Branch driver is available for this kernel")
		}
		return versions.Production, false, nil
	case "latest_nfb":
		if versions.NewFeature == "" {
			return "", false, errors.New("no New Feature Branch driver is available for this kernel")
		}
		return versions.NewFeature, false, nil
	case "latest_nos":
		if versions.OpenSource == "" {
			return "", false, errors.New("no Open Source driver is available for this kernel")
		}
		return versions.OpenSource, true, nil
	}
	for _, version := range versions.Versions {
		if version == selection {
			return selection, false, nil
		}
	}
	return "", false, fmt.Errorf("driver version %s is not available for this kernel", selection)
}

func fetchNvidiaChecksum(ctx context.Context, client *http.Client, target string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Aurora")
	response, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download checksum: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download checksum: GitHub returned %s", response.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 4096))
	if err != nil {
		return "", fmt.Errorf("read checksum: %w", err)
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return "", errors.New("driver checksum file is empty")
	}
	return fields[0], nil
}
