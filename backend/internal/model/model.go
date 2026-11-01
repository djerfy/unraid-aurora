// Package model defines the transport types shared by the API, the data
// providers and the embedded web UI. All types are JSON-first and stable.
package model

import "time"

type Health struct {
	Status string   `json:"status"`
	Score  int      `json:"score"`
	Issues []string `json:"issues"`
}

type CPUSensor struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

type CPUInfo struct {
	Brand   string      `json:"brand"`
	Cores   int         `json:"cores"`
	Threads int         `json:"threads"`
	BaseGHz float64     `json:"baseGhz"`
	MaxGHz  float64     `json:"maxGhz"`
	FreqGHz float64     `json:"freqGhz"`
	Usage   float64     `json:"usage"`
	Temp    float64     `json:"temp"`
	Power   float64     `json:"power"`
	PerCore []float64   `json:"perCore"`
	Sensors []CPUSensor `json:"sensors"`
	Socket  string      `json:"socket"`
	Cache   string      `json:"cache"`
}

type MemoryInfo struct {
	Total     uint64  `json:"total"`
	Used      uint64  `json:"used"`
	Free      uint64  `json:"free"`
	Available uint64  `json:"available"`
	Cached    uint64  `json:"cached"`
	Buffers   uint64  `json:"buffers"`
	SwapTotal uint64  `json:"swapTotal"`
	SwapUsed  uint64  `json:"swapUsed"`
	Usage     float64 `json:"usage"`
	Type      string  `json:"type"`
	Speed     string  `json:"speed"`
}

type Temperature struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
	Max   float64 `json:"max"`
}

type Fan struct {
	Label string `json:"label"`
	RPM   int    `json:"rpm"`
}

type System struct {
	Hostname     string        `json:"hostname"`
	Name         string        `json:"name"`
	Version      string        `json:"version"`
	Kernel       string        `json:"kernel"`
	Board        string        `json:"board"`
	CPU          CPUInfo       `json:"cpu"`
	Memory       MemoryInfo    `json:"memory"`
	Uptime       int64         `json:"uptime"`
	BootTime     time.Time     `json:"bootTime"`
	Load         []float64     `json:"load"`
	Temperatures []Temperature `json:"temperatures"`
	Fans         []Fan         `json:"fans"`
	Health       Health        `json:"health"`
	License      string        `json:"license"`
	FlashGUID    string        `json:"flashGuid"`
	CaseModel    string        `json:"caseModel"`
}

type ParityCheck struct {
	Date     time.Time `json:"date"`
	Duration int64     `json:"duration"`
	Speed    float64   `json:"speed"`
	Status   string    `json:"status"`
	Errors   int       `json:"errors"`
	Action   string    `json:"action"`
	Size     uint64    `json:"size"`
}

type Parity struct {
	Configured bool          `json:"configured"`
	Disks      []string      `json:"disks"`
	Running    bool          `json:"running"`
	Paused     bool          `json:"paused"`
	Progress   float64       `json:"progress"`
	Position   uint64        `json:"position"`
	Speed      float64       `json:"speed"`
	Errors     int           `json:"errors"`
	Elapsed    int64         `json:"elapsed"`
	LastCheck  *ParityCheck  `json:"lastCheck"`
	History    []ParityCheck `json:"history"`
}

type ArrayStartOptions struct {
	KeyType string `json:"keyType"` // none, passphrase or keyfile
	Key     string `json:"key"`     // passphrase, or base64 keyfile content
	Mode    string `json:"mode"`    // normal or maintenance
}

type Array struct {
	State      string  `json:"state"`
	Protection string  `json:"protection"`
	Status     string  `json:"status"`
	Total      uint64  `json:"total"`
	Used       uint64  `json:"used"`
	Free       uint64  `json:"free"`
	Usage      float64 `json:"usage"`
	Disks      int     `json:"disks"`
	Parity     Parity  `json:"parity"`
	Sync       bool    `json:"sync"`
	StartedAt  string  `json:"startedAt"`
}

