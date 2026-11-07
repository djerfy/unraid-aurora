// Package mcpserver exposes Aurora as a Model Context Protocol server.
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"aurora/internal/model"
	"aurora/internal/provider"
)

// Options controls what a single MCP session may do.
type Options struct {
	Provider     provider.Provider
	Version      string
	ReadOnly     bool
	AllowActions bool
	Role         string
	Scopes       []string
	// OnAction is called after every lifecycle action attempt so the HTTP
	// server can write a precise audit entry.
	OnAction func(tool, target string, err error)
}

type service struct {
	opts Options
}

// readOnly is attached to every read tool so clients such as Hermes can run
// them without an approval prompt.
var readOnly = &mcp.ToolAnnotations{ReadOnlyHint: true}

// New builds an MCP server with tools, resources and prompts for one policy.
func New(opts Options) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "aurora", Version: opts.Version}, &mcp.ServerOptions{
		Instructions: "Aurora is a live control plane for Unraid 7.x. Read tools are always available. " +
			"Lifecycle actions require an operator or admin credential and are disabled in read-only mode. " +
			"Read state before acting, and re-read it to confirm an action.",
	})
	svc := &service{opts: opts}
	addReadTools(svc, server)
	addActionTools(svc, server)
	addResources(svc, server)
	addPrompts(server)
	return server
}

// RunStdio serves MCP over stdin/stdout. Stdout is reserved for JSON-RPC.
func RunStdio(ctx context.Context, opts Options) error {
	return New(opts).Run(ctx, &mcp.StdioTransport{})
}

// HTTPHandler serves MCP over the Streamable HTTP transport. options is called
// for every new session and can read the authenticated principal from the
// request context.
func HTTPHandler(options func(*http.Request) Options) http.Handler {
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return New(options(r))
	}, &mcp.StreamableHTTPOptions{
		// JSON responses are easier for the lightweight MCP clients used by
		// agent frameworks; the official SDK accepts both JSON and SSE.
		JSONResponse: true,
		Logger:       slog.Default(),
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originalAccept := r.Header.Get("Accept")
		// Some clients only send "Accept: application/json", while the
		// Streamable HTTP transport requires both media types. Complete the
		// header instead of rejecting an otherwise valid client.
		if !strings.Contains(originalAccept, "application/json") || !strings.Contains(originalAccept, "text/event-stream") {
			r.Header.Set("Accept", "application/json, text/event-stream")
		}
		recorder := &mcpStatusWriter{ResponseWriter: w, status: http.StatusOK}
		handler.ServeHTTP(recorder, r)
		if recorder.status == http.StatusBadRequest {
			slog.Warn("MCP request rejected",
				"method", r.Method,
				"accept", originalAccept,
				"contentType", r.Header.Get("Content-Type"),
				"protocol", r.Header.Get("MCP-Protocol-Version"),
				"session", r.Header.Get("Mcp-Session-Id"),
			)
		}
	})
}

func (s *service) text(value any) (*mcp.CallToolResult, any, error) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}}, nil, nil
}

func (s *service) fail(label string, err error) (*mcp.CallToolResult, any, error) {
	return nil, nil, fmt.Errorf("%s: %w", label, err)
}

// require enforces the scopes of the API token. An unscoped token may use the
// whole tool catalog.
type mcpStatusWriter struct {
	http.ResponseWriter
	status int
}

