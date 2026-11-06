// Package provider exposes a read/write abstraction over an Unraid host.
package provider

import (
	"context"

	"aurora/internal/model"
)

// Provider is the contract the Unraid data source implements. The HTTP layer
// never needs to know where the values come from, which keeps handlers and
// tests independent from the host.
type Provider interface {
	Name() string
	Snapshot(ctx context.Context) (model.Snapshot, error)
	System(ctx context.Context) (model.System, error)
	Array(ctx context.Context) (model.Array, error)
	Disks(ctx context.Context) ([]model.Disk, error)
	Shares(ctx context.Context) ([]model.Share, error)
	RefreshShareUsage(ctx context.Context, name string) (int, error)
	Tasks(ctx context.Context) ([]model.Task, error)
	Pools(ctx context.Context) ([]model.Pool, error)
	Containers(ctx context.Context) ([]model.Container, error)
	ContainerLogs(ctx context.Context, id string, lines int) ([]model.LogLine, error)
	ContainerConnections(ctx context.Context, id string) (model.ContainerConnections, error)
	ContainerNetworkMap(ctx context.Context) (model.ContainerNetworkMap, error)
	ContainerIcon(ctx context.Context, id string) ([]byte, string, error)
	ContainerConfig(ctx context.Context, id string) (model.ContainerConfig, error)
	UpdateContainerConfig(ctx context.Context, id string, cfg model.ContainerConfig) error
	SetContainerAutostart(ctx context.Context, id string, enabled bool) error
	CreateContainer(ctx context.Context, opts model.ContainerCreateOptions) (string, error)
	RemoveContainer(ctx context.Context, id string) error
	CheckContainerUpdates(ctx context.Context) ([]model.ImageCheck, error)
	UpdateContainerImage(ctx context.Context, id string, report func(model.ContainerUpdateEvent)) error
	DockerNetworks(ctx context.Context) ([]string, error)
	ContainerTemplates(ctx context.Context) ([]model.ContainerTemplate, error)
	VMConsole(ctx context.Context, id string) (model.VMConsole, error)
	VMLogs(ctx context.Context, id string, lines int) ([]model.LogLine, error)
	VisualFolders(ctx context.Context, kind string) (model.VisualFolders, error)
	SaveVisualFolders(ctx context.Context, kind string, layout model.VisualFolders) error
	StartVMBackup(ctx context.Context, id, targetDir string) (model.VMBackup, error)
	VMBackupStatus(ctx context.Context, id string) (model.VMBackup, error)
	CancelVMBackup(ctx context.Context, id string) error
	StartVMRestore(ctx context.Context, id, backupDir string, confirm bool) (model.VMBackup, error)
	VMAutostartOrder(ctx context.Context) (model.VMAutostartOrder, error)
	SetVMAutostartOrder(ctx context.Context, order []string, managed bool) (model.VMAutostartOrder, error)
	ApplyVMAutostartOrder(ctx context.Context) (model.VMAutostartResult, error)
	SetVMAutostart(ctx context.Context, id string, enabled bool) error
	VMXML(ctx context.Context, id string) (string, error)
	CreateVM(ctx context.Context, opts model.VMCreateOptions) (model.VM, error)
	VMAdvanced(ctx context.Context, id string) (model.VMAdvancedConfig, error)
	UpdateVMAdvanced(ctx context.Context, id string, cfg model.VMAdvancedConfig) error
	VMHostDevices(ctx context.Context) ([]model.VMHostDevice, error)
	AttachVMUSB(ctx context.Context, id, vendor, product string) error
	DetachVMUSB(ctx context.Context, id, vendor, product string) error
	CloneVM(ctx context.Context, id string, opts model.VMCloneOptions) (model.VM, error)
	ResizeVMDisk(ctx context.Context, id string, opts model.VMDiskResizeOptions) error
	UpdateVMXML(ctx context.Context, id, raw string) error
	VMSnapshots(ctx context.Context, id string) ([]model.VMSnapshot, error)
	CreateVMSnapshot(ctx context.Context, id, name, description string) error
	RestoreVMSnapshot(ctx context.Context, id, name string) error
	DeleteVMSnapshot(ctx context.Context, id, name string) error
	SmartReport(ctx context.Context, device string) (model.SmartReport, error)
	RunSmartTest(ctx context.Context, device string) error
	Diagnostics(ctx context.Context) ([]byte, error)
	ConfigHistory(ctx context.Context) ([]model.ConfigHistory, error)
	ConfigHistoryDetail(ctx context.Context, id string) (model.ConfigHistory, error)
	RollbackConfig(ctx context.Context, id string) error
	BalanceDisks(ctx context.Context) ([]model.BalanceDisk, error)
	BalanceBrowse(ctx context.Context, disk, path string) ([]model.BalanceEntry, error)
	StartBalance(ctx context.Context, opts model.BalanceOptions) (model.BalanceJob, error)
	BalanceStatus(ctx context.Context) (model.BalanceJob, error)
	BalanceHistory(ctx context.Context) ([]model.BalanceJob, error)
	CancelBalance(ctx context.Context) error
	CheckPluginUpdates(ctx context.Context) ([]model.Plugin, error)
	UpdatePlugin(ctx context.Context, name string, report func(model.PluginEvent)) error
	InstallPlugin(ctx context.Context, url string, report func(model.PluginEvent)) error
	RemovePlugin(ctx context.Context, name string, report func(model.PluginEvent)) error
	VMs(ctx context.Context) ([]model.VM, error)
	UserScripts(ctx context.Context) ([]model.UserScript, error)
	ZFSPools(ctx context.Context) ([]model.ZFSPool, error)
	ScrubZFSPool(ctx context.Context, name string) error
	ZFSSnapshots(ctx context.Context, dataset string) ([]model.ZFSSnapshot, error)
	CreateZFSSnapshot(ctx context.Context, dataset, name string) error
	DeleteZFSSnapshot(ctx context.Context, dataset, name string) error
	CreateZFSDataset(ctx context.Context, opts model.ZFSDatasetCreate) error
	UpdateZFSDataset(ctx context.Context, opts model.ZFSDatasetUpdate) error
	DeleteZFSDataset(ctx context.Context, name string, recursive bool) error
	RollbackZFSSnapshot(ctx context.Context, dataset, name string) error
	CloneZFSSnapshot(ctx context.Context, opts model.ZFSSnapshotClone) error
	UserScript(ctx context.Context, id string) (string, error)
	RunUserScript(ctx context.Context, id string) (string, error)
	UpdateUserScript(ctx context.Context, id, content, frequency, cron string) error
	GPUs(ctx context.Context) ([]model.GPU, error)
	StartArray(ctx context.Context, opts model.ArrayStartOptions) error
	StopArray(ctx context.Context) error
	StorageLayout(ctx context.Context) (model.StorageLayout, error)
	SaveStorageLayout(ctx context.Context, layout model.StorageLayout) error
	Mover(ctx context.Context) (model.MoverStatus, error)
	StartMover(ctx context.Context) error
	StopMover(ctx context.Context) error
	VMIcon(ctx context.Context, id string) ([]byte, string, error)
	Network(ctx context.Context) (model.Network, error)
	NetworkConfig(ctx context.Context) (model.NetworkConfig, error)
	SaveNetworkConfig(ctx context.Context, cfg model.NetworkConfig, clientIP string, acknowledge bool) (model.NetworkConfig, error)
	SetNetworkInterfaceState(ctx context.Context, name string, up bool) error
	RevertNetworkConfig(ctx context.Context) (string, error)
	ApplyNetworkConfig(ctx context.Context) error
	OSStatus(ctx context.Context) (model.OSStatus, error)
	CheckOSUpdate(ctx context.Context) (model.OSStatus, error)
	InstallOSUpdate(ctx context.Context, report func(model.PluginEvent)) error
	RestorePreviousOS(ctx context.Context) (string, error)
	UnraidAPIStatus(ctx context.Context) (model.UnraidAPIStatus, error)
	UnraidAPIAction(ctx context.Context, action string) (string, error)
	UnraidAPILogs(ctx context.Context, lines int) ([]model.LogLine, error)
	TrivyStatus(ctx context.Context) (model.TrivyStatus, error)
	TrivyVersions(ctx context.Context) ([]model.TrivyVersion, error)
	StartTrivyInstall(version string) error
	StartTrivyDBUpdate() error
	AppsCatalog(ctx context.Context, query, kind, category, order string, limit, offset int) (model.AppCatalogPage, error)
	AppTemplate(ctx context.Context, templateURL string) (model.ContainerTemplate, error)
	AppIcon(ctx context.Context, iconURL string) ([]byte, string, error)
	SchedulerState(ctx context.Context) (model.SchedulerState, error)
	PreviewScheduler(settings model.SchedulerSettings) (model.SchedulerPreview, error)
	ApplyScheduler(ctx context.Context, settings model.SchedulerSettings) error
	UpdateDisplaySettings(ctx context.Context, settings model.SystemDisplaySettings) error
	ContainerSecurity(ctx context.Context, id string) (model.ContainerSecurityDetail, error)
	SecurityOverview(ctx context.Context) (model.SecurityOverview, error)
	ScanContainer(ctx context.Context, id string) error
	ScanAllContainers(ctx context.Context) (int, error)
	Users(ctx context.Context) ([]model.User, error)
	UserIcon(ctx context.Context, name string) ([]byte, string, error)
	ShareConfig(ctx context.Context, name string) (model.ShareConfig, error)
	CreateShare(ctx context.Context, cfg model.ShareConfig) error
	UpdateShareProperties(ctx context.Context, name string, cfg model.ShareConfig) error
	UpdateShareSMB(ctx context.Context, name string, cfg model.ShareConfig) error
	UpdateShareNFS(ctx context.Context, name string, cfg model.ShareConfig) error
	CreateUser(ctx context.Context, opts model.UserCreateOptions) error
	UpdateUser(ctx context.Context, name, description string) error
	ChangeUserPassword(ctx context.Context, name, password string) error
	DeleteUser(ctx context.Context, name string) error
	Plugins(ctx context.Context) ([]model.Plugin, error)
	Notifications(ctx context.Context) ([]model.Notification, error)
	ParityHistory(ctx context.Context) ([]model.ParityCheck, error)
	UPS(ctx context.Context) (model.UPS, error)
	UPSSettings(ctx context.Context) (model.UPSSettings, error)
	SaveUPSSettings(ctx context.Context, settings model.UPSSettings) error
	NUTSettings(ctx context.Context) (model.NUTSettings, error)
	SaveNUTSettings(ctx context.Context, settings model.NUTSettings) error
	Logs(ctx context.Context, source string, lines int) ([]model.LogLine, error)
	Settings(ctx context.Context) (model.Settings, error)
	SaveSettings(ctx context.Context, s model.Settings) error
	Metrics(ctx context.Context) (model.Metrics, error)
	// History returns the buffered metric series when the provider is
	// simulated; live providers return whatever they can cheaply derive.
	History(ctx context.Context) ([]model.MetricPoint, error)
	// Perform applies a whitelisted action (array start, container restart...).
	Perform(ctx context.Context, action model.Action) error
	Close() error
}

// Action kinds accepted by Perform.
const (
	ActionArrayStart       = "array.start"
	ActionArrayStop        = "array.stop"
	ActionArrayPause       = "array.pause"
	ActionDiskSpinUp       = "disk.spinup"
	ActionDiskSpinDown     = "disk.spindown"
	ActionContainerStart   = "container.start"
	ActionContainerStop    = "container.stop"
	ActionContainerRestart = "container.restart"
	ActionContainerUpdate  = "container.update"
	ActionVMStart          = "vm.start"
	ActionVMStop           = "vm.stop"
	ActionVMPause          = "vm.pause"
	ActionVMResume         = "vm.resume"
	ActionParityStart      = "parity.start"
	ActionParityStop       = "parity.stop"
	ActionParityPause      = "parity.pause"
	ActionParityResume     = "parity.resume"
	ActionNotifyAck        = "notification.ack"
	ActionNotifyAckAll     = "notification.ackall"
	ActionNotifyArchive    = "notification.archive"
)
