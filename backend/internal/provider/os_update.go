package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"aurora/internal/model"
)

// Overridden by tests to point at a local fixture server.
var osReleasesBaseURL = "https://releases.unraid.net"

const (
	osCacheFile      = "os-update.json"
	unraidVersionINI = "/etc/unraid-version"
	unraidOSReadme   = "/usr/local/emhttp/plugins/unRAIDServer/README.md"
	unraidOSPlugin   = "/var/log/plugins/unRAIDServer.plg"
)

type osCheckResponse struct {
	Version         string `json:"version"`
	Name            string `json:"name"`
	Date            string `json:"date"`
	IsNewer         bool   `json:"isNewer"`
	IsEligible      bool   `json:"isEligible"`
	SHA256          string `json:"sha256"`
	Changelog       string `json:"changelog"`
	ChangelogPretty string `json:"changelogPretty"`
	Error           string `json:"error"`
}

type osReleaseResponse struct {
	Version         string `json:"version"`
	Name            string `json:"name"`
	Date            string `json:"date"`
	SHA256          string `json:"sha256"`
	Changelog       string `json:"changelog"`
	ChangelogPretty string `json:"changelog_pretty"`
	PluginURL       string `json:"plugin_url"`
}

type osUpdateCache struct {
	CheckedAt      time.Time `json:"checkedAt"`
	Branch         string    `json:"branch"`
	CurrentVersion string    `json:"currentVersion"`
	Latest         string    `json:"latest"`
	LatestName     string    `json:"latestName"`
	Update         bool      `json:"update"`
	Eligible       bool      `json:"eligible"`
	Changelog      string    `json:"changelog"`
	ChangelogURL   string    `json:"changelogPretty"`
	ReleaseDate    string    `json:"releaseDate"`
	PluginURL      string    `json:"pluginUrl"`
}

// OSStatus reports the installed release, pending reboot and the last update
// check without making a network call.
func (l *Live) OSStatus(ctx context.Context) (model.OSStatus, error) {
	status := model.OSStatus{
		Version: readUnraidVersion(),
		Branch:  l.osBranch(ctx),
	}
	if cache, err := l.loadOSCache(); err == nil && cache != nil {
		checked := cache.CheckedAt
		status.CheckedAt = &checked
		status.Latest = cache.Latest
		status.LatestName = cache.LatestName
		status.UpdateAvailable = cache.Update
		status.Eligible = cache.Eligible
		status.Changelog = cache.Changelog
		status.ChangelogPretty = cache.ChangelogURL
		status.ReleaseDate = cache.ReleaseDate
	}
	l.fillOSDetails(&status)
	return status, nil
}

// CheckOSUpdate asks releases.unraid.net for the newest release on the current
// branch. The result is cached on the flash drive so the UI can show it later.
func (l *Live) CheckOSUpdate(ctx context.Context) (model.OSStatus, error) {
	l.osMu.Lock()
	defer l.osMu.Unlock()

	status, _ := l.OSStatus(ctx)
	query := url.Values{}
	query.Set("branch", status.Branch)
	query.Set("current_version", status.Version)
	if expiry := l.updateExpiry(); expiry != "" {
		query.Set("update_exp", expiry)
	}
	var check osCheckResponse
	if err := l.fetchOSJSON(ctx, osReleasesBaseURL+"/os?"+query.Encode(), nil, &check); err != nil {
		status.CheckError = err.Error()
		return status, err
	}
	if check.Error != "" {
		status.CheckError = check.Error
		return status, errors.New(check.Error)
	}

	checked := time.Now()
	status.CheckedAt = &checked
	status.Latest = check.Version
	status.LatestName = check.Name
	status.UpdateAvailable = check.IsNewer
	status.Eligible = check.IsEligible
	status.Changelog = check.Changelog
	status.ChangelogPretty = check.ChangelogPretty
	status.ReleaseDate = check.Date

	cache := osUpdateCache{
		CheckedAt: checked, Branch: status.Branch, CurrentVersion: status.Version,
		Latest: check.Version, LatestName: check.Name, Update: check.IsNewer,
		Eligible: check.IsEligible, Changelog: check.Changelog,
		ChangelogURL: check.ChangelogPretty, ReleaseDate: check.Date,
	}
	if check.IsNewer && isHexSHA256(check.SHA256) {
		release, err := l.fetchOSRelease(ctx, check.SHA256)
		if err != nil {
			slog.Warn("resolve Unraid OS release", "version", check.Version, "error", err)
			status.CheckError = err.Error()
		} else {
			cache.PluginURL = release.PluginURL
			if release.Changelog != "" {
				cache.Changelog = release.Changelog
			}
			if release.ChangelogPretty != "" {
				cache.ChangelogURL = release.ChangelogPretty
			}
			if release.Date != "" {
				cache.ReleaseDate = release.Date
			}
			status.Changelog = cache.Changelog
			status.ChangelogPretty = cache.ChangelogURL
			status.ReleaseDate = cache.ReleaseDate
		}
	}
	if err := l.saveOSCache(cache); err != nil {
		slog.Warn("persist Unraid OS update check", "error", err)
	}
	l.fillOSDetails(&status)
	return status, nil
}