func (w *mcpStatusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *mcpStatusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (s *service) require(scope string) error {
	if len(s.opts.Scopes) == 0 {
		return nil
	}
	for _, allowed := range s.opts.Scopes {
		if allowed == scope {
			return nil
		}
	}
	return fmt.Errorf("this API token has no %q scope", scope)
}

type emptyArgs struct{}

type targetArgs struct {
	Target string `json:"target" jsonschema:"container ID or name, or virtual machine ID or name"`
}

type logsArgs struct {
	ID    string `json:"id" jsonschema:"container or virtual machine ID or name"`
	Lines int    `json:"lines,omitempty" jsonschema:"number of lines to return, between 1 and 1000"`
}

type historyArgs struct {
	Minutes int `json:"minutes,omitempty" jsonschema:"lookback window in minutes, between 1 and 1440 (default 60)"`
}

type networkMapArgs struct {
	Container string `json:"container,omitempty" jsonschema:"optional container ID or name; omit for every running container"`
}

type priorityArgs struct {
	Target   string `json:"target" jsonschema:"container ID or name"`
	Priority int    `json:"priority" jsonschema:"startup priority from 0 (first) to 100 (last), default 50"`
}

type imageArgs struct {
	ID string `json:"id" jsonschema:"Docker image ID"`
}

type containerUpdateCheck struct {
	Container string `json:"container"`
	ID        string `json:"id"`
	Image     string `json:"image"`
	State     string `json:"state"`
	Update    bool   `json:"update"`
	Local     string `json:"local,omitempty"`
	Remote    string `json:"remote,omitempty"`
	Error     string `json:"error,omitempty"`
}

type containerUpdateOutcome struct {
	Container string                       `json:"container"`
	ID        string                       `json:"id"`
	Image     string                       `json:"image"`
	OK        bool                         `json:"ok"`
	Error     string                       `json:"error,omitempty"`
	Events    []model.ContainerUpdateEvent `json:"events,omitempty"`
}

type containerUpdateReport struct {
	OK         bool                     `json:"ok"`
	Checked    int                      `json:"checked"`
	Updates    int                      `json:"updates"`
	Updated    int                      `json:"updated"`
	Error      string                   `json:"error,omitempty"`
	Containers []containerUpdateOutcome `json:"containers"`
}

// updateOneContainer collects the pull/recreate stages so the MCP client gets
// the same progress trail as the Docker page.
func (s *service) updateOneContainer(ctx context.Context, container model.Container) (containerUpdateOutcome, error) {
	outcome := containerUpdateOutcome{Container: container.Name, ID: container.ID, Image: container.Image}
	var mu sync.Mutex
	events := make([]model.ContainerUpdateEvent, 0, 16)
	err := s.opts.Provider.UpdateContainerImage(ctx, container.ID, func(event model.ContainerUpdateEvent) {
		mu.Lock()
		events = append(events, event)
		if len(events) > 40 {
			events = events[len(events)-40:]
		}
		mu.Unlock()
	})
	mu.Lock()
	outcome.Events = append([]model.ContainerUpdateEvent(nil), events...)
	mu.Unlock()
	if err != nil {
		outcome.Error = err.Error()
		return outcome, err
	}
	outcome.OK = true
	return outcome, nil
}

func (s *service) findContainer(ctx context.Context, target string) (model.Container, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return model.Container{}, errors.New("container is required")
	}
	containers, err := s.opts.Provider.Containers(ctx)
	if err != nil {
		return model.Container{}, err
	}
	for _, container := range containers {
		if container.ID == target || container.Name == target {
			return container, nil
		}
	}
	return model.Container{}, fmt.Errorf("container %q not found", target)
}

