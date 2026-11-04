package trivy

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"aurora/internal/model"
)

const (
	fallbackVersion = "0.76.1"
	maxDownload     = 250 << 20
	maxScanOutput   = 64 << 20
)

var (
	releasesAPIBase      = "https://api.github.com/repos/aquasecurity/trivy/releases"
	releasesDownloadBase = "https://github.com/aquasecurity/trivy/releases/download"
)

var versionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$`)

var errDatabaseNotReady = errors.New("the Trivy vulnerability database is not ready")

type diskState struct {
	Version     string    `json:"version"`
	DBUpdatedAt time.Time `json:"dbUpdatedAt"`
	DBSize      int64     `json:"dbSize"`
}

type scanJob struct {
	key     string
	image   string
	imageID string
}

// Manager owns the persistent Trivy binary, its database and scan results.
// All mutable fields are protected by mu; network and process operations run
// without holding the lock.
type Manager struct {
	dir           string
	binary        string
	runtimeBinary string
	runtimeReady  bool
	cacheDir      string
	scansDir      string
	statePath     string
	client        *http.Client
	dockerSocket  string

	mu                 sync.Mutex
	dbMu               sync.RWMutex
	state              diskState
	installing         bool
	updatingDB         bool
	stage              string
	lastError          string
	lastDBAttempt      time.Time
	lastInstallAttempt time.Time
	versions           []model.TrivyVersion
	versionsAt         time.Time
	scanning           map[string]bool

	queue      chan scanJob
	workerOnce sync.Once
	closed     chan struct{}
	closeOnce  sync.Once
}

func New(dir string) *Manager {
	m := &Manager{
		dir:           dir,
		binary:        filepath.Join(dir, "trivy"),
		runtimeBinary: defaultRuntimeBinary(),
		cacheDir:      filepath.Join(dir, "cache"),
		scansDir:      filepath.Join(dir, "scans"),
		statePath:     filepath.Join(dir, "state.json"),
		client:        &http.Client{Timeout: 20 * time.Minute},
		scanning:      map[string]bool{},
		queue:         make(chan scanJob, 64),
		closed:        make(chan struct{}),
	}
	m.cleanupTemporary()
	m.loadState()
	if m.binaryExists() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if version, err := m.prepareRuntime(ctx); err != nil {
			m.lastError = "Trivy is stored but cannot be executed: " + err.Error()
		} else if m.state.Version == "" && version != "" {
			m.state.Version = version
			_ = m.persistState()
		}
	}
	return m
}

func (m *Manager) cleanupTemporary() {
	if m.dir == "" {
		return
	}
	entries, err := filepath.Glob(filepath.Join(m.dir, "install-*"))
	if err != nil {
		return
	}
	for _, entry := range entries {
		_ = os.RemoveAll(entry)
	}
}

func (m *Manager) SetDockerSocket(path string) {
	m.mu.Lock()
	m.dockerSocket = strings.TrimSpace(path)
	m.mu.Unlock()
}

func (m *Manager) Status() model.TrivyStatus {
	m.mu.Lock()
	state := m.state
	installing := m.installing
	updatingDB := m.updatingDB
	stage := m.stage
	lastError := m.lastError
	runtimeReady := m.runtimeReady
	runtimeBinary := m.runtimeBinary
	m.mu.Unlock()
	if runtimeReady {
		if info, err := os.Stat(runtimeBinary); err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
			runtimeReady = false
		}
	}

	dbReady := m.dbReady()
	size := state.DBSize
	if info, err := os.Stat(m.dbPath()); err == nil && !info.IsDir() && info.Size() > 0 {
		size = info.Size()
	}
	javaDBReady := m.javaDBReady()
	javaDBSize := int64(0)
	if info, err := os.Stat(m.javaDBPath()); err == nil && !info.IsDir() && info.Size() > 0 {
		javaDBSize = info.Size()
	}
	var updated *time.Time
	if !state.DBUpdatedAt.IsZero() {
		value := state.DBUpdatedAt
		updated = &value
	}
	return model.TrivyStatus{
		Installed: m.binaryExists(), RuntimeReady: runtimeReady, Version: state.Version,
		DBReady: dbReady, JavaDBReady: javaDBReady, DBUpdatedAt: updated, DBSize: size, JavaDBSize: javaDBSize,
		Installing: installing, UpdatingDB: updatingDB, Stage: stage, Error: lastError,
	}
}

func (m *Manager) Versions(ctx context.Context) ([]model.TrivyVersion, error) {
	m.mu.Lock()
	cached := append([]model.TrivyVersion(nil), m.versions...)
	cachedAt := m.versionsAt
	m.mu.Unlock()
	if len(cached) > 0 && time.Since(cachedAt) < 30*time.Minute {
		return cached, nil
	}

	requestCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, releasesAPIBase+"?per_page=30", nil)
	if err != nil {
		return cached, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "aurora")
	resp, err := m.client.Do(req)
	if err != nil {
		if len(cached) > 0 {
			return cached, nil
		}
		return nil, fmt.Errorf("list Trivy releases: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if len(cached) > 0 {
			return cached, nil
		}
		return nil, fmt.Errorf("Trivy release service returned HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	var releases []struct {
		TagName     string    `json:"tag_name"`
		Name        string    `json:"name"`
		Draft       bool      `json:"draft"`
		Prerelease  bool      `json:"prerelease"`
		PublishedAt time.Time `json:"published_at"`
	}
	if err := json.Unmarshal(raw, &releases); err != nil {
		return nil, fmt.Errorf("decode Trivy releases: %w", err)
	}
	versions := make([]model.TrivyVersion, 0, len(releases))
	seen := map[string]bool{}
	for _, release := range releases {
		version := normalizeVersion(release.TagName)
		if release.Draft || release.Prerelease || !versionPattern.MatchString(version) || seen[version] {
			continue
		}
		seen[version] = true
		versions = append(versions, model.TrivyVersion{
			Version: version, Name: release.Name, PublishedAt: release.PublishedAt,
		})
	}
	if len(versions) == 0 {
		if len(cached) > 0 {
			return cached, nil
		}
		return nil, errors.New("no stable Trivy release was found")
	}
	m.mu.Lock()
	m.versions = append([]model.TrivyVersion(nil), versions...)
	m.versionsAt = time.Now()
	m.mu.Unlock()
	return versions, nil
}

// StartInstall downloads a release and its database in the background. An
// empty version resolves the latest stable release.
func (m *Manager) StartInstall(version string) error {
	version = normalizeVersion(version)
	if version != "" && !versionPattern.MatchString(version) {
		m.mu.Lock()
		m.lastError = "invalid Trivy version"
		m.mu.Unlock()
		return errors.New("invalid Trivy version")
	}
	m.mu.Lock()
	if m.installing || m.updatingDB {
		m.mu.Unlock()
		return errors.New("a Trivy operation is already running")
	}
	m.installing = true
	m.stage = "Starting installation"
	m.lastError = ""
	m.lastDBAttempt = time.Now()
	m.lastInstallAttempt = time.Now()
	m.mu.Unlock()

	go func() {
		err := m.install(context.Background(), version)
		m.mu.Lock()
		m.installing = false
		m.stage = ""
		if err != nil {
			m.lastError = err.Error()
		}
		m.mu.Unlock()
		if err != nil {
			slog.Error("install Trivy", "error", err)
		}
	}()
	return nil
}

func (m *Manager) StartDBUpdate() error {
	m.mu.Lock()
	if m.installing || m.updatingDB {
		m.mu.Unlock()
		return errors.New("a Trivy operation is already running")
	}
	if !m.binaryExists() {
		m.mu.Unlock()
		return errors.New("Trivy is not installed yet")
	}
	if !m.runtimeReady {
		m.mu.Unlock()
		return errors.New("Trivy is installed but cannot be executed; reinstall it or set AURORA_TRIVY_RUNTIME_DIR")
	}
	m.updatingDB = true
	m.stage = "Starting database update"
	m.lastError = ""
	m.lastDBAttempt = time.Now()
	m.mu.Unlock()

	go func() {
		err := m.updateDBNow(context.Background())
		m.mu.Lock()
		m.updatingDB = false
		m.stage = ""
		if err != nil {
			m.lastError = err.Error()
		}
		m.mu.Unlock()
		if err != nil {
			slog.Error("update Trivy database", "error", err)
		}
	}()
	return nil
}

func (m *Manager) EnqueueScan(image, imageID string, force bool) bool {
	image = strings.TrimSpace(image)
	imageID = strings.TrimSpace(imageID)
	if image == "" && imageID == "" {
		return false
	}
	m.ensureWorker()
	key := scanKey(image, imageID)
	select {
	case <-m.closed:
		return false
	default:
	}
	if !m.scanReady() {
		return false
	}
	m.mu.Lock()
	if m.scanning[key] {
		m.mu.Unlock()
		return false
	}
	m.mu.Unlock()
	if !force {
		if summary, ok := m.readSummary(key); ok {
			if summary.Error == "" && !m.stale(summary) {
				return false
			}
			if summary.Error != "" {
				if info, err := os.Stat(m.summaryPath(key)); err == nil && time.Since(info.ModTime()) < time.Hour {
					return false
				}
			}
		}
	}
	m.mu.Lock()
	if m.scanning[key] {
		m.mu.Unlock()
		return false
	}
	m.scanning[key] = true
	m.mu.Unlock()
	select {
	case m.queue <- scanJob{key: key, image: image, imageID: imageID}:
		return true
	case <-m.closed:
		m.mu.Lock()
		delete(m.scanning, key)
		m.mu.Unlock()
		return false
	default:
		m.mu.Lock()
		delete(m.scanning, key)
		m.mu.Unlock()
		return false
	}
}

func (m *Manager) Security(image, imageID string, enabled bool) model.ContainerSecurity {
	if !enabled {
		return model.ContainerSecurity{Enabled: false}
	}
	key := scanKey(image, imageID)
	m.mu.Lock()
	scanning := m.scanning[key]
	version := m.state.Version
	m.mu.Unlock()
	summary, ok := m.readSummary(key)
	if !ok {
		return model.ContainerSecurity{Enabled: true, Scanning: scanning, Image: image, ImageID: imageID, TrivyVersion: version}
	}
	summary.Enabled = true
	summary.Scanning = scanning
	summary.Stale = summary.Error == "" && m.stale(summary)
	if summary.Image == "" {
		summary.Image = image
	}
	if summary.ImageID == "" {
		summary.ImageID = imageID
	}
	return summary
}

func (m *Manager) Detail(image, imageID string, enabled bool) model.ContainerSecurityDetail {
	summary := m.Security(image, imageID, enabled)
	detail := model.ContainerSecurityDetail{ContainerSecurity: summary}
	if !summary.Enabled || !summary.Available {
		return detail
	}
	raw, err := os.ReadFile(m.detailPath(scanKey(image, imageID)))
	if err != nil {
		return detail
	}
	var payload struct {
		Vulnerabilities []model.ContainerVulnerability `json:"vulnerabilities"`
	}
	if json.Unmarshal(raw, &payload) == nil {
		detail.Vulnerabilities = payload.Vulnerabilities
	}
	if detail.Vulnerabilities == nil {
		detail.Vulnerabilities = []model.ContainerVulnerability{}
	}
	return detail
}

func (m *Manager) RunScheduler(ctx context.Context, settings func() model.Settings) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	m.schedulerTick(settings())
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.schedulerTick(settings())
		}
	}
}

func (m *Manager) Close() {
	m.closeOnce.Do(func() { close(m.closed) })
}

func (m *Manager) schedulerTick(settings model.Settings) {
	if !settings.TrivyEnabled {
		return
	}
	status := m.Status()
	if status.Installing || status.UpdatingDB {
		return
	}
	if !status.Installed || !status.RuntimeReady {
		m.mu.Lock()
		recentInstall := time.Since(m.lastInstallAttempt) < 10*time.Minute
		m.mu.Unlock()
		if !recentInstall {
			_ = m.StartInstall(settings.TrivyVersion)
		}
		return
	}
	desired := normalizeVersion(settings.TrivyVersion)
	if desired != "" && status.Version != desired {
		m.mu.Lock()
		recentInstall := time.Since(m.lastInstallAttempt) < 10*time.Minute
		m.mu.Unlock()
		if !recentInstall {
			_ = m.StartInstall(desired)
		}
		return
	}
	if !status.DBReady || !status.JavaDBReady {
		m.mu.Lock()
		recent := time.Since(m.lastDBAttempt) < 10*time.Minute
		m.mu.Unlock()
		if !recent {
			_ = m.StartDBUpdate()
		}
		return
	}
	hour := settings.TrivyDBUpdateHour
	if hour < 0 || hour > 23 {
		hour = 3
	}
	now := time.Now()
	scheduled := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if now.Before(scheduled) {
		return
	}
	if status.DBUpdatedAt != nil && !status.DBUpdatedAt.Before(scheduled) {
		return
	}
	m.mu.Lock()
	recent := time.Since(m.lastDBAttempt) < 30*time.Minute
	m.mu.Unlock()
	if recent {
		return
	}
	_ = m.StartDBUpdate()
}

func (m *Manager) install(ctx context.Context, requested string) error {
	version, err := m.resolveVersion(ctx, requested)
	if err != nil {
		return err
	}
	m.mu.Lock()
	current := m.state.Version
	m.mu.Unlock()
	versionChanged := current != version
	if versionChanged || !m.binaryExists() {
		m.setStage("Downloading Trivy " + version)
		if err := m.downloadAndInstall(ctx, version); err != nil {
			return err
		}
	} else if !m.runtimeReady {
		m.setStage("Preparing the Trivy runtime binary")
		if _, err := m.prepareRuntime(ctx); err != nil {
			return fmt.Errorf("Trivy binary is not executable: %w", err)
		}
	}
	if versionChanged || !m.dbReady() || !m.javaDBReady() {
		return m.updateDBNow(ctx)
	}
	return nil
}

func (m *Manager) resolveVersion(ctx context.Context, requested string) (string, error) {
	if requested != "" {
		version := normalizeVersion(requested)
		if !versionPattern.MatchString(version) {
			return "", errors.New("invalid Trivy version")
		}
		return version, nil
	}
	versions, err := m.Versions(ctx)
	if err == nil && len(versions) > 0 {
		return versions[0].Version, nil
	}
	if err != nil {
		slog.Warn("resolve latest Trivy release, using fallback", "error", err, "version", fallbackVersion)
	}
	return fallbackVersion, nil
}

func (m *Manager) downloadAndInstall(ctx context.Context, version string) error {
	asset, err := assetName(version)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return err
	}
	expected, err := m.checksumFor(ctx, version, asset)
	if err != nil {
		return err
	}
	temporary, err := os.MkdirTemp(m.dir, "install-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	archive := filepath.Join(temporary, asset)
	actual, err := m.download(ctx, fmt.Sprintf("%s/v%s/%s", releasesDownloadBase, version, asset), archive)
	if err != nil {
		return err
	}
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("Trivy checksum mismatch: got %s, want %s", actual, expected)
	}
	binary := filepath.Join(temporary, "trivy")
	if err := extractBinary(archive, binary); err != nil {
		return err
	}
	if err := os.Chmod(binary, 0o755); err != nil {
		return err
	}
	if err := os.Rename(binary, m.binary); err != nil {
		return fmt.Errorf("install Trivy binary: %w", err)
	}
	if _, err := m.prepareRuntime(ctx); err != nil {
		return fmt.Errorf("prepare the Trivy runtime binary: %w", err)
	}
	m.mu.Lock()
	m.state.Version = version
	m.mu.Unlock()
	return m.persistState()
}

func (m *Manager) checksumFor(ctx context.Context, version, asset string) (string, error) {
	endpoint := fmt.Sprintf("%s/v%s/trivy_%s_checksums.txt", releasesDownloadBase, version, version)
	raw, err := m.get(ctx, endpoint, 1<<20)
	if err != nil {
		return "", fmt.Errorf("download Trivy checksums: %w", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == asset {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("checksum for %s was not found", asset)
}

func (m *Manager) download(ctx context.Context, endpoint, destination string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "aurora")
	resp, err := m.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download Trivy: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download Trivy: HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxDownload {
		return "", errors.New("Trivy archive is larger than expected")
	}
	file, err := os.Create(destination)
	if err != nil {
		return "", err
	}
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(file, hasher), io.LimitReader(resp.Body, maxDownload+1))
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(destination)
		return "", err
	}
	if written > maxDownload {
		_ = os.Remove(destination)
		return "", errors.New("Trivy archive is larger than expected")
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (m *Manager) get(ctx context.Context, endpoint string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "aurora")
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, errors.New("response is too large")
	}
	return raw, nil
}

func (m *Manager) updateDBNow(ctx context.Context) error {
	if !m.binaryExists() {
		return errors.New("Trivy is not installed")
	}
	executable := m.executablePath()
	if executable == "" {
		return errors.New("Trivy is installed but cannot be executed; reinstall it or set AURORA_TRIVY_RUNTIME_DIR")
	}
	m.dbMu.Lock()
	defer m.dbMu.Unlock()
	runCtx, cancel := context.WithTimeout(ctx, 45*time.Minute)
	defer cancel()
	m.mu.Lock()
	version := m.state.Version
	m.lastDBAttempt = time.Now()
	m.mu.Unlock()

	m.setStage("Downloading vulnerability database")
	command := exec.CommandContext(runCtx, executable, "image", "--download-db-only", "--cache-dir", m.cacheDir, "--quiet", "--timeout", "20m")
	command.Env = append(os.Environ(), "TRIVY_NO_PROGRESS=true")
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("download Trivy database: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	info, err := os.Stat(m.dbPath())
	if err != nil || info.IsDir() || info.Size() == 0 || !m.dbReady() {
		return errors.New("Trivy did not create the vulnerability database and metadata")
	}

	m.setStage("Downloading Java vulnerability database")
	javaCommand := exec.CommandContext(runCtx, executable, "image", "--download-java-db-only", "--cache-dir", m.cacheDir, "--quiet", "--timeout", "20m")
	javaCommand.Env = append(os.Environ(), "TRIVY_NO_PROGRESS=true")
	javaOutput, err := javaCommand.CombinedOutput()
	if err != nil {
		return fmt.Errorf("download Trivy Java database: %w (%s)", err, strings.TrimSpace(string(javaOutput)))
	}
	javaInfo, err := os.Stat(m.javaDBPath())
	if err != nil || javaInfo.IsDir() || javaInfo.Size() == 0 || !m.javaDBReady() {
		return errors.New("Trivy did not create the Java vulnerability database and metadata")
	}

	m.mu.Lock()
	m.state.DBUpdatedAt = time.Now()
	m.state.DBSize = info.Size()
	if m.state.Version == "" {
		m.state.Version = version
	}
	m.mu.Unlock()
	return m.persistState()
}

func (m *Manager) scan(ctx context.Context, image, imageID string) (model.ContainerSecurityDetail, error) {
	m.mu.Lock()
	version := m.state.Version
	m.mu.Unlock()
	if version == "" || !m.dbReady() || !m.javaDBReady() {
		return model.ContainerSecurityDetail{}, errors.New("Trivy or its vulnerability databases are not ready")
	}
	executable := m.executablePath()
	if executable == "" {
		return model.ContainerSecurityDetail{}, errors.New("Trivy is installed but cannot be executed; reinstall it or set AURORA_TRIVY_RUNTIME_DIR")
	}
	// Keep scans and database replacement apart so a refresh cannot swap the
	// SQLite files while Trivy is reading them.
	m.dbMu.RLock()
	defer m.dbMu.RUnlock()
	target := strings.TrimSpace(imageID)
	if target == "" {
		target = strings.TrimSpace(image)
	}
	if target == "" {
		return model.ContainerSecurityDetail{}, errors.New("image reference is empty")
	}
	runCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	command := exec.CommandContext(runCtx, executable, "image",
		"--format", "json",
		"--skip-db-update",
		"--skip-java-db-update",
		"--scanners", "vuln",
		"--image-src", "docker",
		"--cache-dir", m.cacheDir,
		"--quiet",
		"--timeout", "20m",
		target,
	)
	m.mu.Lock()
	dockerSocket := m.dockerSocket
	m.mu.Unlock()
	env := append(os.Environ(), "TRIVY_NO_PROGRESS=true", "TRIVY_CACHE_DIR="+m.cacheDir)
	if dockerSocket != "" {
		env = append(env, "DOCKER_HOST=unix://"+dockerSocket)
	}
	command.Env = env
	var stdout limitedBuffer
	stdout.limit = maxScanOutput
	var stderr limitedBuffer
	stderr.limit = 1 << 20
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return model.ContainerSecurityDetail{}, fmt.Errorf("Trivy scan failed: %s", message)
	}
	if stdout.overflow {
		return model.ContainerSecurityDetail{}, errors.New("Trivy scan output is too large")
	}
	return m.parseReport(stdout.Bytes(), image, imageID, version)
}

func (m *Manager) parseReport(raw []byte, image, imageID, version string) (model.ContainerSecurityDetail, error) {
	var report trivyReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return model.ContainerSecurityDetail{}, fmt.Errorf("decode Trivy report: %w", err)
	}
	detail := model.ContainerSecurityDetail{
		ContainerSecurity: model.ContainerSecurity{
			Enabled: true, Available: true, Image: image, ImageID: imageID, TrivyVersion: version,
		},
		Vulnerabilities: []model.ContainerVulnerability{},
	}
	now := time.Now()
	detail.ScannedAt = &now
	m.mu.Lock()
	if !m.state.DBUpdatedAt.IsZero() {
		dbUpdated := m.state.DBUpdatedAt
		detail.DBUpdatedAt = &dbUpdated
	}
	m.mu.Unlock()

	for _, result := range report.Results {
		for _, finding := range result.Vulnerabilities {
			severity := strings.ToUpper(strings.TrimSpace(finding.Severity))
			switch severity {
			case "CRITICAL":
				detail.Critical++
			case "HIGH":
				detail.High++
			case "MEDIUM":
				detail.Medium++
			case "LOW":
				detail.Low++
			default:
				detail.Unknown++
			}
			if len(detail.Vulnerabilities) < maxVulnerabilities {
				detail.Vulnerabilities = append(detail.Vulnerabilities, model.ContainerVulnerability{
					ID: finding.VulnerabilityID, Package: finding.PkgName,
					InstalledVersion: finding.InstalledVersion, FixedVersion: finding.FixedVersion,
					Severity: severity, Title: finding.Title, Target: result.Target,
					Type: result.Type, URL: finding.PrimaryURL,
				})
			}
		}
	}
	detail.Total = detail.Critical + detail.High + detail.Medium + detail.Low + detail.Unknown
	sort.SliceStable(detail.Vulnerabilities, func(i, j int) bool {
		left, right := severityRank(detail.Vulnerabilities[i].Severity), severityRank(detail.Vulnerabilities[j].Severity)
		if left != right {
			return left < right
		}
		return detail.Vulnerabilities[i].ID < detail.Vulnerabilities[j].ID
	})
	return detail, nil
}

func (m *Manager) worker() {
	for {
		select {
		case <-m.closed:
			return
		case job := <-m.queue:
			if !m.scanReady() {
				m.mu.Lock()
				delete(m.scanning, job.key)
				m.mu.Unlock()
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
			detail, err := m.scan(ctx, job.image, job.imageID)
			if errors.Is(err, errDatabaseNotReady) {
				m.mu.Lock()
				delete(m.scanning, job.key)
				m.mu.Unlock()
				cancel()
				continue
			}
			if err != nil {
				now := time.Now()
				detail = model.ContainerSecurityDetail{
					ContainerSecurity: model.ContainerSecurity{
						Enabled: true, Available: true, Error: err.Error(), Image: job.image,
						ImageID: job.imageID, ScannedAt: &now,
					},
				}
				slog.Warn("scan container image", "image", job.image, "error", err)
			}
			m.saveScan(job.key, job.image, job.imageID, detail)
			m.mu.Lock()
			delete(m.scanning, job.key)
			m.mu.Unlock()
			cancel()
		}
	}
}

func (m *Manager) saveScan(key, image, imageID string, detail model.ContainerSecurityDetail) {
	if err := os.MkdirAll(m.scansDir, 0o755); err != nil {
		return
	}
	summary := detail.ContainerSecurity
	summary.Enabled = true
	summary.Available = true
	if summary.ScannedAt == nil {
		now := time.Now()
		summary.ScannedAt = &now
	}
	if summary.Image == "" {
		summary.Image = image
	}
	if summary.ImageID == "" {
		summary.ImageID = imageID
	}
	detail.ContainerSecurity = summary
	if detail.Vulnerabilities == nil {
		detail.Vulnerabilities = []model.ContainerVulnerability{}
	}
	summaryRaw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return
	}
	detailRaw, err := json.MarshalIndent(struct {
		Vulnerabilities []model.ContainerVulnerability `json:"vulnerabilities"`
	}{Vulnerabilities: detail.Vulnerabilities}, "", "  ")
	if err != nil {
		return
	}
	// Write the details first: the summary is what marks a scan as available.
	if err := writeAtomic(m.detailPath(key), detailRaw, 0o644); err != nil {
		slog.Warn("write Trivy detail", "error", err)
		return
	}
	if err := writeAtomic(m.summaryPath(key), summaryRaw, 0o644); err != nil {
		slog.Warn("write Trivy summary", "error", err)
		return
	}
	m.pruneScans()
}

func (m *Manager) readSummary(key string) (model.ContainerSecurity, bool) {
	raw, err := os.ReadFile(m.summaryPath(key))
	if err != nil {
		return model.ContainerSecurity{}, false
	}
	var summary model.ContainerSecurity
	if err := json.Unmarshal(raw, &summary); err != nil {
		return model.ContainerSecurity{}, false
	}
	return summary, true
}

func (m *Manager) stale(summary model.ContainerSecurity) bool {
	m.mu.Lock()
	version := m.state.Version
	dbUpdated := m.state.DBUpdatedAt
	m.mu.Unlock()
	if version != "" && summary.TrivyVersion != "" && summary.TrivyVersion != version {
		return true
	}
	if !dbUpdated.IsZero() {
		if summary.DBUpdatedAt == nil {
			return true
		}
		return summary.DBUpdatedAt.Before(dbUpdated)
	}
	return false
}

func (m *Manager) scanReady() bool {
	m.mu.Lock()
	ready := m.runtimeReady
	busy := m.installing || m.updatingDB
	m.mu.Unlock()
	return ready && !busy && m.dbReady() && m.javaDBReady()
}

func (m *Manager) binaryExists() bool {
	info, err := os.Stat(m.binary)
	return err == nil && !info.IsDir()
}

func (m *Manager) dbReady() bool {
	return fileReady(m.dbPath()) && fileReady(m.dbMetadataPath())
}

func (m *Manager) javaDBReady() bool {
	return fileReady(m.javaDBPath()) && fileReady(m.javaMetadataPath())
}

func fileReady(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}

func (m *Manager) dbPath() string {
	return filepath.Join(m.cacheDir, "db", "trivy.db")
}

func (m *Manager) javaDBPath() string {
	return filepath.Join(m.cacheDir, "java-db", "trivy-java.db")
}

func (m *Manager) dbMetadataPath() string {
	return filepath.Join(m.cacheDir, "db", "metadata.json")
}

func (m *Manager) javaMetadataPath() string {
	return filepath.Join(m.cacheDir, "java-db", "metadata.json")
}

func (m *Manager) summaryPath(key string) string {
	return filepath.Join(m.scansDir, key+".summary.json")
}

func (m *Manager) detailPath(key string) string {
	return filepath.Join(m.scansDir, key+".detail.json")
}

func (m *Manager) setStage(stage string) {
	m.mu.Lock()
	m.stage = stage
	m.mu.Unlock()
}

func (m *Manager) ensureWorker() {
	m.workerOnce.Do(func() { go m.worker() })
}

func (m *Manager) validateRuntime(ctx context.Context) (string, error) {
	m.mu.Lock()
	path := m.runtimeBinary
	m.mu.Unlock()
	if path == "" {
		return "", errors.New("Trivy runtime path is empty")
	}
	runCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	output, err := exec.CommandContext(runCtx, path, "--version").Output()
	if err != nil {
		return "", err
	}
	matches := regexp.MustCompile(`(?m)^Version:\s*(\S+)`).FindStringSubmatch(string(output))
	if len(matches) == 2 {
		return matches[1], nil
	}
	return "", nil
}

// executablePath returns the runtime copy, never the persistent flash copy:
// Unraid mounts /boot with noexec, so executing from AURORA_TRIVY_DIR fails.
func (m *Manager) executablePath() string {
	m.mu.Lock()
	ready := m.runtimeReady
	path := m.runtimeBinary
	m.mu.Unlock()
	if !ready || path == "" {
		return ""
	}
	return path
}

// prepareRuntime copies the persistent binary to the first executable runtime
// location. The flash drive is often mounted noexec, so the persistent copy is
// never executed directly.
func (m *Manager) prepareRuntime(ctx context.Context) (string, error) {
	sourceInfo, err := os.Stat(m.binary)
	if err != nil {
		return "", err
	}
	var lastErr error
	for _, candidate := range runtimeCandidates() {
		if err := copyRuntimeBinary(m.binary, candidate, sourceInfo); err != nil {
			lastErr = err
			continue
		}
		m.mu.Lock()
		m.runtimeBinary = candidate
		m.mu.Unlock()
		version, err := m.validateRuntime(ctx)
		if err == nil {
			m.mu.Lock()
			m.runtimeReady = true
			m.mu.Unlock()
			return version, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("no executable runtime directory is available")
	}
	m.mu.Lock()
	m.runtimeReady = false
	m.mu.Unlock()
	return "", lastErr
}

func defaultRuntimeBinary() string {
	candidates := runtimeCandidates()
	if len(candidates) > 0 {
		return candidates[0]
	}
	return "/tmp/aurora-trivy/trivy"
}

func runtimeCandidates() []string {
	candidates := make([]string, 0, 3)
	if dir := strings.TrimSpace(os.Getenv("AURORA_TRIVY_RUNTIME_DIR")); dir != "" {
		candidates = append(candidates, filepath.Join(dir, "trivy"))
	}
	candidates = append(candidates,
		"/usr/local/aurora/trivy/trivy",
		"/tmp/aurora-trivy/trivy",
	)
	seen := map[string]bool{}
	out := candidates[:0]
	for _, candidate := range candidates {
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		out = append(out, candidate)
	}
	return out
}

func copyRuntimeBinary(source, destination string, sourceInfo os.FileInfo) error {
	if info, err := os.Stat(destination); err == nil && !info.IsDir() &&
		info.Size() == sourceInfo.Size() && !info.ModTime().Before(sourceInfo.ModTime()) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".trivy-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err := io.Copy(temporary, in); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o755); err != nil {
		return err
	}
	return os.Rename(name, destination)
}

func (m *Manager) loadState() {
	raw, err := os.ReadFile(m.statePath)
	if err != nil {
		return
	}
	var state diskState
	if err := json.Unmarshal(raw, &state); err != nil {
		slog.Warn("parse Trivy state", "error", err)
		return
	}
	m.state = state
}

func (m *Manager) persistState() error {
	m.mu.Lock()
	raw, err := json.MarshalIndent(m.state, "", "  ")
	m.mu.Unlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return err
	}
	return writeAtomic(m.statePath, raw, 0o600)
}

func (m *Manager) pruneScans() {
	entries, err := filepath.Glob(filepath.Join(m.scansDir, "*.summary.json"))
	if err != nil || len(entries) <= 300 {
		return
	}
	type candidate struct {
		path    string
		modTime time.Time
	}
	items := make([]candidate, 0, len(entries))
	for _, entry := range entries {
		if info, err := os.Stat(entry); err == nil {
			items = append(items, candidate{path: entry, modTime: info.ModTime()})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].modTime.Before(items[j].modTime) })
	for _, item := range items[:len(items)-300] {
		base := strings.TrimSuffix(item.path, ".summary.json")
		_ = os.Remove(base + ".summary.json")
		_ = os.Remove(base + ".detail.json")
	}
}

func scanKey(image, imageID string) string {
	source := strings.TrimSpace(imageID)
	if source == "" {
		source = strings.TrimSpace(image)
	}
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

func normalizeVersion(value string) string {
	value = strings.TrimSpace(value)
	return strings.TrimPrefix(value, "v")
}

func assetName(version string) (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "trivy_" + version + "_Linux-64bit.tar.gz", nil
	case "arm64":
		return "trivy_" + version + "_Linux-ARM64.tar.gz", nil
	default:
		return "", fmt.Errorf("Trivy is not available for architecture %s", runtime.GOARCH)
	}
}

func extractBinary(archive, destination string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != "trivy" {
			continue
		}
		out, err := os.Create(destination)
		if err != nil {
			return err
		}
		written, copyErr := io.Copy(out, io.LimitReader(reader, maxDownload+1))
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if written > maxDownload {
			return errors.New("Trivy binary is larger than expected")
		}
		return nil
	}
	return errors.New("Trivy binary was not found in the archive")
}

func writeAtomic(path string, raw []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".aurora-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err := temporary.Write(raw); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func severityRank(severity string) int {
	switch strings.ToUpper(severity) {
	case "CRITICAL":
		return 0
	case "HIGH":
		return 1
	case "MEDIUM":
		return 2
	case "LOW":
		return 3
	default:
		return 4
	}
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.overflow {
		return len(p), nil
	}
	remaining := b.limit - b.Len()
	if remaining <= 0 {
		b.overflow = true
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.Buffer.Write(p[:remaining])
		b.overflow = true
		return len(p), nil
	}
	return b.Buffer.Write(p)
}

const maxVulnerabilities = 1000

type trivyReport struct {
	Results []struct {
		Target          string `json:"Target"`
		Class           string `json:"Class"`
		Type            string `json:"Type"`
		Vulnerabilities []struct {
			VulnerabilityID  string `json:"VulnerabilityID"`
			PkgName          string `json:"PkgName"`
			InstalledVersion string `json:"InstalledVersion"`
			FixedVersion     string `json:"FixedVersion"`
			Severity         string `json:"Severity"`
			Title            string `json:"Title"`
			PrimaryURL       string `json:"PrimaryURL"`
		} `json:"Vulnerabilities"`
	} `json:"Results"`
}
