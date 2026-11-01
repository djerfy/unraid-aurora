# Aurora — architecture

## 1. Goals

1. **Read the real Unraid host** without reimplementing `emhttpd`.
2. **One binary**: no runtime dependency beyond the host it inspects.
3. **A typed contract** between providers, HTTP and the UI.
4. **Live only**: there is no simulated provider or authentication bypass.
5. **A small mutation surface** that cannot be turned into arbitrary command execution.

Non-goals: replacing the array engine, running plugin code, VM consoles, flash operations.

## 2. Process model

```
aurorad
  ├── config.Load()                 environment + flags
  ├── provider.NewLive()             Unraid state + host services
  ├── metrics.NewHub(provider)
  │     └── goroutine: sample every 2 s → fan-out to SSE subscribers
  ├── api.New(...)                  middleware + routes
  └── http.Server (TLS 1.2+)
        ├── /api/v1/*               JSON + SSE
        └── /*                      embedded React SPA
```

Graceful shutdown listens for `SIGINT`/`SIGTERM` and calls `Shutdown` with a 10 s deadline. The SSE hub
stops when the root context is cancelled.

## 3. Packages

| Package | Responsibility |
|---|---|
| `cmd/aurorad` | flags, logging, provider, HTTP server lifecycle |
| `internal/config` | env/flags parsing and validation |
| `internal/model` | JSON transport types, no behaviour |
| `internal/provider` | `Provider` interface, live Unraid adapter, INI parsing |
| `internal/metrics` | 2 s sampling loop, ring history, subscriber fan-out |
| `internal/auth` | native root/shadow authentication, HMAC sessions, throttling |
| `internal/api` | routing, middleware, handlers, SSE, validation |
| `internal/webui` | `go:embed` of the built SPA and SPA fallback |

Dependencies point inward: `api` → `provider`/`metrics`/`auth`/`model`; `provider` → `model`; `model` imports
nothing from the app. There are no import cycles. The only third-party dependency is a small pure-Go
crypt(3) implementation used to verify shadow hashes.

## 4. Provider contract

```go
type Provider interface {
    Name() string
    Snapshot(ctx) (model.Snapshot, error)
    System/Array/Disks/Shares/Pools/Containers/VMs/Network/Users/Plugins/
    Notifications/ParityHistory/UPS/Logs/Settings/Metrics/History(...)
    ContainerLogs(ctx, id, lines) ([]model.LogLine, error)
    SaveSettings(ctx, model.Settings) error
    Perform(ctx, model.Action) error
    Close() error
}
```

`Snapshot` is the hydration payload for the SPA. `Metrics` is the cheap 2 s sample. The split keeps the hot
path independent from Docker/VM enumeration.

### Native authentication

Aurora authenticates exactly like the Unraid webGUI at the credential layer:

1. only the username `root` is accepted;
2. the hash comes from `getent shadow root`, with `/etc/shadow` as a fallback;
3. locked and expired accounts are rejected;
4. the password is verified locally with crypt(3) — `$6$` SHA-512, `$5$` SHA-256 and `$1$` MD5;
5. three failures per address trigger a 15 minute cooldown.

Passwords never enter the session store and are never written to logs. Unraid Connect SSO tokens are not
implemented; Aurora is deliberately limited to local root credentials.

### Live provider

Reads the same sources as the original webGUI:

| Source | Implementation |
|---|---|
| `var.ini`, `disks.ini`, `shares.ini` | `parseINI` (sections, quotes, no interpolation) |
| CPU / memory / load / uptime | `/proc/stat`, `/proc/meminfo`, `/proc/loadavg`, `/proc/uptime` |
| Per-core usage | `/proc/stat` `cpuN` lines |
| Network | `net.Interfaces`, `/proc/net/dev`, `/sys/class/net/*/speed` |
| Disk throughput | `/proc/diskstats` deltas over a 2 s window |
| Temperatures | `/sys/class/hwmon/hwmon*/temp1_input` |
| Docker | HTTP over `unix:/var/run/docker.sock`; image updates use registry v2 manifest digests, with credentials from `~/.docker/config.json` |
| VM inventory | `virsh list --all`, `virsh domstate`, `virsh dumpxml` (graphics websocket for the console) |
| Notifications | `/tmp/notifications/{unread,archive}/*.notify` |
| Parity history | `/boot/config/parity-checks.log` |
| UPS | `apcaccess status` |
| Logs | tail of `/var/log/syslog` (`docker.log` for Docker) |
| Container icons | `/var/lib/docker/unraid/images`, `/usr/local/emhttp/.../docker.manager/images`, user template XML |
| User icons | `/boot/config/plugins/dynamix/users/<name>.png` |
| GPUs | `nvidia-smi` (NVIDIA) or `rocm-smi` (AMD): utilisation, VRAM, temperature, power, processes |
| Mover | `/var/run/mover.pid`, `/var/local/emhttp/mover.ini`, run history in `$AURORA_DATA_DIR/mover-history.json` |
| VM icons | domain metadata icon, `/usr/local/emhttp/.../vm.manager/templates/images` |
| Settings | `AURORA_DATA_DIR/settings.json` |
| Image update status | `AURORA_DATA_DIR/image-updates.json` (last check + per-image result) |
| Container templates | `/boot/config/plugins/dockerMan/templates-user/my-<name>.xml`, read and written like the official webGUI |

