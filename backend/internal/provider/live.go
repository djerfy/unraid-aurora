package provider

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"aurora/internal/model"
	trivymanager "aurora/internal/trivy"
)

// Live reads the real Unraid host: state INI files written by emhttpd, procfs,
// sysfs, the Docker socket and libvirt. Every field is best-effort: a missing
// source yields zero values instead of failing the whole snapshot.
type Live struct {
	stateDir     string
	bootDir      string
	dockerSocket string
	dataDir      string
	configRoot   string
	startedAt    time.Time

	mu        sync.Mutex
	lastCPU   cpuSample
	lastNet   netSample
	lastDisk  diskSample
	history   []model.MetricPoint
	settings  model.Settings
	collected model.Metrics
	client    *http.Client
	webClient *http.Client

	imageMu        sync.Mutex
	imageUpdates   map[string]bool
	imageCheckedAt time.Time

	pluginMu        sync.Mutex
	pluginUpdates   map[string]pluginUpdate
	pluginCheckedAt time.Time

	appsMu         sync.Mutex
	appsEntries    []model.AppCatalogEntry
	appsCategories []string
	appsFetchedAt  time.Time
	appsFeedAt     time.Time

	osMu sync.Mutex

	trivyManager *trivymanager.Manager

	apiMu        sync.Mutex
	apiVersion   string
	apiVersionAt time.Time
	apiConfig    string
	apiConfigAt  time.Time

	mountBase      string
	balanceMu      sync.Mutex
	balance        *balanceRuntime
	balanceHistory []model.BalanceJob

	statsMu        sync.Mutex
	containerStats map[string]cachedContainerStats
	vmCPU          map[string]vmCPUSample
	vmCache        []model.VM
	vmCacheAt      time.Time
	gpuMu          sync.Mutex
	gpuCache       []model.GPU
	gpuCacheAt     time.Time
	upsMu          sync.Mutex
	upsCache       model.UPS
	upsCacheAt     time.Time

	moverMu              sync.Mutex
	moverActive          bool
	moverStarted         time.Time
	moverStopped         bool
	moverLast            model.MoverStatus
	moverHistory         []model.MoverRun
	moverFiles           []string
	moverSeen            map[string]bool
	moverOutput          string
	moverOutputOffset    int64
	moverCandidates      map[string]int64
	moverScannedLists    map[string]int64
	moverCandidateTotal  uint64
	moverCandidateRemain uint64
	moverSyslogOffset    int64
	moverStop            chan struct{}
	moverCloseOnce       sync.Once

	raplLast map[string]raplReading
	raplAt   time.Time

	geoMu    sync.Mutex
	geoCache map[string]geoEntry
}

type cpuSample struct {
	idle  uint64
	total uint64
}

type netSample struct {
	rx uint64
	tx uint64
}

type diskSample struct {
	read  uint64
	write uint64
}

func NewLive(stateDir, bootDir, dockerSocket, dataDir, configDir string) (*Live, error) {
	if _, err := os.Stat(filepath.Join(stateDir, "var.ini")); err != nil {
		return nil, fmt.Errorf("not an Unraid host: %s/var.ini is missing: %w", stateDir, err)
	}
	l := &Live{
		stateDir:     stateDir,
		bootDir:      bootDir,
		dockerSocket: dockerSocket,
		dataDir:      dataDir,
		configRoot:   configDir,
		startedAt:    time.Now(),
		settings: model.Settings{
			Theme: "dark", Accent: "ember", Density: "comfortable",
			RefreshInterval: 5, Language: "en", TemperatureUnit: "celsius",
			TrivyDBUpdateHour: 3,
		},
		containerStats: map[string]cachedContainerStats{},
		imageUpdates:   map[string]bool{},
		pluginUpdates:  map[string]pluginUpdate{},
		vmCPU:          map[string]vmCPUSample{},
		moverStop:      make(chan struct{}),
		moverSeen:      map[string]bool{},
	}
	trivyDir := strings.TrimSpace(os.Getenv("AURORA_TRIVY_DIR"))
	if trivyDir == "" && configDir != "" {
		trivyDir = filepath.Join(configDir, "trivy")
	}
	if trivyDir != "" {
		l.trivyManager = trivymanager.New(trivyDir)
		l.trivyManager.SetDockerSocket(dockerSocket)
	}
	l.migrateDurableState()
	l.loadSettings()
	l.loadMoverHistory()
	l.loadImageUpdates()
	l.loadPluginUpdates()
	l.loadBalanceHistory()
	go l.watchMover()
	l.client = &http.Client{
		Timeout: 4 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", dockerSocket)
			},
		},
	}
	l.webClient = &http.Client{Timeout: 20 * time.Second}
	l.lastCPU = readCPU()
	l.lastNet = readNet()
	l.lastDisk = readDiskStats()
	if m, err := l.Metrics(context.Background()); err == nil {
		l.collected = m
	}
	return l, nil
}

func (l *Live) Name() string { return "live" }

func (l *Live) Snapshot(ctx context.Context) (model.Snapshot, error) {
	return l.snapshot(ctx, false)
}

// SnapshotLight skips the plugin and notification scans. The UI loads those two
// collections from their dedicated endpoints so the first paint is cheaper.
func (l *Live) SnapshotLight(ctx context.Context) (model.Snapshot, error) {
	return l.snapshot(ctx, true)
}