// InstallOSUpdate runs the official plugin manager against the release plugin,
// which is exactly how the WebGUI and Unraid Connect install an OS update.
func (l *Live) InstallOSUpdate(ctx context.Context, report func(model.PluginEvent)) error {
	emit := func(event model.PluginEvent) {
		if report != nil {
			report(event)
		}
	}
	cache, err := l.loadOSCache()
	if err != nil || cache == nil || cache.PluginURL == "" || time.Since(cache.CheckedAt) > 15*time.Minute {
		if _, err := l.CheckOSUpdate(ctx); err != nil {
			return err
		}
		cache, err = l.loadOSCache()
		if err != nil {
			return err
		}
	}
	if cache == nil || cache.PluginURL == "" {
		return errors.New("no Unraid OS update is available")
	}
	if err := validateReleaseURL(cache.PluginURL); err != nil {
		return err
	}
	if info, err := os.Stat(pluginManagerScript); err != nil || info.IsDir() {
		return errors.New("the Unraid plugin manager is not available")
	}
	emit(model.PluginEvent{Line: fmt.Sprintf("Installing %s", cache.Latest)})
	command := exec.CommandContext(ctx, pluginManagerScript, "install", cache.PluginURL, "forced")
	writer := &pluginLineWriter{emit: func(line string) { emit(model.PluginEvent{Line: line}) }}
	command.Stdout = writer
	command.Stderr = writer
	runErr := command.Run()
	writer.flush()
	if runErr != nil {
		emit(model.PluginEvent{Line: runErr.Error(), Error: true, Done: true})
		return runErr
	}
	emit(model.PluginEvent{Done: true, Line: "Unraid OS files installed — reboot required"})
	return nil
}