Every read is best-effort. A missing file or binary yields zero values, never a failed snapshot. The only
hard failure is a host without `/var/local/emhttp/var.ini`: Aurora refuses to start rather than serve empty
fake data. Snapshots are normalised so JSON never contains `null` where the UI expects an array.

## 5. HTTP surface

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/health` | Liveness, version, provider |
| `GET` | `/api/v1/auth/session` | Session state (public) |
| `POST` | `/api/v1/auth/login` | Credentials → HttpOnly cookie |
| `POST` | `/api/v1/auth/logout` | Clear cookie |
| `GET` | `/api/v1/bootstrap` | Full snapshot + hub history |
| `GET` | `/api/v1/system` | System identity, CPU, memory, sensors |
| `GET` | `/api/v1/array` | Array + parity state |
| `GET` | `/api/v1/disks` | Devices with live I/O |
| `GET` | `/api/v1/disks/{device}/smart` | SMART health and attributes |
| `POST` | `/api/v1/disks/{device}/smart-test` | Start a short self-test |
| `GET` | `/api/v1/shares` | Shares and protocols |
| `GET` | `/api/v1/pools` | Cache/boot pools |
| `GET` | `/api/v1/docker/containers` | Containers |
| `GET` | `/api/v1/docker/containers/{id}/logs?lines=` | Container log tail |
| `GET` | `/api/v1/docker/containers/{id}/icon` | Cached Unraid container icon |
| `GET` | `/api/v1/docker/containers/{id}/config` | Editable container configuration |
| `PUT` | `/api/v1/docker/containers/{id}/config` | Recreate the container with new paths/ports/env |
| `PUT` | `/api/v1/docker/containers/{id}/autostart` | Toggle the Unraid autostart list |
| `POST` | `/api/v1/docker/containers` | Create and start a container, pulling the image if needed |
| `DELETE` | `/api/v1/docker/containers/{id}` | Remove a container (force stop first) |
| `GET` | `/api/v1/docker/networks` | Docker networks for the create form |
| `GET` | `/api/v1/docker/templates` | Unraid XML templates available for recreation |
| `POST` | `/api/v1/docker/check-updates` | Compare local and registry image digests |
| `POST` | `/api/v1/docker/containers/{id}/update` | Stream pull/recreate stages as NDJSON |
| `GET` | `/api/v1/vms` | Virtual machines |
| `GET` | `/api/v1/vms/{id}/icon` | Unraid OS icon for the VM |
| `GET` | `/api/v1/vms/{id}/console` | WebSocket VNC/SPICE proxy to the libvirt console |
| `PUT` | `/api/v1/vms/{id}/autostart` | Toggle libvirt autostart |
| `GET` | `/api/v1/vms/{id}/xml` | Read-only domain XML |
| `GET` | `/api/v1/plugins` | Installed plugin metadata |
| `POST` | `/api/v1/plugins/check-updates` | Compare local and remote .plg versions |
| `POST` | `/api/v1/plugins/{name}/update` | Stream the official plugin manager output |
| `GET` | `/api/v1/tools/diagnostics` | Download a diagnostics ZIP |
| `GET` | `/api/v1/network` | Interfaces, WireGuard, services |
| `GET` | `/api/v1/users` | Accounts and access |
| `GET` | `/api/v1/plugins` | Installed plugins |
| `GET` | `/api/v1/notifications` | Inbox + archive |
| `GET` | `/api/v1/parity/history` | Historical checks |
| `GET` | `/api/v1/ups` | UPS state |
| `GET` | `/api/v1/gpus` | NVIDIA telemetry |
| `GET` | `/api/v1/mover` | Mover status and progress |
| `POST` | `/api/v1/mover/start` / `stop` | Background mover control |
| `POST` | `/api/v1/array/start` / `stop` | Array lifecycle with optional LUKS key |
| `GET` | `/api/v1/users/{name}/icon` | Unraid user avatar |
| `GET` | `/api/v1/terminal` | Authenticated WebSocket root shell |
| `GET` | `/api/v1/logs?source=&lines=` | syslog, docker or parity |
| `GET` | `/api/v1/settings` | UI settings |
| `PUT` | `/api/v1/settings` | Persist validated settings |
| `GET` | `/api/v1/metrics/history` | Hub ring buffer |
| `GET` | `/api/v1/events` | SSE: `hello`, `metrics`, heartbeats |
| `POST` | `/api/v1/actions` | Enumerated mutation |

Errors are consistent:

```json
{ "error": { "code": "invalid_action", "message": "target is required for container.start" } }
```

## 6. Authentication and request safety

### Session

- Cookie `aurora_session`, `HttpOnly`, `SameSite=Lax`, `Secure` (Aurora has no plaintext listener).
- Value = base64url(JSON `{u,exp,n}`) + "." + base64url(HMAC-SHA256).
- Secret generated with `crypto/rand` at process start. Restarting invalidates sessions; that is deliberate
  for a local control plane (no secret at rest).
- TTL from `AURORA_SESSION_TTL` (default 24 h).
- Password verification happens in `auth.Manager.Login` against the host shadow hash; only `root` is accepted.
- Three failures per IP open a 15 minute cooldown, matching the stock webGUI.
- Locked (`!`, `*`), missing and expired accounts are rejected before hashing.
- Session tokens are signed with HMAC-SHA256 and expire server-side.

### CSRF

Every mutating `/api/` request must carry `X-Aurora-Request: aurora`; when an `Origin` header is present its
host must match the request host. This blocks form-based cross-site posts even when cookies are sent.

### Headers

```
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Referrer-Policy: no-referrer
Permissions-Policy: camera=(), microphone=(), geolocation=()
Content-Security-Policy: default-src 'self'; img-src 'self' data:;
  style-src 'self' 'unsafe-inline'; font-src 'self' data:; connect-src 'self';
  frame-ancestors 'none'; base-uri 'self'; form-action 'self'