func addReadTools(s *service, server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{Name: "get_system", Description: "Host identity, CPU, memory, load, uptime and array summary.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("system"); err != nil {
				return nil, nil, err
			}
			system, err := s.opts.Provider.System(ctx)
			if err != nil {
				return s.fail("get system", err)
			}
			return s.text(system)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_array", Description: "Array state, capacity, parity, rebuild and sync status.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("storage"); err != nil {
				return nil, nil, err
			}
			array, err := s.opts.Provider.Array(ctx)
			if err != nil {
				return s.fail("get array", err)
			}
			return s.text(array)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_disks", Description: "Physical array disks with state, temperature, usage and SMART summary.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("storage"); err != nil {
				return nil, nil, err
			}
			disks, err := s.opts.Provider.Disks(ctx)
			if err != nil {
				return s.fail("list disks", err)
			}
			return s.text(disks)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_shares", Description: "User shares with allocation, cache and export settings.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("storage"); err != nil {
				return nil, nil, err
			}
			shares, err := s.opts.Provider.Shares(ctx)
			if err != nil {
				return s.fail("list shares", err)
			}
			return s.text(shares)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_pools", Description: "Storage pools and their devices.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("storage"); err != nil {
				return nil, nil, err
			}
			pools, err := s.opts.Provider.Pools(ctx)
			if err != nil {
				return s.fail("list pools", err)
			}
			return s.text(pools)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_containers", Description: "Docker containers with state, image, uptime and live resources.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("docker"); err != nil {
				return nil, nil, err
			}
			containers, err := s.opts.Provider.Containers(ctx)
			if err != nil {
				return s.fail("list containers", err)
			}
			return s.text(containers)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "check_container_updates", Description: "Check registry digests for every container image and report which containers have an update.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("docker"); err != nil {
				return nil, nil, err
			}
			checks, err := s.opts.Provider.CheckContainerUpdates(ctx)
			if err != nil {
				return s.fail("check container updates", err)
			}
			containers, err := s.opts.Provider.Containers(ctx)
			if err != nil {
				return s.fail("list containers", err)
			}
			byImage := make(map[string]model.ImageCheck, len(checks))
			for _, check := range checks {
				byImage[check.Image] = check
			}
			rows := make([]containerUpdateCheck, 0, len(containers))
			updates := 0
			for _, container := range containers {
				check, ok := byImage[container.Image]
				row := containerUpdateCheck{
					Container: container.Name, ID: container.ID, Image: container.Image,
					State: container.State, Update: ok && check.Update,
				}
				if ok {
					row.Local, row.Remote, row.Error = check.Local, check.Remote, check.Error
				}
				if row.Update {
					updates++
				}
				rows = append(rows, row)
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Container < rows[j].Container })
			return s.text(map[string]any{
				"checkedAt": time.Now().UTC(), "updates": updates, "containers": rows,
			})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_images", Description: "Docker images with tags, size and the containers using them.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("docker"); err != nil {
				return nil, nil, err
			}
			images, err := s.opts.Provider.ContainerImages(ctx)
			if err != nil {
				return s.fail("list images", err)
			}
			return s.text(images)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_network_map", Description: "Container-to-peer network graph with connection directions.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, args networkMapArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("docker"); err != nil {
				return nil, nil, err
			}
			containerID := ""
			if strings.TrimSpace(args.Container) != "" {
				container, err := s.findContainer(ctx, args.Container)
				if err != nil {
					return s.fail("get network map", err)
				}
				containerID = container.ID
			}
			networkMap, err := s.opts.Provider.ContainerNetworkMap(ctx, containerID)
			if err != nil {
				return s.fail("get network map", err)
			}
			return s.text(networkMap)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "container_logs", Description: "Tail the logs of a Docker container.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, args logsArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("docker"); err != nil {
				return nil, nil, err
			}
			lines := args.Lines
			if lines <= 0 {
				lines = 120
			}
			logs, err := s.opts.Provider.ContainerLogs(ctx, args.ID, lines)
			if err != nil {
				return s.fail("container logs", err)
			}
			return s.text(logs)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_vms", Description: "libvirt virtual machines with state, resources, IP and autostart.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("vms"); err != nil {
				return nil, nil, err
			}
			vms, err := s.opts.Provider.VMs(ctx)
			if err != nil {
				return s.fail("list virtual machines", err)
			}
			return s.text(vms)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_vm", Description: "Details of one virtual machine by ID or name.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, args targetArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("vms"); err != nil {
				return nil, nil, err
			}
			vms, err := s.opts.Provider.VMs(ctx)
			if err != nil {
				return s.fail("get virtual machine", err)
			}
			for _, vm := range vms {
				if vm.ID == args.Target || vm.Name == args.Target {
					return s.text(vm)
				}
			}
			return nil, nil, fmt.Errorf("virtual machine %q not found", args.Target)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "vm_logs", Description: "Tail the libvirt domain log of a virtual machine.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, args logsArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("vms"); err != nil {
				return nil, nil, err
			}
			lines := args.Lines
			if lines <= 0 {
				lines = 240
			}
			logs, err := s.opts.Provider.VMLogs(ctx, args.ID, lines)
			if err != nil {
				return s.fail("virtual machine logs", err)
			}
			return s.text(logs)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_zfs_pools", Description: "ZFS pools, datasets and health.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("storage"); err != nil {
				return nil, nil, err
			}
			pools, err := s.opts.Provider.ZFSPools(ctx)
			if err != nil {
				return s.fail("list ZFS pools", err)
			}
			return s.text(pools)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_gpus", Description: "GPU inventory, driver, utilisation and temperature.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("system"); err != nil {
				return nil, nil, err
			}
			gpus, err := s.opts.Provider.GPUs(ctx)
			if err != nil {
				return s.fail("list GPUs", err)
			}
			return s.text(gpus)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_ups", Description: "UPS battery, load, runtime and power state.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("ups"); err != nil {
				return nil, nil, err
			}
			ups, err := s.opts.Provider.UPS(ctx)
			if err != nil {
				return s.fail("get UPS", err)
			}
			return s.text(ups)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_metrics", Description: "Latest CPU, memory, network, disk and temperature sample.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("system"); err != nil {
				return nil, nil, err
			}
			metrics, err := s.opts.Provider.Metrics(ctx)
			if err != nil {
				return s.fail("get metrics", err)
			}
			return s.text(metrics)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_metrics_history", Description: "Recent metric samples for trend analysis.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, args historyArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("system"); err != nil {
				return nil, nil, err
			}
			minutes := args.Minutes
			if minutes <= 0 {
				minutes = 60
			}
			if minutes > 1440 {
				minutes = 1440
			}
			points, err := s.opts.Provider.History(ctx)
			if err != nil {
				return s.fail("get metric history", err)
			}
			cutoff := time.Now().Add(-time.Duration(minutes) * time.Minute).Unix()
			filtered := make([]model.MetricPoint, 0, len(points))
			for _, point := range points {
				if point.T >= cutoff {
					filtered = append(filtered, point)
				}
			}
			return s.text(filtered)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_tasks", Description: "Background tasks with status and PID.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("system"); err != nil {
				return nil, nil, err
			}
			tasks, err := s.opts.Provider.Tasks(ctx)
			if err != nil {
				return s.fail("list tasks", err)
			}
			return s.text(tasks)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_notifications", Description: "Unraid notifications and their importance.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("system"); err != nil {
				return nil, nil, err
			}
			notifications, err := s.opts.Provider.Notifications(ctx)
			if err != nil {
				return s.fail("list notifications", err)
			}
			return s.text(notifications)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_plugins", Description: "Installed Unraid plugins and update state.", Annotations: readOnly},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			if err := s.require("apps"); err != nil {
				return nil, nil, err
			}
			plugins, err := s.opts.Provider.Plugins(ctx)
			if err != nil {
				return s.fail("list plugins", err)
			}
			return s.text(plugins)
		})
}