type Disk struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Device      string   `json:"device"`
	Type        string   `json:"type"`
	Pool        string   `json:"pool"`
	FsType      string   `json:"fsType"`
	Size        uint64   `json:"size"`
	Used        uint64   `json:"used"`
	Free        uint64   `json:"free"`
	Temp        int      `json:"temp"`
	Status      string   `json:"status"`
	Smart       string   `json:"smart"`
	Errors      int      `json:"errors"`
	ReadBps     float64  `json:"readBps"`
	WriteBps    float64  `json:"writeBps"`
	Rotational  bool     `json:"rotational"`
	Serial      string   `json:"serial"`
	Model       string   `json:"model"`
	Firmware    string   `json:"firmware"`
	SpinupGroup string   `json:"spinupGroup"`
	Luks        bool     `json:"luks"`
	Warnings    []string `json:"warnings"`
	Position    int      `json:"position"`
	LastSpinUp  string   `json:"lastSpinUp"`
	Color       string   `json:"color"`
}

type Share struct {
	Name         string    `json:"name"`
	Comment      string    `json:"comment"`
	Size         uint64    `json:"size"`
	Free         uint64    `json:"free"`
	Used         uint64    `json:"used"`
	Usage        float64   `json:"usage"`
	Allocator    string    `json:"allocator"`
	SplitLevel   string    `json:"splitLevel"`
	CacheMode    string    `json:"cacheMode"`
	Pool         string    `json:"pool"`
	Security     string    `json:"security"`
	SMB          bool      `json:"smb"`
	NFS          bool      `json:"nfs"`
	Exclusive    bool      `json:"exclusive"`
	Include      []string  `json:"include"`
	Exclude      []string  `json:"exclude"`
	LastModified time.Time `json:"lastModified"`
}

type Pool struct {
	Name    string   `json:"name"`
	FsType  string   `json:"fsType"`
	Devices []string `json:"devices"`
	Size    uint64   `json:"size"`
	Used    uint64   `json:"used"`
	Free    uint64   `json:"free"`
	Usage   float64  `json:"usage"`
	Status  string   `json:"status"`
	Cache   bool     `json:"cache"`
}

type Port struct {
	Host      int    `json:"host"`
	Container int    `json:"container"`
	Protocol  string `json:"protocol"`
	Label     string `json:"label,omitempty"`
}

type Container struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Image           string    `json:"image"`
	Repository      string    `json:"repository"`
	Tag             string    `json:"tag"`
	Version         string    `json:"version"`
	State           string    `json:"state"`
	Status          string    `json:"status"`
	Uptime          int64     `json:"uptime"`
	Autostart       bool      `json:"autostart"`
	Network         string    `json:"network"`
	IP              string    `json:"ip"`
	Ports           []Port    `json:"ports"`
	CPU             float64   `json:"cpu"`
	Memory          uint64    `json:"memory"`
	MemoryLimit     uint64    `json:"memoryLimit"`
	NetworkRx       uint64    `json:"networkRx"`
	NetworkTx       uint64    `json:"networkTx"`
	Color           string    `json:"color"`
	Category        string    `json:"category"`
	UpdateAvailable bool      `json:"updateAvailable"`
	Created         time.Time `json:"created"`
	Icon            string    `json:"icon"`
	WebUI           string    `json:"webUi"`
	RestartPolicy   string    `json:"restartPolicy"`
}

type ContainerEnv struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ContainerTemplate struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Network       string            `json:"network"`
	RestartPolicy string            `json:"restartPolicy"`
	Icon          string            `json:"icon"`
	WebUI         string            `json:"webUi"`
	Privileged    bool              `json:"privileged"`
	Ports         []Port            `json:"ports"`
	Volumes       []ContainerVolume `json:"volumes"`
	Env           []ContainerEnv    `json:"env"`
	Modified      time.Time         `json:"modified"`
}

type ContainerCreateOptions struct {
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	NetworkMode   string            `json:"networkMode"`
	RestartPolicy string            `json:"restartPolicy"`
	Ports         []Port            `json:"ports"`
	Volumes       []ContainerVolume `json:"volumes"`
	Env           []ContainerEnv    `json:"env"`
	Icon          string            `json:"icon"`
	WebUI         string            `json:"webUi"`
	Privileged    bool              `json:"privileged"`
}

type ContainerUpdateEvent struct {
	Step     string  `json:"step"`
	Message  string  `json:"message"`
	Progress float64 `json:"progress"`
	Done     bool    `json:"done"`
	Error    bool    `json:"error"`
}

type ImageCheck struct {
	Image  string `json:"image"`
	Local  string `json:"local"`
	Remote string `json:"remote"`
	Update bool   `json:"update"`
	Error  string `json:"error,omitempty"`
}

type ContainerVolume struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	ReadOnly    bool   `json:"readOnly"`
}