func (l *Live) snapshot(ctx context.Context, light bool) (model.Snapshot, error) {
	// Reuse the hub's last sample when it is fresh: the SSE sampler already
	// owns the delta counters and the handler replaces Metrics with the same
	// value. Fall back to a direct sample on a cold start.
	metrics, ok := l.cachedMetrics()
	if !ok || time.Since(time.Unix(metrics.T, 0)) > 6*time.Second {
		metrics, _ = l.Metrics(ctx)
	}
	var (
		sys           model.System
		arr           model.Array
		disks         []model.Disk
		shares        []model.Share
		containers    []model.Container
		vms           []model.VM
		network       model.Network
		users         []model.User
		plugins       []model.Plugin
		notifications []model.Notification
		gpus          []model.GPU
		history       []model.MetricPoint
		ups           model.UPS
		settings      model.Settings
	)
	var wg sync.WaitGroup
	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}
	run(func() { sys = l.system(ctx, metrics) })
	run(func() { arr, _ = l.Array(ctx) })
	run(func() { disks, _ = l.Disks(ctx) })
	run(func() { shares, _ = l.Shares(ctx) })
	run(func() { containers, _ = l.Containers(ctx) })
	run(func() { vms, _ = l.VMs(ctx) })
	run(func() { network, _ = l.Network(ctx) })
	run(func() { users, _ = l.Users(ctx) })
	if !light {
		run(func() { plugins, _ = l.Plugins(ctx) })
		run(func() { notifications, _ = l.Notifications(ctx) })
	}
	run(func() { gpus, _ = l.GPUs(ctx) })
	run(func() { history, _ = l.History(ctx) })
	run(func() { ups, _ = l.UPS(ctx) })
	run(func() { settings, _ = l.Settings(ctx) })
	wg.Wait()

	snap := model.Snapshot{
		System: sys, Array: arr, Disks: disks, Shares: shares, Pools: poolsFromDisks(disks),
		Containers: containers, VMs: vms, Network: network, Users: users,
		Plugins: plugins, Notifications: notifications, GPUs: gpus, UPS: ups,
		Settings: settings, Metrics: metrics, History: history,
		ServerTime: time.Now(),
	}
	normalizeSnapshot(&snap)
	return snap, nil
}

// normalizeSnapshot turns nil slices into empty ones so the JSON contract never
// contains null where the UI expects an array.
func normalizeSnapshot(s *model.Snapshot) {
	s.Disks = emptyIfNil(s.Disks)
	s.Shares = emptyIfNil(s.Shares)
	s.Pools = emptyIfNil(s.Pools)
	s.Containers = emptyIfNil(s.Containers)
	s.VMs = emptyIfNil(s.VMs)
	s.Users = emptyIfNil(s.Users)
	s.Plugins = emptyIfNil(s.Plugins)
	s.Notifications = emptyIfNil(s.Notifications)
	s.GPUs = emptyIfNil(s.GPUs)
	s.History = emptyIfNil(s.History)
	s.Metrics.Cores = emptyIfNil(s.Metrics.Cores)
	s.System.CPU.PerCore = emptyIfNil(s.System.CPU.PerCore)
	s.System.CPU.Sensors = emptyIfNil(s.System.CPU.Sensors)
	s.System.Temperatures = emptyIfNil(s.System.Temperatures)
	s.System.Fans = emptyIfNil(s.System.Fans)
	s.System.Health.Issues = emptyIfNil(s.System.Health.Issues)
	if len(s.System.Load) < 3 {
		load := make([]float64, 3)
		copy(load, s.System.Load)
		s.System.Load = load
	}
	s.Array.Parity.Disks = emptyIfNil(s.Array.Parity.Disks)
	s.Array.Parity.History = emptyIfNil(s.Array.Parity.History)
	s.Network.DNS = emptyIfNil(s.Network.DNS)
	s.Network.Interfaces = emptyIfNil(s.Network.Interfaces)
	s.Network.WireGuard.Peers = emptyIfNil(s.Network.WireGuard.Peers)
	s.Network.Services = emptyIfNil(s.Network.Services)
	for i := range s.Containers {
		s.Containers[i].Ports = emptyIfNil(s.Containers[i].Ports)
	}
	for i := range s.Users {
		s.Users[i].Shares = emptyIfNil(s.Users[i].Shares)
	}
	for i := range s.Disks {
		s.Disks[i].Warnings = emptyIfNil(s.Disks[i].Warnings)
	}
}

func emptyIfNil[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}

func (l *Live) System(ctx context.Context) (model.System, error) {
	metrics, _ := l.Metrics(ctx)
	return l.system(ctx, metrics), nil
}

func (l *Live) system(ctx context.Context, metrics model.Metrics) model.System {
	varIni, _ := parseINI(filepath.Join(l.stateDir, "var.ini"))
	cpuInfo := readCPUInfo()
	mem := readMemory()

	version := ""
	if ini, err := parseINI("/etc/unraid-version"); err == nil {
		version = ini.get("", "version")
	}
	board := readTrim("/sys/class/dmi/id/board_name")
	product := readTrim("/sys/class/dmi/id/product_name")
	name := varIni.get("", "NAME")
	if name == "" {
		name, _ = os.Hostname()
	}
	uptime := readUptime()

	temp := readCPUTemp()
	health := model.Health{Status: "healthy", Score: 100}
	if metrics.CPU > 90 {
		health = model.Health{Status: "warning", Score: 78, Issues: []string{"CPU usage is sustained above 90%"}}
	}
	if mem.Usage > 92 {
		health = model.Health{Status: "warning", Score: 74, Issues: append(health.Issues, "Memory pressure is high")}
	}

	sys := model.System{
		Hostname: name,
		Name:     name,
		Version:  version,
		Kernel:   readKernel(),
		Board:    strings.TrimSpace(board + " " + product),
		CPU: model.CPUInfo{
			Brand:   cpuInfo.brand,
			Cores:   cpuInfo.cores,
			Threads: cpuInfo.threads,
			BaseGHz: cpuInfo.baseGHz,
			MaxGHz:  cpuInfo.maxGHz,
			FreqGHz: cpuInfo.freqGHz,
			Usage:   metrics.CPU,
			Temp:    temp,
			Power:   metrics.Power,
			PerCore: metrics.Cores,
			Socket:  cpuInfo.socket,
			Cache:   cpuInfo.cache,
		},
		Memory:    mem,
		Uptime:    uptime,
		BootTime:  time.Now().Add(-time.Duration(uptime) * time.Second),
		Load:      readLoad(),
		Health:    health,
		License:   licenseName(varIni.get("", "regTy")),
		FlashGUID: varIni.get("", "flashGUID"),
	}
	if temp > 0 {
		sys.Temperatures = []model.Temperature{{Label: "CPU package", Value: temp, Max: 95}}
	}
	sys.Temperatures = append(sys.Temperatures, readBoardTemps()...)
	return sys
}