func addActionTools(s *service, server *mcp.Server) {
	actions := []struct {
		name        string
		description string
		kind        string
		scope       string
	}{
		{"start_container", "Start a Docker container (operator+).", provider.ActionContainerStart, "docker"},
		{"stop_container", "Stop a Docker container (operator+).", provider.ActionContainerStop, "docker"},
		{"restart_container", "Restart a Docker container (operator+).", provider.ActionContainerRestart, "docker"},
		{"start_vm", "Start a virtual machine (operator+).", provider.ActionVMStart, "vms"},
		{"shutdown_vm", "Gracefully shut down a virtual machine (operator+).", provider.ActionVMStop, "vms"},
		{"pause_vm", "Suspend a virtual machine (operator+).", provider.ActionVMPause, "vms"},
		{"resume_vm", "Resume a suspended virtual machine (operator+).", provider.ActionVMResume, "vms"},
	}
	for _, action := range actions {
		action := action
		mcp.AddTool(server, &mcp.Tool{Name: action.name, Description: action.description},
			func(ctx context.Context, _ *mcp.CallToolRequest, args targetArgs) (*mcp.CallToolResult, any, error) {
				if s.opts.ReadOnly || !s.opts.AllowActions {
					return nil, nil, errors.New("actions are disabled: this MCP session is read-only")
				}
				if err := s.require(action.scope); err != nil {
					return nil, nil, err
				}
				target := strings.TrimSpace(args.Target)
				if target == "" {
					return nil, nil, errors.New("target is required")
				}
				err := s.opts.Provider.Perform(ctx, model.Action{Kind: action.kind, Target: target})
				if s.opts.OnAction != nil {
					s.opts.OnAction(action.name, target, err)
				}
				if err != nil {
					return s.fail(action.name, err)
				}
				return s.text(map[string]any{"ok": true, "action": action.name, "target": target})
			})
	}

	mcp.AddTool(server, &mcp.Tool{Name: "update_container", Description: "Pull the latest image and recreate a container with its current settings; the container may restart (operator+)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, args targetArgs) (*mcp.CallToolResult, any, error) {
			if s.opts.ReadOnly || !s.opts.AllowActions {
				return nil, nil, errors.New("actions are disabled: this MCP session is read-only")
			}
			if err := s.require("docker"); err != nil {
				return nil, nil, err
			}
			container, err := s.findContainer(ctx, args.Target)
			if err != nil {
				return s.fail("update container", err)
			}
			outcome, updateErr := s.updateOneContainer(context.WithoutCancel(ctx), container)
			if s.opts.OnAction != nil {
				s.opts.OnAction("update_container", container.Name, updateErr)
			}
			result, structured, err := s.text(outcome)
			if err != nil {
				return nil, nil, err
			}
			if updateErr != nil {
				result.IsError = true
			}
			return result, structured, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "update_all_containers", Description: "Check for updates, then pull and recreate every container that has one, stopping at the first failure (operator+)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			if s.opts.ReadOnly || !s.opts.AllowActions {
				return nil, nil, errors.New("actions are disabled: this MCP session is read-only")
			}
			if err := s.require("docker"); err != nil {
				return nil, nil, err
			}
			checks, err := s.opts.Provider.CheckContainerUpdates(ctx)
			if err != nil {
				return s.fail("check container updates", err)
			}
			containers, err := s.opts.Provider.Containers(ctx)
			if err != nil {
				return s.fail("list containers", err)
			}
			updateImages := make(map[string]bool, len(checks))
			for _, check := range checks {
				if check.Update {
					updateImages[check.Image] = true
				}
			}
			targets := make([]model.Container, 0, len(containers))
			for _, container := range containers {
				if updateImages[container.Image] {
					targets = append(targets, container)
				}
			}
			sort.Slice(targets, func(i, j int) bool { return targets[i].Name < targets[j].Name })
			report := containerUpdateReport{OK: true, Checked: len(containers), Updates: len(targets)}
			updateCtx := context.WithoutCancel(ctx)
			for _, container := range targets {
				outcome, updateErr := s.updateOneContainer(updateCtx, container)
				report.Containers = append(report.Containers, outcome)
				if s.opts.OnAction != nil {
					s.opts.OnAction("update_all_containers", container.Name, updateErr)
				}
				if updateErr != nil {
					report.OK = false
					report.Error = updateErr.Error()
					break
				}
				report.Updated++
			}
			result, structured, err := s.text(report)
			if err != nil {
				return nil, nil, err
			}
			if !report.OK {
				result.IsError = true
			}
			return result, structured, nil
		})

	mcp.AddTool(server, &mcp.Tool{Name: "set_container_priority", Description: "Set a container startup priority from 0 (first) to 100 (last)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, args priorityArgs) (*mcp.CallToolResult, any, error) {
			if s.opts.ReadOnly || !s.opts.AllowActions {
				return nil, nil, errors.New("actions are disabled: this MCP session is read-only")
			}
			if err := s.require("docker"); err != nil {
				return nil, nil, err
			}
			if args.Priority < 0 || args.Priority > 100 {
				return nil, nil, errors.New("priority must be between 0 and 100")
			}
			container, err := s.findContainer(ctx, args.Target)
			if err != nil {
				return s.fail("set container priority", err)
			}
			_, err = s.opts.Provider.SaveContainerPriorities(ctx, map[string]int{container.Name: args.Priority})
			if s.opts.OnAction != nil {
				s.opts.OnAction("set_container_priority", container.Name, err)
			}
			if err != nil {
				return s.fail("set container priority", err)
			}
			return s.text(map[string]any{"ok": true, "container": container.Name, "priority": args.Priority})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "remove_image", Description: "Remove an unused Docker image without force."},
		func(ctx context.Context, _ *mcp.CallToolRequest, args imageArgs) (*mcp.CallToolResult, any, error) {
			if s.opts.ReadOnly || !s.opts.AllowActions {
				return nil, nil, errors.New("actions are disabled: this MCP session is read-only")
			}
			if err := s.require("docker"); err != nil {
				return nil, nil, err
			}
			id := strings.TrimSpace(args.ID)
			err := s.opts.Provider.RemoveContainerImage(ctx, id)
			if s.opts.OnAction != nil {
				s.opts.OnAction("remove_image", id, err)
			}
			if err != nil {
				return s.fail("remove image", err)
			}
			return s.text(map[string]any{"ok": true, "id": id})
		})

	mcp.AddTool(server, &mcp.Tool{Name: "remove_unused_images", Description: "Remove every Docker image not used by a container; used images are skipped."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			if s.opts.ReadOnly || !s.opts.AllowActions {
				return nil, nil, errors.New("actions are disabled: this MCP session is read-only")
			}
			if err := s.require("docker"); err != nil {
				return nil, nil, err
			}
			result, err := s.opts.Provider.RemoveUnusedContainerImages(ctx)
			if s.opts.OnAction != nil {
				s.opts.OnAction("remove_unused_images", "", err)
			}
			if err != nil {
				return s.fail("remove unused images", err)
			}
			return s.text(result)
		})
}