type ContainerConfig struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	RestartPolicy string            `json:"restartPolicy"`
	NetworkMode   string            `json:"networkMode"`
	Ports         []Port            `json:"ports"`
	Volumes       []ContainerVolume `json:"volumes"`
	Env           []ContainerEnv    `json:"env"`
}

type VM struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	State       string    `json:"state"`
	OS          string    `json:"os"`
	VCPUs       int       `json:"vcpus"`
	Memory      uint64    `json:"memory"`
	CPUUsage    float64   `json:"cpuUsage"`
	MemoryUsage float64   `json:"memoryUsage"`
	DiskSize    uint64    `json:"diskSize"`
	DiskUsed    uint64    `json:"diskUsed"`
	Uptime      int64     `json:"uptime"`
	Autostart   bool      `json:"autostart"`
	IP          string    `json:"ip"`
	Machine     string    `json:"machine"`
	Color       string    `json:"color"`
	Icon        string    `json:"icon"`
	PrimaryGPU  string    `json:"primaryGpu"`
	LastStarted time.Time `json:"lastStarted"`

	ConsoleProtocol  string `json:"consoleProtocol"`
	ConsolePort      int    `json:"consolePort"`
	ConsoleWebSocket int    `json:"consoleWebSocket"`
}

type Notification struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Subject     string    `json:"subject"`
	Description string    `json:"description"`
	Importance  string    `json:"importance"`
	Timestamp   time.Time `json:"timestamp"`
	Read        bool      `json:"read"`
	Archived    bool      `json:"archived"`
	Source      string    `json:"source"`
}

type NetInterface struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	MAC         string   `json:"mac"`
	IP          string   `json:"ip"`
	IPv6        string   `json:"ipv6"`
	SpeedMbps   int      `json:"speedMbps"`
	State       string   `json:"state"`
	RxBps       float64  `json:"rxBps"`
	TxBps       float64  `json:"txBps"`
	RxTotal     uint64   `json:"rxTotal"`
	TxTotal     uint64   `json:"txTotal"`
	DHCP        bool     `json:"dhcp"`
	Gateway     string   `json:"gateway"`
	DNS         []string `json:"dns"`
	Bond        string   `json:"bond"`
	Bridge      string   `json:"bridge"`
}

type WireGuardPeer struct {
	Name       string    `json:"name"`
	PublicKey  string    `json:"publicKey"`
	Endpoint   string    `json:"endpoint"`
	AllowedIPs []string  `json:"allowedIps"`
	Latest     time.Time `json:"latestHandshake"`
	Rx         uint64    `json:"rx"`
	Tx         uint64    `json:"tx"`
	Online     bool      `json:"online"`
}

type WireGuard struct {
	Enabled bool            `json:"enabled"`
	Port    int             `json:"port"`
	Peers   []WireGuardPeer `json:"peers"`
}