// RestorePreviousOS swaps the boot files kept in /boot/previous back into
// /boot, mirroring the official Downgrade/Cancel Update flow.
func (l *Live) RestorePreviousOS(ctx context.Context) (string, error) {
	boot := l.bootPath()
	previous := filepath.Join(boot, "previous")
	if info, err := os.Stat(filepath.Join(previous, "bzroot")); err != nil || info.IsDir() {
		return "", errors.New("no previous Unraid OS version is available")
	}
	version, _ := changesVersion(filepath.Join(previous, "changes.txt"))
	temporary, err := os.MkdirTemp(boot, "deletemedowngrade.")
	if err != nil {
		return "", fmt.Errorf("create downgrade staging directory: %w", err)
	}
	moved := make([]string, 0)
	restored := make([]string, 0)
	rollback := func() {
		for _, name := range restored {
			_ = os.Rename(filepath.Join(boot, name), filepath.Join(previous, name))
		}
		for _, name := range moved {
			_ = os.Rename(filepath.Join(temporary, name), filepath.Join(boot, name))
		}
		_ = os.RemoveAll(temporary)
	}
	entries, err := os.ReadDir(boot)
	if err != nil {
		rollback()
		return "", fmt.Errorf("read boot directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "bz") {
			continue
		}
		if err := os.Rename(filepath.Join(boot, entry.Name()), filepath.Join(temporary, entry.Name())); err != nil {
			rollback()
			return "", fmt.Errorf("stage boot file %s: %w", entry.Name(), err)
		}
		moved = append(moved, entry.Name())
	}
	previousEntries, err := os.ReadDir(previous)
	if err != nil {
		rollback()
		return "", fmt.Errorf("read previous boot directory: %w", err)
	}
	for _, entry := range previousEntries {
		if err := os.Rename(filepath.Join(previous, entry.Name()), filepath.Join(boot, entry.Name())); err != nil {
			rollback()
			return "", fmt.Errorf("restore previous boot file %s: %w", entry.Name(), err)
		}
		restored = append(restored, entry.Name())
	}
	_ = os.RemoveAll(temporary)
	_ = os.RemoveAll(filepath.Join(boot, "config", "plugins-nextboot"))
	if version != "" {
		_ = os.WriteFile(unraidOSReadme, []byte("**DOWNGRADE TO VERSION "+version+"**\n"), 0o644)
	}
	_ = ctx
	return version, nil
}

func (l *Live) fetchOSJSON(ctx context.Context, endpoint string, headers map[string]string, target any) error {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return errors.New("invalid Unraid release endpoint")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	req.Header.Set("Accept", "application/json")
	client := l.webClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("contact Unraid release service: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Unraid release service returned HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode Unraid release response: %w", err)
	}
	return nil
}

func (l *Live) fetchOSRelease(ctx context.Context, sha string) (osReleaseResponse, error) {
	var release osReleaseResponse
	headers := map[string]string{}
	if key := l.registrationKey(); key != "" {
		headers["X-Unraid-Keyfile"] = key
	}
	if err := l.fetchOSJSON(ctx, osReleasesBaseURL+"/sha256/"+sha, headers, &release); err != nil {
		return release, err
	}
	return release, nil
}