func (l *Live) Array(ctx context.Context) (model.Array, error) {
	varIni, err := parseINI(filepath.Join(l.stateDir, "var.ini"))
	if err != nil {
		return model.Array{}, err
	}
	disksIni, _ := parseINI(filepath.Join(l.stateDir, "disks.ini"))
	state := varIni.get("", "mdState")
	if state == "" {
		state = "STOPPED"
	}
	started := strings.EqualFold(state, "STARTED")

	var total, used uint64
	disks := 0
	for name, d := range disksIni {
		if strings.HasPrefix(name, "disk") && d["type"] == "Data" {
			disks++
			total += parseKiB(d["fsSize"])
			used += parseKiB(d["fsUsed"])
		}
	}
	resync := uint64(parseInt(varIni.get("", "mdResyncPos")))
	resyncSize := uint64(parseInt(varIni.get("", "mdResyncSize")))
	progress := 0.0
	if resyncSize > 0 {
		progress = float64(resync) / float64(resyncSize) * 100
		if progress > 100 {
			progress = 100
		}
	}
	running := varIni.get("", "mdResync") != "0" && varIni.get("", "mdResync") != ""
	action := varIni.get("", "mdResyncAction")
	parity := model.Parity{
		Configured: strings.Contains(strings.ToLower(varIni.get("", "mdColor")), "parity") || disksIni.section("parity") != nil,
		Disks:      parityNames(disksIni),
		Running:    running,
		Progress:   round1(progress),
		Position:   resync * 1024,
		Speed:      float64(parseInt(varIni.get("", "mdResyncDt"))) * 1024,
		Errors:     parseInt(varIni.get("", "sbSyncErrs")),
		Elapsed:    int64(parseInt(varIni.get("", "mdResyncElapsed"))),
	}
	if last := lastParityCheck(varIni); last != nil {
		parity.LastCheck = last
	}
	parity.History, _ = l.ParityHistory(ctx)
	rebuild := model.ArrayRebuild{
		Running: running, Action: action, Progress: round1(progress),
		Position: resync * 1024, Size: resyncSize * 1024,
		Speed:   float64(parseInt(varIni.get("", "mdResyncDt"))) * 1024,
		Errors:  parseInt(varIni.get("", "sbSyncErrs")),
		Elapsed: int64(parseInt(varIni.get("", "mdResyncElapsed"))),
	}

	status := "protected"
	if !started {
		status = "stopped"
	} else if strings.Contains(varIni.get("", "mdColor"), "yellow") {
		status = "unprotected"
	}
	usage := 0.0
	if total > 0 {
		usage = float64(used) / float64(total) * 100
	}
	protection := "None"
	if parity.Configured {
		protection = "Parity"
	}
	return model.Array{
		State:      state,
		Protection: protection,
		Status:     status,
		Total:      total,
		Used:       used,
		Free:       total - used,
		Usage:      round1(usage),
		Disks:      disks,
		Parity:     parity,
		Rebuild:    rebuild,
		Sync:       running && action != "",
	}, nil
}

func (l *Live) Disks(ctx context.Context) ([]model.Disk, error) {
	disksIni, err := parseINI(filepath.Join(l.stateDir, "disks.ini"))
	if err != nil {
		return nil, err
	}
	diskIO := readDiskIO(filepath.Join(l.stateDir, "diskload.ini"))
	keys := make([]string, 0, len(disksIni))
	for k := range disksIni {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]model.Disk, 0, len(keys))
	for i, name := range keys {
		d := disksIni[name]
		if name == "" || strings.HasPrefix(name, "luks") {
			continue
		}
		typ := d["type"]
		if typ == "" {
			typ = "Data"
		}
		size := parseKiB(d["fsSize"])
		if size == 0 {
			size = parseKiB(d["size"])
		}
		used := parseKiB(d["fsUsed"])
		dev := d["device"]
		io := diskIO[dev]
		temp := parseInt(d["temp"])
		status := "active"
		if strings.Contains(d["color"], "grey") || d["spundown"] == "1" {
			status = "standby"
		}
		if strings.Contains(d["status"], "MISSING") {
			status = "missing"
		}
		smart := "passed"
		if parseInt(d["warning"]) > 0 || parseInt(d["critical"]) > 0 {
			smart = "warning"
		}
		brand, modelName, serial := diskIdentity(d)
		out = append(out, model.Disk{
			ID: name, Name: name, Device: dev, Type: typ, Pool: d["pool"], FsType: d["fsType"],
			Size: size, Used: used, Free: size - used, Temp: temp, Status: status, Smart: smart,
			Errors: parseInt(d["numErrors"]), ReadBps: io[0], WriteBps: io[1],
			Rotational: d["rotational"] != "0", Serial: serial, Brand: brand, Model: modelName, Media: diskMedia(d),
			Firmware: d["firmware"], SpinupGroup: d["spinupGroup"], Luks: strings.HasPrefix(d["fsType"], "luks:"),
			Color: diskColor(typ, smart), Position: i,
		})
	}
	// Flash is not part of disks.ini; expose it as a first-class device.
	if flash := readFlash(l.bootDir); flash != nil {
		out = append(out, *flash)
	}
	return out, nil
}