type ServicePort struct {
	Name     string `json:"name"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Status   string `json:"status"`
	URL      string `json:"url"`
}

type Network struct {
	Interfaces []NetInterface `json:"interfaces"`
	Gateway    string         `json:"gateway"`
	DNS        []string       `json:"dns"`
	Hostname   string         `json:"hostname"`
	Domain     string         `json:"domain"`
	WireGuard  WireGuard      `json:"wireGuard"`
	Services   []ServicePort  `json:"services"`
}

type User struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	UID         int       `json:"uid"`
	Role        string    `json:"role"`
	Shares      []string  `json:"shares"`
	LastActive  time.Time `json:"lastActive"`
	Root        bool      `json:"root"`
	Color       string    `json:"color"`
}

type Plugin struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	Latest          string `json:"latest"`
	Author          string `json:"author"`
	Description     string `json:"description"`
	Category        string `json:"category"`
	UpdateAvailable bool   `json:"updateAvailable"`
	Enabled         bool   `json:"enabled"`
	Color           string `json:"color"`
}

type MoverRun struct {
	Started  time.Time `json:"started"`
	Finished time.Time `json:"finished"`
	Duration int64     `json:"duration"`
	Total    uint64    `json:"total"`
	Moved    uint64    `json:"moved"`
	Status   string    `json:"status"`
}

type MoverStatus struct {
	Running         bool       `json:"running"`
	Total           uint64     `json:"total"`
	Remain          uint64     `json:"remain"`
	Progress        float64    `json:"progress"`
	Throughput      float64    `json:"throughput"`
	ETA             int64      `json:"eta"`
	ShareTotal      uint64     `json:"shareTotal"`
	ShareRemain     uint64     `json:"shareRemain"`
	ShareThroughput float64    `json:"shareThroughput"`
	ShareETA        int64      `json:"shareEta"`
	File            string     `json:"file"`
	Action          string     `json:"action"`
	FilePercent     float64    `json:"filePercent"`
	PID             int        `json:"pid"`
	Updated         time.Time  `json:"updated"`
	History         []MoverRun `json:"history"`
}

type GPUProcess struct {
	PID    int    `json:"pid"`
	Name   string `json:"name"`
	Memory uint64 `json:"memory"`
}

type VMConsole struct {
	Name      string `json:"name"`
	Protocol  string `json:"protocol"`
	Port      int    `json:"port"`
	WebSocket int    `json:"webSocket"`
	Address   string `json:"address"`
}

type GPU struct {
	Index          int          `json:"index"`
	UUID           string       `json:"uuid"`
	Name           string       `json:"name"`
	Vendor         string       `json:"vendor"`
	Driver         string       `json:"driver"`
	MemoryTotal    uint64       `json:"memoryTotal"`
	MemoryUsed     uint64       `json:"memoryUsed"`
	Utilization    float64      `json:"utilization"`
	Temperature    float64      `json:"temperature"`
	Power          float64      `json:"power"`
	PowerLimit     float64      `json:"powerLimit"`
	Fan            float64      `json:"fan"`
	ClockMHz       int          `json:"clockMhz"`
	MemoryClockMHz int          `json:"memoryClockMhz"`
	Processes      []GPUProcess `json:"processes"`
}

type UPS struct {
	Present       bool    `json:"present"`
	Model         string  `json:"model"`
	Status        string  `json:"status"`
	Battery       float64 `json:"battery"`
	Load          float64 `json:"load"`
	Runtime       int     `json:"runtime"`
	InputVoltage  float64 `json:"inputVoltage"`
	OutputVoltage float64 `json:"outputVoltage"`
	NominalPower  float64 `json:"nominalPower"`
	LastEvent     string  `json:"lastEvent"`
	OnBattery     bool    `json:"onBattery"`
}

type Settings struct {
	Theme           string `json:"theme"`
	Accent          string `json:"accent"`
	Density         string `json:"density"`
	RefreshInterval int    `json:"refreshInterval"`
	Language        string `json:"language"`
	ServerName      string `json:"serverName"`
	AutoUpdate      bool   `json:"autoUpdate"`
	EmailAlerts     bool   `json:"emailAlerts"`
	PushAlerts      bool   `json:"pushAlerts"`
	Telemetry       bool   `json:"telemetry"`
	TemperatureUnit string `json:"temperatureUnit"`
}

type LogLine struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Source  string    `json:"source"`
	Message string    `json:"message"`
}

type Metrics struct {
	T         int64     `json:"t"`
	CPU       float64   `json:"cpu"`
	Memory    float64   `json:"memory"`
	NetRx     float64   `json:"netRx"`
	NetTx     float64   `json:"netTx"`
	DiskRead  float64   `json:"diskRead"`
	DiskWrite float64   `json:"diskWrite"`
	Temp      float64   `json:"temp"`
	Load      float64   `json:"load"`
	Cores     []float64 `json:"cores"`
}

type MetricPoint struct {
	T         int64   `json:"t"`
	CPU       float64 `json:"cpu"`
	Memory    float64 `json:"memory"`
	NetRx     float64 `json:"netRx"`
	NetTx     float64 `json:"netTx"`
	DiskRead  float64 `json:"diskRead"`
	DiskWrite float64 `json:"diskWrite"`
	Temp      float64 `json:"temp"`
}

type Snapshot struct {
	System        System         `json:"system"`
	Array         Array          `json:"array"`
	Disks         []Disk         `json:"disks"`
	Shares        []Share        `json:"shares"`
	Pools         []Pool         `json:"pools"`
	Containers    []Container    `json:"containers"`
	VMs           []VM           `json:"vms"`
	Network       Network        `json:"network"`
	Users         []User         `json:"users"`
	Plugins       []Plugin       `json:"plugins"`
	Notifications []Notification `json:"notifications"`
	GPUs          []GPU          `json:"gpus"`
	UPS           UPS            `json:"ups"`
	Settings      Settings       `json:"settings"`
	Metrics       Metrics        `json:"metrics"`
	History       []MetricPoint  `json:"history"`
	ServerTime    time.Time      `json:"serverTime"`
}

type Action struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
	Value  string `json:"value,omitempty"`
}

type Toast struct {
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	Message string `json:"message"`
}
