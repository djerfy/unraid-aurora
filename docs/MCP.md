# MCP server

Aurora embeds a [Model Context Protocol](https://modelcontextprotocol.io) server so AI clients can read
the host and run a controlled set of lifecycle actions. The same tool catalog is served over two
transports: **stdio** for local clients and **Streamable HTTP** for remote clients.

The HTTP endpoint is **disabled by default**. Enable it under **Settings → API → MCP server**; the
local `aurora mcp` stdio mode is independent.

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

Enable **Settings → API → MCP server**, then the daemon exposes
`https://<server>:8484/api/v1/mcp` once authentication succeeds.

- Use an API token created in **Settings → API tokens**. An unscoped token exposes the full catalog;
  a scoped token only allows calls in its scopes.
- `viewer` tokens get the read tools only. `operator` and `admin` tokens can also run lifecycle
  actions, unless the server is in read-only mode or the token itself is read-only.
- Scope mapping: `docker` for containers and container actions, `vms` for VMs and VM actions,
  `storage` for array/disks/shares/pools/ZFS, `system` for host/metrics/tasks/notifications/GPUs,
  `ups` for the UPS and `apps` for plugins. Calls outside the token scopes return a tool error.
- Authenticate with `Authorization: Bearer <token>`. Browser cookie sessions require CSRF headers,
  so they are not usable from MCP clients.
- The endpoint inherits Aurora's TLS configuration, security headers and access log.

Clients without native Streamable HTTP support can use a local bridge such as `mcp-remote`.

## Hermes Agent (Nous Research)

[Hermes Agent](https://hermes-agent.nousresearch.com) reads MCP servers from `~/.hermes/config.yaml`
under `mcp_servers`. Aurora works over both transports.

### Remote HTTP (recommended)

1. Create a token in Aurora under **Settings → API tokens**. A `viewer` token is enough for read
   tools; add `operator` or `admin` if Hermes should start or stop workloads. Add the scopes you
   need (`docker`, `vms`, `storage`, `system`, `ups`, `apps`) or leave scopes empty for the full
   catalog.
2. Store the secret in `~/.hermes/.env`, never in `config.yaml`:

   ```bash
   AURORA_MCP_TOKEN=aurora_...
   ```

3. If Unraid uses its self-signed certificate, copy the bundle to the Hermes host and point
   `ssl_verify` at it:

   ```bash
   scp root@homeserver:/boot/config/ssl/certs/homeserver_unraid_bundle.pem \
     ~/.hermes/certs/homeserver.pem
   ```

4. Add the server to `~/.hermes/config.yaml`:

   ```yaml
   mcp_servers:
     aurora:
       url: "https://homeserver.unifi:8484/api/v1/mcp"
       headers:
         Authorization: "Bearer ${AURORA_MCP_TOKEN}"
       ssl_verify: "~/.hermes/certs/homeserver.pem"   # or true with a trusted CA
       connect_timeout: 30
       trust: "untrusted"                              # lifecycle actions ask for approval
       tools:
         include:
           - get_system
           - get_array
           - list_disks
           - list_shares
           - list_pools
           - list_containers
           - container_logs
           - list_vms
           - get_vm
           - vm_logs
           - list_zfs_pools
           - list_gpus
           - get_ups
           - get_metrics
           - get_metrics_history
           - list_tasks
           - list_notifications
           - list_plugins
         resources: true
         prompts: true
   ```

   The `include` list keeps Hermes read-only. To allow lifecycle actions, add the action tools you
   want (`start_container`, `stop_container`, `restart_container`, `start_vm`, `shutdown_vm`,
   `pause_vm`, `resume_vm`) or remove `include` entirely.

5. Check the connection, review the discovered tools, and reload a running session:

   ```bash
   hermes mcp test aurora
   hermes mcp configure aurora
   ```

   Inside a chat, `/reload-mcp` refreshes the server list after a config change.

6. Hermes prefixes the tools with the server name, for example `mcp__aurora__get_array` and
   `mcp__aurora__container_logs`.

### Local stdio

If Hermes runs on the Unraid host itself:

```yaml
mcp_servers:
  aurora:
    command: "/usr/local/bin/aurora"
    args: ["mcp"]            # add "--allow-actions" to enable lifecycle actions
    env:
      AURORA_READ_ONLY: "1"  # optional hard read-only guard
```

From another machine, tunnel stdio over SSH:

```yaml
mcp_servers:
  aurora:
    command: "ssh"
    args: ["-T", "root@homeserver", "/usr/local/bin/aurora", "mcp", "--allow-actions"]
```

`-T` avoids allocating a TTY. Aurora sends JSON-RPC on stdout and all logs to stderr, so the
transport stays clean.

### Hermes notes

- Read tools declare `readOnlyHint`, so `trust: untrusted` only prompts for write-capable actions.
- Scoped tokens only expose calls in their scopes; calls outside a scope return an explicit tool
  error instead of hiding the tool.
- `${AURORA_MCP_TOKEN}` is expanded from `~/.hermes/.env` (or the profile secret scope). Keep the
  secret out of `config.yaml`.
- Prefer the Unraid certificate bundle for `ssl_verify`; `ssl_verify: false` is acceptable only on a
  trusted lab network.
- `hermes mcp test aurora` prints the HTTP status and the start of the response body when a
  connection fails, which is usually enough to spot a token scope or TLS problem.

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

## Keeping MCP in sync

[`docs/mcp-coverage.json`](mcp-coverage.json) maps every route registered in
`backend/internal/api/server.go` to the MCP tools that expose it, or records why it is intentionally
excluded. `scripts/mcp_coverage.py` runs in CI and fails when:

- an API route is added or removed without updating the manifest;
- a manifest entry references a tool that no longer exists;
- an MCP tool is not mapped to any API route.

When you add a tool, implement it, add its name to the matching route entry, and update the catalog
above. When you add an API route, either expose it as a tool or add an exclusion reason.

```bash
python3 scripts/mcp_coverage.py
```

## Security

- Stdio runs with the privileges of the Aurora binary. Protect the host account that launches it.
- HTTP requires TLS and a bearer token; the MCP endpoint is never public.
- Settings, secrets, diagnostics exports, terminal access, array stop/start and destructive storage
  operations are **not** exposed as MCP tools.
- Every lifecycle action is recorded in the Aurora audit log with the tool name and target.
- `AURORA_READ_ONLY=1` disables actions for both transports.