func (l *Live) Shares(ctx context.Context) ([]model.Share, error) {
	ini, err := parseINI(filepath.Join(l.stateDir, "shares.ini"))
	if err != nil {
		return nil, err
	}
	out := make([]model.Share, 0, len(ini))
	for name, s := range ini {
		size := parseKiB(s["size"])
		free := parseKiB(s["free"])
		used := uint64(0)
		if size > free {
			used = size - free
		}
		usage := 0.0
		if size > 0 {
			usage = float64(used) / float64(size) * 100
		}
		out = append(out, model.Share{
			Name: name, Comment: s["comment"], Size: size, Free: free, Used: used, Usage: round1(usage),
			Allocator: s["allocator"], SplitLevel: s["splitLevel"], CacheMode: s["useCache"],
			Pool: s["cachePool"], Security: s["security"], SMB: s["export"] == "e" || s["export"] == "yes",
			NFS: s["exportNFS"] == "yes" || s["exportNFS"] == "e", Exclusive: s["exclusive"] == "yes",
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (l *Live) Pools(ctx context.Context) ([]model.Pool, error) {
	disks, _ := l.Disks(ctx)
	return poolsFromDisks(disks), nil
}

func poolsFromDisks(disks []model.Disk) []model.Pool {
	byPool := map[string]*model.Pool{}
	for _, d := range disks {
		if d.Type != "Cache" || d.Pool == "" {
			continue
		}
		p, ok := byPool[d.Pool]
		if !ok {
			p = &model.Pool{Name: d.Pool, FsType: d.FsType, Cache: true, Status: "healthy"}
			byPool[d.Pool] = p
		}
		p.Devices = append(p.Devices, d.Name)
		p.Size += d.Size
		p.Used += d.Used
	}
	out := make([]model.Pool, 0, len(byPool))
	for _, p := range byPool {
		p.Free = p.Size - p.Used
		if p.Size > 0 {
			p.Usage = round1(float64(p.Used) / float64(p.Size) * 100)
		}
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (l *Live) Containers(ctx context.Context) ([]model.Container, error) {
	containers, err := l.listContainers(ctx)
	if err != nil {
		return nil, err
	}
	l.enrichContainerStats(ctx, containers)
	l.enrichContainerSecurity(ctx, containers)
	return containers, nil
}

func (l *Live) listContainers(ctx context.Context) ([]model.Container, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/containers/json?all=1", nil)
	if err != nil {
		return nil, err
	}
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker socket: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker socket returned %s", resp.Status)
	}
	var raw []struct {
		ID              string            `json:"Id"`
		Names           []string          `json:"Names"`
		Image           string            `json:"Image"`
		ImageID         string            `json:"ImageID"`
		State           string            `json:"State"`
		Status          string            `json:"Status"`
		Created         int64             `json:"Created"`
		Labels          map[string]string `json:"Labels"`
		Mounts          []dockerMount     `json:"Mounts"`
		NetworkSettings struct {
			Networks map[string]containerNetwork `json:"Networks"`
		} `json:"NetworkSettings"`
		Ports []struct {
			IP          string `json:"IP"`
			PrivatePort int    `json:"PrivatePort"`
			PublicPort  int    `json:"PublicPort"`
			Type        string `json:"Type"`
		} `json:"Ports"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode docker containers: %w", err)
	}
	out := make([]model.Container, 0, len(raw))
	autostart := l.containerAutostart()
	for _, c := range raw {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		ports := make([]model.Port, 0, len(c.Ports))
		for _, p := range c.Ports {
			// PublicPort is 0 for ports that are only exposed inside the
			// container. They are not reachable from the host, so surfacing
			// them as "0 → 8080/tcp" is just noise.
			if p.PublicPort == 0 {
				continue
			}
			ports = append(ports, model.Port{Host: p.PublicPort, Container: p.PrivatePort, Protocol: p.Type})
		}
		repo, tag := c.Image, "latest"
		if i := strings.LastIndex(c.Image, ":"); i > 0 && !strings.Contains(c.Image[i:], "/") {
			repo, tag = c.Image[:i], c.Image[i+1:]
		}
		color := colorFor(name)
		network, ip := pickContainerNetwork(c.NetworkSettings.Networks)
		out = append(out, model.Container{
			ID: c.ID, Name: name, Image: c.Image, ImageID: c.ImageID, Repository: repo, Tag: tag,
			Network: network, IP: ip, Mounts: containerMounts(c.Mounts),
			State: c.State, Status: c.Status, Health: dockerHealth(c.Status), Autostart: autostart[name],
			Uptime: dockerStatusUptime(c.Status),
			Ports:  ports, Color: color, Created: time.Unix(c.Created, 0),
			Icon: c.Labels["net.unraid.docker.icon"], WebUI: c.Labels["net.unraid.docker.webui"],
			RestartPolicy: "", UpdateAvailable: l.imageUpdate(c.Image),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (l *Live) ContainerLogs(ctx context.Context, id string, lines int) ([]model.LogLine, error) {
	if lines <= 0 || lines > 1000 {
		lines = 120
	}
	path := fmt.Sprintf("http://docker/containers/%s/logs?stdout=1&stderr=1&timestamps=1&tail=%d", url.PathEscape(id), lines)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker logs: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker logs returned %s", resp.Status)
	}
	// Docker multiplexes stdout/stderr with an 8-byte header per frame.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var out []model.LogLine
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if len(line) > 8 && (line[0] == 1 || line[0] == 2) && line[1] == 0 {
			line = line[8:]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		ts := time.Now()
		if stamp, rest, ok := strings.Cut(line, " "); ok {
			if parsed, err := time.Parse(time.RFC3339Nano, stamp); err == nil {
				ts, line = parsed, rest
			}
		}
		level := "info"
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "fatal") {
			level = "error"
		} else if strings.Contains(lower, "warn") {
			level = "warn"
		}
		out = append(out, model.LogLine{Time: ts, Level: level, Source: id, Message: line})
	}
	return out, nil
}

func (l *Live) VMs(ctx context.Context) ([]model.VM, error) {
	l.statsMu.Lock()
	if len(l.vmCache) > 0 && time.Since(l.vmCacheAt) < 5*time.Second {
		cached := append([]model.VM(nil), l.vmCache...)
		l.statsMu.Unlock()
		return cached, nil
	}
	l.statsMu.Unlock()

	cmd := exec.CommandContext(ctx, "virsh", "list", "--all", "--name")
	outBytes, err := cmd.Output()
	if err != nil {
		return nil, nil // libvirt is optional
	}
	var vms []model.VM
	for _, name := range strings.Split(strings.TrimSpace(string(outBytes)), "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		vms = append(vms, l.vmDetails(ctx, name))
	}
	l.statsMu.Lock()
	l.vmCache = append([]model.VM(nil), vms...)
	l.vmCacheAt = time.Now()
	l.statsMu.Unlock()
	return vms, nil
}

func (l *Live) Network(ctx context.Context) (model.Network, error) {
	varIni, _ := parseINI(filepath.Join(l.stateDir, "var.ini"))
	networks := model.Network{
		Hostname: varIni.get("", "NAME"),
		Domain:   varIni.get("", "LOCAL_TLD"),
		Gateway:  varIni.get("", "GATEWAY"),
		DNS:      []string{varIni.get("", "DNS1"), varIni.get("", "DNS2")},
	}
	ifaces, _ := net.Interfaces()
	stats := readNetStats()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if !strings.HasPrefix(iface.Name, "eth") && !strings.HasPrefix(iface.Name, "br") && !strings.HasPrefix(iface.Name, "bond") && !strings.HasPrefix(iface.Name, "wg") {
			continue
		}
		ni := model.NetInterface{
			Name: iface.Name, MAC: iface.HardwareAddr.String(), State: "down",
			SpeedMbps: readLinkSpeed(iface.Name),
		}
		if iface.Flags&net.FlagUp != 0 {
			ni.State = "up"
		}
		if addrs, err := iface.Addrs(); err == nil {
			for _, a := range addrs {
				ipnet, ok := a.(*net.IPNet)
				if !ok {
					continue
				}
				if ipnet.IP.To4() != nil && ni.IP == "" {
					ni.IP = ipnet.IP.String()
				}
			}
		}
		if s, ok := stats[iface.Name]; ok {
			ni.RxTotal, ni.TxTotal = s.rx, s.tx
		}
		networks.Interfaces = append(networks.Interfaces, ni)
	}
	return networks, nil
}

func (l *Live) Users(ctx context.Context) ([]model.User, error) {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return nil, err
	}
	var out []model.User
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) < 7 {
			continue
		}
		uid, _ := strconv.Atoi(parts[2])
		if uid != 0 && (uid < 1000 || uid >= 65534) {
			continue
		}
		role := "read-only"
		if uid == 0 {
			role = "administrator"
		}
		out = append(out, model.User{
			Name: parts[0], Description: parts[4], UID: uid, Role: role, Root: uid == 0,
			Shares: []string{}, Color: colorFor(parts[0]),
		})
	}
	return out, nil
}

func (l *Live) Plugins(_ context.Context) ([]model.Plugin, error) {
	return l.pluginList(), nil
}

func (l *Live) Notifications(ctx context.Context) ([]model.Notification, error) {
	var out []model.Notification
	dirs := []struct {
		path    string
		read    bool
		archive bool
	}{
		{filepath.Join("/tmp/notifications/unread"), false, false},
		{filepath.Join("/tmp/notifications/archive"), true, true},
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir.path)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".notify") {
				continue
			}
			ini, err := parseINI(filepath.Join(dir.path, e.Name()))
			if err != nil {
				continue
			}
			ts := int64(parseInt(ini.get("", "timestamp")))
			out = append(out, model.Notification{
				ID: e.Name(), Title: ini.get("", "event"), Subject: ini.get("", "subject"),
				Description: ini.get("", "description"), Importance: ini.get("", "importance"),
				Timestamp: time.Unix(ts, 0), Read: dir.read, Archived: dir.archive,
				Source: ini.get("", "event"),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.After(out[j].Timestamp) })
	return out, nil
}

func (l *Live) ParityHistory(ctx context.Context) ([]model.ParityCheck, error) {
	path := filepath.Join(l.bootDir, "config", "parity-checks.log")
	f, err := os.Open(path)
	if err != nil {
		return nil, nil
	}
	defer f.Close()
	var out []model.ParityCheck
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "|")
		if len(parts) < 6 {
			continue
		}
		duration, _ := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		speed, _ := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
		errors, _ := strconv.Atoi(strings.TrimSpace(parts[4]))
		date, _ := time.Parse("02-01-2006 15:04:05", strings.TrimSpace(parts[0]))
		out = append(out, model.ParityCheck{
			Date: date, Duration: duration, Speed: speed, Status: parts[3], Errors: errors,
			Action: parts[5],
		})
	}
	if len(out) > 20 {
		out = out[len(out)-20:]
	}
	return out, nil
}

func (l *Live) Settings(ctx context.Context) (model.Settings, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.settings, nil
}

func (l *Live) SaveSettings(ctx context.Context, s model.Settings) error {
	if s.TrivyDBUpdateHour < 0 || s.TrivyDBUpdateHour > 23 {
		s.TrivyDBUpdateHour = 3
	}
	l.mu.Lock()
	l.settings = s
	l.mu.Unlock()
	if err := os.MkdirAll(l.configDir(), 0o750); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := l.saveConfigFile("settings.json", raw); err != nil {
		return err
	}
	l.applyTrivySettings(s)
	return nil
}

func (l *Live) loadSettings() {
	raw, err := l.readConfigFile("settings.json")
	if err != nil {
		return
	}
	var s model.Settings
	if json.Unmarshal(raw, &s) != nil {
		return
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) == nil {
		if _, ok := fields["trivyDbUpdateHour"]; !ok {
			s.TrivyDBUpdateHour = 3
		}
	}
	if s.Theme != "" {
		l.settings = s
	}
}

func (l *Live) Metrics(ctx context.Context) (model.Metrics, error) {
	now := time.Now()
	curCPU := readCPU()
	curNet := readNet()
	curDisk := readDiskStats()

	l.mu.Lock()
	defer l.mu.Unlock()

	cpuPct := pctDelta(l.lastCPU.idle, l.lastCPU.total, curCPU.idle, curCPU.total)
	netRx, netTx := netDelta(l.lastNet, curNet, now)
	diskRead, diskWrite := diskDelta(l.lastDisk, curDisk, now)
	l.lastCPU, l.lastNet, l.lastDisk = curCPU, curNet, curDisk

	mem := readMemory()
	cores := readPerCore()
	metrics := model.Metrics{
		T: now.Unix(), CPU: cpuPct, Memory: mem.Usage,
		NetRx: netRx, NetTx: netTx, DiskRead: diskRead, DiskWrite: diskWrite,
		Temp: readCPUTemp(), Load: readLoad()[0], Power: l.cpuPower(now), Cores: cores,
	}
	l.history = append(l.history, model.MetricPoint{
		T: metrics.T, CPU: metrics.CPU, Memory: metrics.Memory, NetRx: netRx,
		NetTx: netTx, DiskRead: diskRead, DiskWrite: diskWrite, Temp: metrics.Temp,
	})
	if len(l.history) > historyLength {
		l.history = l.history[len(l.history)-historyLength:]
	}
	l.collected = metrics
	return metrics, nil
}

// cachedMetrics returns the last sample produced by the SSE hub or History.
func (l *Live) cachedMetrics() (model.Metrics, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.collected.T == 0 {
		return model.Metrics{}, false
	}
	return l.collected, true
}

func (l *Live) History(ctx context.Context) ([]model.MetricPoint, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]model.MetricPoint(nil), l.history...), nil
}

func (l *Live) Logs(ctx context.Context, source string, lines int) ([]model.LogLine, error) {
	if lines <= 0 || lines > 1000 {
		lines = 200
	}
	switch source {
	case "nginx":
		return nginxLogLines(lines)
	case "libvirt":
		return readLogSource("libvirt", lines, "/var/log/libvirt/libvirtd.log", "/var/log/libvirt/libvirt.log")
	case "maillog":
		return readLogSource("maillog", lines, "/var/log/maillog", "/var/log/mail.log", "/var/log/mail/maillog")
	case "docker":
		return readLogSource("docker", lines, "/var/log/docker.log")
	case "parity":
		return readLogSource("parity", lines, "/var/log/parity.log")
	default:
		return readLogSource("syslog", lines, "/var/log/syslog")
	}
}

func readLogSource(source string, lines int, candidates ...string) ([]model.LogLine, error) {
	var lastErr error
	for _, path := range candidates {
		if _, err := os.Stat(path); err != nil {
			lastErr = err
			continue
		}
		return logLinesFromFile(source, path, lines)
	}
	if lastErr == nil {
		lastErr = os.ErrNotExist
	}
	return nil, fmt.Errorf("%s log is unavailable: %w", source, lastErr)
}

func logLinesFromFile(source, path string, lines int) ([]model.LogLine, error) {
	out, err := tailLines(path, lines)
	if err != nil {
		return nil, err
	}
	return buildLogLines(source, out), nil
}

func buildLogLines(source string, lines []string) []model.LogLine {
	result := make([]model.LogLine, 0, len(lines))
	for _, line := range lines {
		level := "info"
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "error") || strings.Contains(lower, "fail"):
			level = "error"
		case strings.Contains(lower, "warn"):
			level = "warn"
		case strings.Contains(lower, "debug"):
			level = "debug"
		}
		result = append(result, model.LogLine{Time: time.Now(), Level: level, Source: source, Message: line})
	}
	return result
}

// nginxLogLines merges the access and error logs kept in /var/log/nginx,
// prefixing each line with its file name so the source stays obvious.
func nginxLogLines(lines int) ([]model.LogLine, error) {
	matches, err := filepath.Glob("/var/log/nginx/*.log")
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no nginx logs found in /var/log/nginx: %w", os.ErrNotExist)
	}
	sort.Strings(matches)
	perFile := lines / len(matches)
	if perFile < 20 {
		perFile = 20
	}
	result := make([]model.LogLine, 0, lines)
	for _, path := range matches {
		tail, err := tailLines(path, perFile)
		if err != nil {
			continue
		}
		name := filepath.Base(path)
		for _, line := range tail {
			result = append(result, buildLogLines("nginx", []string{name + ": " + line})...)
		}
	}
	if len(result) == 0 {
		return []model.LogLine{}, nil
	}
	if len(result) > lines {
		result = result[len(result)-lines:]
	}
	return result, nil
}

func (l *Live) Perform(ctx context.Context, action model.Action) error {
	if action.Target != "" && strings.ContainsAny(action.Target, " \t\n;&|$`") {
		return errors.New("invalid target")
	}
	run := func(bin string, args ...string) error {
		cmd := exec.CommandContext(ctx, bin, args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w (%s)", bin, err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	switch action.Kind {
	case ActionArrayStart:
		return run("/usr/local/sbin/emcmd", "cmdStart=Start")
	case ActionArrayStop:
		return run("/usr/local/sbin/emcmd", "cmdStop=Stop")
	case ActionDiskSpinUp:
		return run("/usr/local/sbin/mdcmd", "spinup", action.Target)
	case ActionDiskSpinDown:
		return run("/usr/local/sbin/mdcmd", "spindown", action.Target)
	case ActionContainerStart:
		return run("/usr/bin/docker", "start", action.Target)
	case ActionContainerStop:
		return run("/usr/bin/docker", "stop", action.Target)
	case ActionContainerRestart:
		return run("/usr/bin/docker", "restart", action.Target)
	case ActionContainerUpdate:
		return l.UpdateContainerImage(ctx, action.Target, nil)
	case ActionVMStart:
		return run("/usr/bin/virsh", "start", action.Target)
	case ActionVMStop:
		return run("/usr/bin/virsh", "shutdown", action.Target)
	case ActionVMPause:
		return run("/usr/bin/virsh", "suspend", action.Target)
	case ActionVMResume:
		return run("/usr/bin/virsh", "resume", action.Target)
	case ActionParityStart:
		return run("/usr/local/sbin/emcmd", "cmdCheck=Check")
	case ActionParityStop:
		return run("/usr/local/sbin/emcmd", "cmdCheck=Cancel")
	case ActionParityPause:
		return run("/usr/local/sbin/emcmd", "cmdCheck=Pause")
	case ActionParityResume:
		return run("/usr/local/sbin/emcmd", "cmdCheck=Resume")
	case ActionNotifyAck, ActionNotifyArchive:
		file := filepath.Base(action.Target)
		src := filepath.Join("/tmp/notifications/unread", file)
		dst := filepath.Join("/tmp/notifications/archive", file)
		if action.Kind == ActionNotifyArchive {
			return os.Rename(src, dst)
		}
		// Acknowledge simply marks the notification as read by archiving it.
		return os.Rename(src, dst)
	case ActionNotifyAckAll:
		entries, _ := os.ReadDir("/tmp/notifications/unread")
		for _, e := range entries {
			_ = os.Rename(filepath.Join("/tmp/notifications/unread", e.Name()), filepath.Join("/tmp/notifications/archive", e.Name()))
		}
		return nil
	default:
		return fmt.Errorf("unsupported action %q", action.Kind)
	}
}

func (l *Live) Close() error {
	l.moverCloseOnce.Do(func() {
		if l.moverStop != nil {
			close(l.moverStop)
		}
	})
	l.moverMu.Lock()
	_ = l.saveMoverHistory()
	l.moverMu.Unlock()
	l.balanceMu.Lock()
	if l.balance != nil {
		l.balance.cancel()
	}
	l.balanceMu.Unlock()
	if l.trivyManager != nil {
		l.trivyManager.Close()
	}
	return nil
}

// --- procfs helpers -------------------------------------------------------

func readCPU() cpuSample {
	raw, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuSample{}
	}
	line, _, _ := strings.Cut(string(raw), "\n")
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return cpuSample{}
	}
	var total, idle uint64
	for i, f := range fields[1:] {
		v, _ := strconv.ParseUint(f, 10, 64)
		total += v
		if i == 3 || i == 4 {
			idle += v
		}
	}
	return cpuSample{idle: idle, total: total}
}

func readPerCore() []float64 {
	raw, err := os.ReadFile("/proc/stat")
	if err != nil {
		return nil
	}
	var out []float64
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, "cpu") || strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		var total, idle uint64
		for i, f := range fields[1:] {
			v, _ := strconv.ParseUint(f, 10, 64)
			total += v
			if i == 3 || i == 4 {
				idle += v
			}
		}
		if total == 0 {
			out = append(out, 0)
			continue
		}
		out = append(out, round1(float64(total-idle)/float64(total)*100))
	}
	return out
}

func readMemory() model.MemoryInfo {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return model.MemoryInfo{}
	}
	vals := map[string]uint64{}
	for _, line := range strings.Split(string(raw), "\n") {
		key, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		v, _ := strconv.ParseUint(fields[0], 10, 64)
		vals[key] = v * 1024
	}
	total := vals["MemTotal"]
	available := vals["MemAvailable"]
	used := total - available
	usage := 0.0
	if total > 0 {
		usage = float64(used) / float64(total) * 100
	}
	return model.MemoryInfo{
		Total: total, Used: used, Free: vals["MemFree"], Available: available,
		Cached: vals["Cached"], Buffers: vals["Buffers"], SwapTotal: vals["SwapTotal"],
		SwapUsed: vals["SwapTotal"] - vals["SwapFree"], Usage: round1(usage),
	}
}

func readUptime() int64 {
	raw, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(fields[0], 64)
	return int64(v)
}

func readLoad() []float64 {
	raw, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return []float64{0, 0, 0}
	}
	fields := strings.Fields(string(raw))
	out := []float64{0, 0, 0}
	for i := 0; i < 3 && i < len(fields); i++ {
		out[i], _ = strconv.ParseFloat(fields[i], 64)
	}
	return out
}