func addResources(s *service, server *mcp.Server) {
	server.AddResource(&mcp.Resource{
		URI: "aurora://summary", Name: "Aurora summary", Description: "Host, array, running containers and VMs in one JSON document.",
		MIMEType: "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return s.resourceJSON(ctx, req.Params.URI, func(ctx context.Context) (any, error) {
			system, err := s.opts.Provider.System(ctx)
			if err != nil {
				return nil, err
			}
			array, err := s.opts.Provider.Array(ctx)
			if err != nil {
				return nil, err
			}
			containers, err := s.opts.Provider.Containers(ctx)
			if err != nil {
				return nil, err
			}
			vms, err := s.opts.Provider.VMs(ctx)
			if err != nil {
				return nil, err
			}
			return map[string]any{"system": system, "array": array, "containers": containers, "vms": vms}, nil
		})
	})

	server.AddResource(&mcp.Resource{
		URI: "aurora://vms", Name: "Virtual machines", Description: "libvirt virtual machines.", MIMEType: "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return s.resourceJSON(ctx, req.Params.URI, func(ctx context.Context) (any, error) {
			return s.opts.Provider.VMs(ctx)
		})
	})

	server.AddResource(&mcp.Resource{
		URI: "aurora://containers", Name: "Containers", Description: "Docker containers.", MIMEType: "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return s.resourceJSON(ctx, req.Params.URI, func(ctx context.Context) (any, error) {
			return s.opts.Provider.Containers(ctx)
		})
	})

	server.AddResource(&mcp.Resource{
		URI: "aurora://array", Name: "Array", Description: "Array state and capacity.", MIMEType: "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return s.resourceJSON(ctx, req.Params.URI, func(ctx context.Context) (any, error) {
			return s.opts.Provider.Array(ctx)
		})
	})
}

