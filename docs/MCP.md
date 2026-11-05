# MCP server

Aurora embeds a [Model Context Protocol](https://modelcontextprotocol.io) server so AI clients can read
the host and run a controlled set of lifecycle actions. The same tool catalog is served over two
transports: **stdio** for local clients and **Streamable HTTP** for remote clients.

## Transports

### stdio (local)

```bash
aurora mcp                  # read-only
aurora mcp --allow-actions  # operator actions, still blocked by AURORA_READ_ONLY=1
```

`AURORA_MCP_ALLOW_ACTIONS=1` is the environment equivalent. Stdout carries JSON-RPC only; logs are
written to stderr. The process needs the same privileges as Aurora: root, the Docker socket and
`virsh`.

Claude Desktop / Cursor configuration:

```json
{
  "mcpServers": {
    "aurora": {
      "command": "/usr/local/bin/aurora",
      "args": ["mcp"]
    }
  }
}
```

### Streamable HTTP (remote)

The daemon exposes `https://<server>:8484/api/v1/mcp` once authentication succeeds.

- Use an **unscoped API token** created in **Settings → API tokens**. Scoped tokens are refused on
  this endpoint because MCP spans every subsystem.
- `viewer` tokens get the read tools only. `operator` and `admin` tokens can also run lifecycle
  actions, unless the server is in read-only mode or the token itself is read-only.
- Authenticate with `Authorization: Bearer <token>`. Browser cookie sessions require CSRF headers,
  so they are not usable from MCP clients.
- The endpoint inherits Aurora's TLS configuration, security headers and access log.

Clients without native Streamable HTTP support can use a local bridge such as `mcp-remote`.

## Tools

Read tools are available to every allowed session:

| Tool | Description |
|---|---|
| `get_system` | Host identity, CPU, memory, load and uptime |
| `get_array` | Array state, capacity, parity, rebuild and sync |
| `list_disks` | Array disks, temperature, usage and SMART summary |
| `list_shares` | User shares and their settings |
| `list_pools` | Storage pools and devices |
| `list_containers` | Docker containers with state and live resources |
| `container_logs` | Tail a container log |
| `list_vms` | libvirt virtual machines |
| `get_vm` | One virtual machine by ID or name |
| `vm_logs` | Tail a libvirt domain log |
| `list_zfs_pools` | ZFS pools, datasets and health |
| `list_gpus` | GPU inventory and telemetry |
| `get_ups` | UPS state and battery |
| `get_metrics` | Latest metric sample |
| `get_metrics_history` | Recent samples for trend analysis |
| `list_tasks` | Background tasks and PIDs |
| `list_notifications` | Unraid notifications |
| `list_plugins` | Installed plugins |

Lifecycle actions require an operator/admin session with `AllowActions`:

`start_container`, `stop_container`, `restart_container`, `start_vm`, `shutdown_vm`, `pause_vm`,
`resume_vm`.

## Resources

| URI | Contents |
|---|---|
| `aurora://summary` | System, array, containers and VMs in one JSON document |
| `aurora://vms` | Virtual machines |
| `aurora://containers` | Docker containers |
| `aurora://array` | Array state and capacity |

## Prompts

| Prompt | Purpose |
|---|---|
| `host-health` | Review the array, disks, metrics, notifications and tasks |
| `container-triage` | Diagnose a container from its state and logs |
| `vm-triage` | Diagnose a virtual machine from its state and libvirt log |

## Security

- Stdio runs with the privileges of the Aurora binary. Protect the host account that launches it.
- HTTP requires TLS and a bearer token; the MCP endpoint is never public.
- Settings, secrets, diagnostics exports, terminal access, array stop/start and destructive storage
  operations are **not** exposed as MCP tools.
- Every lifecycle action is recorded in the Aurora audit log with the tool name and target.
- `AURORA_READ_ONLY=1` disables actions for both transports.