```

The only inline style usage is CSS variables and React `style` props (`style-src` allows it); scripts are
external or module chunks. TLS is terminated by the Go process with the Unraid `<server>_unraid_bundle.pem`; there is no HTTP listener.

### Actions

`validateAction` rejects unknown kinds, missing targets, targets longer than 128 characters and control
characters. The live provider additionally rejects shell metacharacters and executes fixed binaries with
argument arrays (no shell). `AURORA_READ_ONLY=1` rejects every mutation before validation.

| Kind | Live command |
|---|---|
| `array.start` / `array.stop` | `emcmd cmdStart=Start` / `cmdStop=Stop` |
| `disk.spinup` / `disk.spindown` | `mdcmd spinup\|spindown <device>` |
| `container.start/stop/restart` | `docker start\|stop\|restart <name>` |
| `container.update` | pull the image, then stop → rename → create → start, with rollback; steps streamed to the UI |
| container config `PUT` | Docker inspect → stop → rename → create → start, with rollback; the Unraid XML template is synced afterwards |
| `vm.start/stop/pause/resume` | `virsh start\|shutdown\|suspend\|resume` |
| `parity.start/stop/pause/resume` | `emcmd cmdCheck=...` |
| `notification.ack/archive/ackall` | move `.notify` files from unread to archive |
| mover start/stop | detached `/usr/local/sbin/mover start` and `mover stop` |
| array start/stop | `emcmd`, with base64 `luksKey` or a host keyfile |

## 7. Realtime

`metrics.Hub`:

- samples the provider every 2 s (first sample immediately);
- appends to a 1,800-point ring (one hour), capped;
- broadcasts to subscriber channels with a non-blocking send (slow clients skip frames);
- seeds history from `provider.History()` at startup, so the first render has a chart.

SSE handler:

```
retry: 3000

event: hello
data: {"mode":"live","version":"0.1.0"}

event: metrics
data: {"t":...,"cpu":...,"memory":...,"netRx":...}