func (s *service) resourceJSON(ctx context.Context, uri string, build func(context.Context) (any, error)) (*mcp.ReadResourceResult, error) {
	value, err := build(ctx)
	if err != nil {
		return nil, err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
		URI: uri, MIMEType: "application/json", Text: string(raw),
	}}}, nil
}

func addPrompts(server *mcp.Server) {
	server.AddPrompt(&mcp.Prompt{
		Name: "host-health", Title: "Host health review",
		Description: "Review the Unraid host and report the most important issues first.",
	}, func(_ context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return promptResult("Host health review",
			"Review this Unraid host. Call get_array, list_disks, get_metrics, list_notifications and list_tasks. "+
				"Report the array state, unhealthy disks, parity or rebuild progress, resource pressure and open alerts. "+
				"Order findings by severity and cite the values you used."), nil
	})

	server.AddPrompt(&mcp.Prompt{
		Name: "container-triage", Title: "Container triage",
		Description: "Diagnose a container that fails, restarts or consumes too much.",
		Arguments:   []*mcp.PromptArgument{{Name: "container", Description: "Container name or ID", Required: true}},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		name := strings.TrimSpace(req.Params.Arguments["container"])
		if name == "" {
			name = "<container>"
		}
		return promptResult("Container triage",
			"Diagnose the container "+name+
				". Call list_containers, then container_logs for it. Check its state, image, restart count, CPU and memory. "+
				"Summarise the failure mode, cite log lines, and propose the smallest safe fix. Do not restart it without being asked."), nil
	})

	server.AddPrompt(&mcp.Prompt{
		Name: "vm-triage", Title: "Virtual machine triage",
		Description: "Diagnose a virtual machine that is down, slow or not booting.",
		Arguments:   []*mcp.PromptArgument{{Name: "vm", Description: "Virtual machine name or ID", Required: true}},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		name := strings.TrimSpace(req.Params.Arguments["vm"])
		if name == "" {
			name = "<vm>"
		}
		return promptResult("Virtual machine triage",
			"Diagnose the virtual machine "+name+
				". Call get_vm and vm_logs. Check its state, resource usage, console availability and the libvirt log. "+
				"Cite the decisive log lines and propose the smallest safe fix. Do not start or stop it without being asked."), nil
	})
}

func promptResult(description, text string) *mcp.GetPromptResult {
	return &mcp.GetPromptResult{
		Description: description,
		Messages: []*mcp.PromptMessage{{
			Role: "user", Content: &mcp.TextContent{Text: text},
		}},
	}
}