func (l *Live) registrationKey() string {
	varIni, _ := parseINI(filepath.Join(l.stateDir, "var.ini"))
	path := varIni.get("", "regFILE")
	if path == "" {
		path = varIni.get("", "regFile")
	}
	if path == "" {
		return ""
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	return strings.TrimRight(strings.NewReplacer("+", "-", "/", "_").Replace(encoded), "=")
}

func (l *Live) updateExpiry() string {
	varIni, _ := parseINI(filepath.Join(l.stateDir, "var.ini"))
	raw := strings.TrimSpace(varIni.get("", "regExp"))
	if raw == "" {
		return ""
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return ""
	}
	return time.Unix(value, 0).Format("2006-01-02")
}

func (l *Live) osBranch(ctx context.Context) string {
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(runCtx, pluginManagerScript, "category", unraidOSPlugin).Output()
	if err == nil {
		if branch := strings.TrimSpace(string(output)); branch != "" {
			return branch
		}
	}
	return "stable"
}

func (l *Live) fillOSDetails(status *model.OSStatus) {
	if status.Branch == "" {
		status.Branch = "stable"
	}
	if readme, err := os.ReadFile(unraidOSReadme); err == nil {
		head := string(readme)
		if len(head) > 32 {
			head = head[:32]
		}
		if regexp.MustCompile(`^\*\*(REBOOT REQUIRED|DOWNGRADE)`).MatchString(head) {
			if version, _ := changesVersion(filepath.Join(l.bootPath(), "changes.txt")); version != "" && status.Version != "" {
				status.RebootRequired = true
				status.RebootVersion = version
				switch compareOSVersions(version, status.Version) {
				case -1:
					status.RebootType = "downgrade"
				case 1:
					status.RebootType = "update"
				}
				if status.RebootType != "" && thirdPartyDriverUpdatePending() {
					status.ThirdPartyDrivers = true
				}
			}
		}
	}
	previous := filepath.Join(l.bootPath(), "previous")
	if info, err := os.Stat(filepath.Join(previous, "bzroot")); err == nil && !info.IsDir() {
		status.RestoreAvailable = true
		if version, date := changesVersion(filepath.Join(previous, "changes.txt")); version != "" {
			status.PreviousVersion = version
			status.PreviousDate = date
		}
	}
}

func (l *Live) loadOSCache() (*osUpdateCache, error) {
	raw, err := l.readConfigFile(osCacheFile)
	if err != nil {
		return nil, err
	}
	var cache osUpdateCache
	if err := json.Unmarshal(raw, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

func (l *Live) saveOSCache(cache osUpdateCache) error {
	raw, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return l.saveConfigFile(osCacheFile, raw)
}

func (l *Live) bootPath() string {
	if l.bootDir != "" {
		return l.bootDir
	}
	return "/boot"
}

func readUnraidVersion() string {
	ini, err := parseINI(unraidVersionINI)
	if err != nil {
		return ""
	}
	return ini.get("", "version")
}

func changesVersion(path string) (string, string) {
	file, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer file.Close()
	pattern := regexp.MustCompile(`(?i)version\s+(\S+)(?:\s+(\S+))?`)
	buffer := make([]byte, 4096)
	read, _ := file.Read(buffer)
	for _, line := range strings.Split(string(buffer[:read]), "\n") {
		if matches := pattern.FindStringSubmatch(line); len(matches) > 1 {
			date := ""
			if len(matches) > 2 {
				date = matches[2]
			}
			return matches[1], date
		}
	}
	return "", ""
}

func compareOSVersions(a, b string) int {
	coreA, preA := splitOSVersion(a)
	coreB, preB := splitOSVersion(b)
	count := len(coreA)
	if len(coreB) > count {
		count = len(coreB)
	}
	for index := 0; index < count; index++ {
		left, right := "0", "0"
		if index < len(coreA) {
			left = coreA[index]
		}
		if index < len(coreB) {
			right = coreB[index]
		}
		if result := compareVersionPart(left, right); result != 0 {
			return result
		}
	}
	switch {
	case preA == "" && preB == "":
		return 0
	case preA == "":
		return 1
	case preB == "":
		return -1
	}
	partsA := strings.Split(preA, ".")
	partsB := strings.Split(preB, ".")
	partCount := len(partsA)
	if len(partsB) > partCount {
		partCount = len(partsB)
	}
	for index := 0; index < partCount; index++ {
		left, right := "", ""
		if index < len(partsA) {
			left = partsA[index]
		}
		if index < len(partsB) {
			right = partsB[index]
		}
		if result := compareVersionPart(left, right); result != 0 {
			return result
		}
	}
	return 0
}

func splitOSVersion(raw string) ([]string, string) {
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "v")
	core, pre, _ := strings.Cut(raw, "-")
	return strings.Split(core, "."), pre
}

func compareVersionPart(left, right string) int {
	leftNumber, leftErr := strconv.Atoi(left)
	rightNumber, rightErr := strconv.Atoi(right)
	if leftErr == nil && rightErr == nil {
		switch {
		case leftNumber < rightNumber:
			return -1
		case leftNumber > rightNumber:
			return 1
		}
		return 0
	}
	return strings.Compare(left, right)
}

func thirdPartyDriverUpdatePending() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "pgrep", "-f", "inotifywait -q /boot/changes.txt").Output()
	return err == nil && len(strings.TrimSpace(string(output))) > 0
}

func isHexSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if !strings.ContainsRune("0123456789abcdefABCDEF", char) {
			return false
		}
	}
	return true
}

func validateReleaseURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return errors.New("invalid Unraid OS release URL")
	}
	base, err := url.Parse(osReleasesBaseURL)
	if err != nil {
		return errors.New("invalid Unraid release service")
	}
	if !strings.EqualFold(parsed.Scheme, base.Scheme) || !strings.EqualFold(parsed.Host, base.Host) {
		return errors.New("Unraid OS updates must come from the release service")
	}
	return nil
}