func readKernel() string {
	var u uname
	_ = u.read()
	if u.release != "" {
		return "Linux " + u.release
	}
	return ""
}

type uname struct{ release string }

func (u *uname) read() error {
	raw, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return err
	}
	u.release = strings.TrimSpace(string(raw))
	return nil
}

type cpuDetails struct {
	brand, socket, cache     string
	cores, threads           int
	baseGHz, maxGHz, freqGHz float64
}

func readCPUInfo() cpuDetails {
	raw, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return cpuDetails{}
	}
	var d cpuDetails
	seen := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		switch key {
		case "model name":
			if d.brand == "" {
				d.brand = val
			}
		case "physical id":
			seen[val] = true
		case "cpu cores":
			d.cores, _ = strconv.Atoi(val)
		case "siblings":
			d.threads, _ = strconv.Atoi(val)
		case "cpu MHz":
			if f, _ := strconv.ParseFloat(val, 64); f > 0 {
				d.freqGHz = f / 1000
			}
		case "cache size":
			d.cache = val
		}
	}
	if d.threads == 0 {
		d.threads = len(seen)
	}
	if d.cores == 0 {
		d.cores = d.threads
	}
	return d
}

func readCPUTemp() float64 {
	entries, err := filepath.Glob("/sys/class/hwmon/hwmon*/temp1_input")
	if err != nil || len(entries) == 0 {
		return 0
	}
	best := 0.0
	for _, path := range entries {
		if v := readFloat(path); v > best {
			best = v
		}
	}
	return round1(best / 1000)
}

