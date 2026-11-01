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
	Pools(ctx context.Context) ([]model.Pool, error)
	Containers(ctx context.Context) ([]model.Container, error)
	ContainerLogs(ctx context.Context, id string, lines int) ([]model.LogLine, error)
	ContainerIcon(ctx context.Context, id string) ([]byte, string, error)
	ContainerConfig(ctx context.Context, id string) (model.ContainerConfig, error)
	UpdateContainerConfig(ctx context.Context, id string, cfg model.ContainerConfig) error
	CreateContainer(ctx context.Context, opts model.ContainerCreateOptions) (string, error)
	RemoveContainer(ctx context.Context, id string) error
	CheckContainerUpdates(ctx context.Context) ([]model.ImageCheck, error)
	DockerNetworks(ctx context.Context) ([]string, error)
	VMs(ctx context.Context) ([]model.VM, error)
	GPUs(ctx context.Context) ([]model.GPU, error)
	StartArray(ctx context.Context, opts model.ArrayStartOptions) error
	StopArray(ctx context.Context) error
	Mover(ctx context.Context) (model.MoverStatus, error)
	StartMover(ctx context.Context) error
	StopMover(ctx context.Context) error
	VMIcon(ctx context.Context, id string) ([]byte, string, error)
	Network(ctx context.Context) (model.Network, error)
	Users(ctx context.Context) ([]model.User, error)
	UserIcon(ctx context.Context, name string) ([]byte, string, error)
	Plugins(ctx context.Context) ([]model.Plugin, error)
	Notifications(ctx context.Context) ([]model.Notification, error)
	ParityHistory(ctx context.Context) ([]model.ParityCheck, error)
	UPS(ctx context.Context) (model.UPS, error)
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