: ping        (every 15 s)
```

The React `AppDataProvider` subscribes once, merges points by timestamp, and keeps 900 points. SSE was
chosen over WebSocket because the stream is one-way, works through proxies, reconnects automatically, and
needs no extra Go dependency.

## 8. React application

```
main.tsx
  QueryClientProvider
    ThemeProvider            localStorage theme/accent/density, system listener
      ToastProvider          max 3 stacked toasts, auto-dismiss
        AppDataProvider      session query, bootstrap query, SSE, history
          BrowserRouter
            App              routes + document title
```

- **Server state** lives in TanStack Query (`bootstrap` every 12 s, logs every 6 s, container logs on demand).
- **Theme state** is local and persisted; the Settings page mirrors it to the server.
- **Metrics** are local state fed by SSE, merged with the bootstrap history.
- **Actions** go through `useAction`, which invalidates `bootstrap` and shows a toast on success/failure.
- **Routing** is explicit in `App.tsx`; the classic filesystem routing is gone.
- **Command palette** is generated from navigation, global actions, containers and VMs.

### Route table

| Route | Page |
|---|---|
| `/login` | Split-screen sign-in |
| `/` | Dashboard bento |
| `/storage` | Array / shares / pools / parity tabs |
| `/docker` | Container grid/list + logs drawer |
| `/vms` | VM cards + hypervisor summary |
| `/network` | Traffic, interfaces, peers, services |
| `/gpus` | GPU telemetry |
| `/terminal` | Live root shell |
| `/users` | Accounts + access matrix |
| `/notifications` | Inbox with filters and drawer |
| `/plugins` | Installed extensions |
| `/logs` | Terminal-style log tail |
| `/settings` | Appearance, general, notifications, security, about |
| `*` | 404 |

## 9. Build pipeline

```bash
npm run build            # tsc -b && vite build → frontend/dist
cp -R frontend/dist backend/internal/webui/dist
go build ./cmd/aurorad   # //go:embed all:dist
```

`webui.Handler` serves existing files with immutable caching for `/assets/*`, returns 404 for missing files
with an extension, and falls back to `index.html` for client routes. The SPA shell is never cached.

The UI can be developed separately: Vite proxies `/api` to `:8484`, including the SSE stream and cookies.

## 10. Failure behaviour

| Failure | Behaviour |
|---|---|
| Invalid env | exit code 2 with a precise message |
| Missing `/var/local/emhttp/var.ini` | exit 1 with an explicit "not an Unraid host" message |
| Missing or unreadable TLS bundle | exit 1 with the certificate path and a generation hint |
| Shadow entry unreadable | login rejected; the UI never reveals which step failed |
| Docker socket unavailable | empty container list, error logged by handler |
| `virsh` absent | empty VM list |
| SSE client too slow | frame dropped, connection kept |
| Provider handler error | 500 `{"error":{"code":"internal"}}` + server log |
| Panic in handler | recovered, 500, stack-free log entry |
| Lost SSE | browser retries every 3 s; React Query keeps the coarse snapshot fresh |

## 11. Performance

- Binary ~11 MB with embedded UI; RSS dominated by Go runtime.
- Startup < 150 ms with the embedded UI.
- SSE payload ~200 bytes every 2 s per client.
- Dashboard renders ~10 charts; each chart downsamples to at most ~180 points and uses one SVG path per
  series.
- `AreaChart` measures its container with `ResizeObserver` and only recomputes paths when data or width change.
- No charting library: the whole graph layer is ~400 lines and ships in the main chunk.

## 12. Extension points

- Add a provider method + model type → expose it through `api` and a query hook.
- Add a page → add a route in `App.tsx`, a nav entry in `nav.ts`, and a page component.
- Add an action → add the kind constant, validation rule, provider implementation, and UI call.
- Add a theme accent → add a `[data-accent]` block in `globals.css` and an entry in `Settings.tsx`.

## 13. Deployment sketch

```ini
# /etc/systemd/system/aurora.service
[Unit]
Description=Aurora control plane
After=network-online.target docker.service

[Service]
ExecStart=/usr/local/bin/aurora -addr :8484
Environment=AURORA_STATE_DIR=/var/local/emhttp
# Certificate defaults to /boot/config/ssl/certs/<server>_unraid_bundle.pem
Environment=AURORA_TLS_CERT_DIR=/boot/config/ssl/certs
EnvironmentFile=/etc/aurora.env
Restart=on-failure
User=root

[Install]
WantedBy=multi-user.target
```

Aurora needs root for shadow access, the Docker socket, `emcmd`, `mdcmd` and `virsh`. It serves HTTPS
directly with Unraid's self-signed `<server>_unraid_bundle.pem` and has no plaintext HTTP listener; the root
login is the only authentication boundary.