func readBoardTemps() []model.Temperature {
	return nil
}

func readNet() netSample {
	stats := readNetStats()
	var rx, tx uint64
	for _, s := range stats {
		rx += s.rx
		tx += s.tx
	}
	return netSample{rx: rx, tx: tx}
}

type netStat struct{ rx, tx uint64 }

func readNetStats() map[string]netStat {
	out := map[string]netStat{}
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return out
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		name, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) < 9 {
			continue
		}
		rx, _ := strconv.ParseUint(fields[0], 10, 64)
		tx, _ := strconv.ParseUint(fields[8], 10, 64)
		out[strings.TrimSpace(name)] = netStat{rx: rx, tx: tx}
	}
	return out
}

func readDiskStats() diskSample {
	raw, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		return diskSample{}
	}
	var read, write uint64
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		r, _ := strconv.ParseUint(fields[5], 10, 64)
		w, _ := strconv.ParseUint(fields[9], 10, 64)
		read += r * 512
		write += w * 512
	}
	return diskSample{read: read, write: write}
}

func readDiskIO(path string) map[string][2]float64 {
	out := map[string][2]float64{}
	ini, err := parseINI(path)
	if err != nil {
		return out
	}
	for dev, s := range ini {
		v := strings.Fields(s[""])
		if len(v) >= 2 {
			r, _ := strconv.ParseFloat(v[0], 64)
			w, _ := strconv.ParseFloat(v[1], 64)
			out[dev] = [2]float64{r, w}
		}
	}
	return out
}

