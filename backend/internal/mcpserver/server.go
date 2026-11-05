// Package mcpserver exposes Aurora as a Model Context Protocol server.
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
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
	// OnAction is called after every lifecycle action attempt so the HTTP
	// server can write a precise audit entry.
	OnAction func(tool, target string, err error)
}

type service struct {
	opts Options
}

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
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return New(options(r))
	}, nil)
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

func addReadTools(s *service, server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{Name: "get_system", Description: "Host identity, CPU, memory, load, uptime and array summary."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			system, err := s.opts.Provider.System(ctx)
			if err != nil {
				return s.fail("get system", err)
			}
			return s.text(system)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_array", Description: "Array state, capacity, parity, rebuild and sync status."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			array, err := s.opts.Provider.Array(ctx)
			if err != nil {
				return s.fail("get array", err)
			}
			return s.text(array)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_disks", Description: "Physical array disks with state, temperature, usage and SMART summary."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			disks, err := s.opts.Provider.Disks(ctx)
			if err != nil {
				return s.fail("list disks", err)
			}
			return s.text(disks)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_shares", Description: "User shares with allocation, cache and export settings."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			shares, err := s.opts.Provider.Shares(ctx)
			if err != nil {
				return s.fail("list shares", err)
			}
			return s.text(shares)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_pools", Description: "Storage pools and their devices."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			pools, err := s.opts.Provider.Pools(ctx)
			if err != nil {
				return s.fail("list pools", err)
			}
			return s.text(pools)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_containers", Description: "Docker containers with state, image, uptime and live resources."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			containers, err := s.opts.Provider.Containers(ctx)
			if err != nil {
				return s.fail("list containers", err)
			}
			return s.text(containers)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "container_logs", Description: "Tail the logs of a Docker container."},
		func(ctx context.Context, _ *mcp.CallToolRequest, args logsArgs) (*mcp.CallToolResult, any, error) {
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

	mcp.AddTool(server, &mcp.Tool{Name: "list_vms", Description: "libvirt virtual machines with state, resources, IP and autostart."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			vms, err := s.opts.Provider.VMs(ctx)
			if err != nil {
				return s.fail("list virtual machines", err)
			}
			return s.text(vms)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_vm", Description: "Details of one virtual machine by ID or name."},
		func(ctx context.Context, _ *mcp.CallToolRequest, args targetArgs) (*mcp.CallToolResult, any, error) {
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

	mcp.AddTool(server, &mcp.Tool{Name: "vm_logs", Description: "Tail the libvirt domain log of a virtual machine."},
		func(ctx context.Context, _ *mcp.CallToolRequest, args logsArgs) (*mcp.CallToolResult, any, error) {
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

	mcp.AddTool(server, &mcp.Tool{Name: "list_zfs_pools", Description: "ZFS pools, datasets and health."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			pools, err := s.opts.Provider.ZFSPools(ctx)
			if err != nil {
				return s.fail("list ZFS pools", err)
			}
			return s.text(pools)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_gpus", Description: "GPU inventory, driver, utilisation and temperature."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			gpus, err := s.opts.Provider.GPUs(ctx)
			if err != nil {
				return s.fail("list GPUs", err)
			}
			return s.text(gpus)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_ups", Description: "UPS battery, load, runtime and power state."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			ups, err := s.opts.Provider.UPS(ctx)
			if err != nil {
				return s.fail("get UPS", err)
			}
			return s.text(ups)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_metrics", Description: "Latest CPU, memory, network, disk and temperature sample."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			metrics, err := s.opts.Provider.Metrics(ctx)
			if err != nil {
				return s.fail("get metrics", err)
			}
			return s.text(metrics)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "get_metrics_history", Description: "Recent metric samples for trend analysis."},
		func(ctx context.Context, _ *mcp.CallToolRequest, args historyArgs) (*mcp.CallToolResult, any, error) {
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

	mcp.AddTool(server, &mcp.Tool{Name: "list_tasks", Description: "Background tasks with status and PID."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			tasks, err := s.opts.Provider.Tasks(ctx)
			if err != nil {
				return s.fail("list tasks", err)
			}
			return s.text(tasks)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_notifications", Description: "Unraid notifications and their importance."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
			notifications, err := s.opts.Provider.Notifications(ctx)
			if err != nil {
				return s.fail("list notifications", err)
			}
			return s.text(notifications)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "list_plugins", Description: "Installed Unraid plugins and update state."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ emptyArgs) (*mcp.CallToolResult, any, error) {
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
	}{
		{"start_container", "Start a Docker container (operator+).", provider.ActionContainerStart},
		{"stop_container", "Stop a Docker container (operator+).", provider.ActionContainerStop},
		{"restart_container", "Restart a Docker container (operator+).", provider.ActionContainerRestart},
		{"start_vm", "Start a virtual machine (operator+).", provider.ActionVMStart},
		{"shutdown_vm", "Gracefully shut down a virtual machine (operator+).", provider.ActionVMStop},
		{"pause_vm", "Suspend a virtual machine (operator+).", provider.ActionVMPause},
		{"resume_vm", "Resume a suspended virtual machine (operator+).", provider.ActionVMResume},
	}
	for _, action := range actions {
		action := action
		mcp.AddTool(server, &mcp.Tool{Name: action.name, Description: action.description},
			func(ctx context.Context, _ *mcp.CallToolRequest, args targetArgs) (*mcp.CallToolResult, any, error) {
				if s.opts.ReadOnly || !s.opts.AllowActions {
					return nil, nil, errors.New("actions are disabled: this MCP session is read-only")
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