func pctDelta(oldIdle, oldTotal, newIdle, newTotal uint64) float64 {
	if newTotal <= oldTotal {
		return 0
	}
	total := newTotal - oldTotal
	idle := newIdle - oldIdle
	if idle > total {
		return 0
	}
	return round1(float64(total-idle) / float64(total) * 100)
}

func netDelta(old netSample, cur netSample, now time.Time) (float64, float64) {
	// The caller keeps the last timestamp implicitly; use a fixed 2s window as
	// the hub samples every two seconds.
	window := 2.0
	if cur.rx < old.rx || cur.tx < old.tx {
		return 0, 0
	}
	return float64(cur.rx-old.rx) / window, float64(cur.tx-old.tx) / window
}

func diskDelta(old diskSample, cur diskSample, now time.Time) (float64, float64) {
	if cur.read < old.read || cur.write < old.write {
		return 0, 0
	}
	return float64(cur.read-old.read) / 2, float64(cur.write-old.write) / 2
}

func readLinkSpeed(name string) int {
	raw, err := os.ReadFile(filepath.Join("/sys/class/net", name, "speed"))
	if err != nil {
		return 0
	}
	v, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	return v
}

func readFloat(path string) float64 {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	v, _ := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
	return v
}

func readTrim(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func readFlash(bootDir string) *model.Disk {
	// /boot is a vfat partition; report it as the flash device.
	disk := model.Disk{Name: "flash", ID: "flash", Type: "Flash", FsType: "vfat", Status: "active", Smart: "passed", Color: diskColor("Flash", "")}
	var st syscall.Statfs_t
	if err := syscall.Statfs(bootDir, &st); err == nil {
		disk.Size = st.Blocks * uint64(st.Bsize)
		disk.Free = st.Bavail * uint64(st.Bsize)
		disk.Used = disk.Size - disk.Free
	}
	return &disk
}

func firstFloat(value string) float64 {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return 0
	}
	f, _ := strconv.ParseFloat(fields[0], 64)
	return f
}

func parseInt(v string) int {
	i, _ := strconv.Atoi(strings.TrimSpace(v))
	return i
}

func parseKiB(v string) uint64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if f <= 0 {
		return 0
	}
	return uint64(f * 1024)
}

func licenseName(regTy string) string {
	switch strings.ToLower(regTy) {
	case "pro":
		return "Unraid Pro"
	case "plus":
		return "Unraid Plus"
	case "basic":
		return "Unraid Basic"
	case "trial":
		return "Unraid Trial"
	default:
		return ""
	}
}

func parityNames(ini iniFile) []string {
	var out []string
	for _, name := range []string{"parity", "parity2", "parity3"} {
		if _, ok := ini[name]; ok {
			out = append(out, name)
		}
	}
	return out
}

func lastParityCheck(varIni iniFile) *model.ParityCheck {
	start := int64(parseInt(varIni.get("", "sbSynced")))
	end := int64(parseInt(varIni.get("", "sbSynced2")))
	if start == 0 {
		return nil
	}
	dur := end - start
	if dur < 0 {
		dur = 0
	}
	speed := 0.0
	size := float64(parseInt(varIni.get("", "sbSyncedSize"))) * 1024
	if dur > 0 && size > 0 {
		speed = size / float64(dur)
	}
	return &model.ParityCheck{
		Date: time.Unix(start, 0), Duration: dur, Speed: speed,
		Status: "completed", Errors: parseInt(varIni.get("", "sbSyncErrs")),
		Action: varIni.get("", "mdResyncAction"), Size: uint64(size),
	}
}

func tailLines(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return nil, err
	}
	const chunk = 32 * 1024
	var data []byte
	pos, _ := f.Seek(0, io.SeekCurrent)
	for len(data) < chunk*8 && pos > 0 {
		readSize := int64(chunk)
		if pos < readSize {
			readSize = pos
		}
		pos -= readSize
		buf := make([]byte, readSize)
		if _, err := f.ReadAt(buf, pos); err != nil && err != io.EOF {
			return nil, err
		}
		data = append(buf, data...)
		if bytes.Count(data, []byte("\n")) > n {
			break
		}
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}

func colorFor(seed string) string {
	palette := []string{"#FF6B4A", "#5AA9FF", "#9B7BFF", "#3DDC97", "#FFB020", "#FF4D67", "#18BCF2", "#76B900", "#E5A00D", "#7C5CFC"}
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	return palette[int(h.Sum32())%len(palette)]
}

func slug(s string) string {
	h := sha1.Sum([]byte(s))
	return fmt.Sprintf("%x", h[:4])
}
