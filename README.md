<p align="center">
  <img src="docs/assets/unraid-logo-gradient.svg" alt="Unraid" width="260" />
</p>

<p align="center">
  <a href="https://github.com/djerfy/unraid-aurora/actions/workflows/release.yml"><img src="https://github.com/djerfy/unraid-aurora/actions/workflows/release.yml/badge.svg" alt="Release" /></a>
  <a href="https://github.com/djerfy/unraid-aurora/releases/latest"><img src="https://img.shields.io/github/v/release/djerfy/unraid-aurora?label=Version&color=ff6b4a" alt="Latest version" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-AGPL_v3-blue.svg" alt="License: AGPL v3" /></a>
</p>

# Aurora

**A modern, independent control plane for Unraid.**
Go on the server, React in the browser, native root authentication, and a design system built to make homelab
infrastructure feel calm.

> Aurora is not affiliated with Lime Technology. Unraid is a trademark of Lime Technology, Inc.
> This repository is a redevelopment study based on the public [unraid/webgui](https://github.com/unraid/webgui) source.

## Why

The Unraid webGUI is a PHP/jQuery application that has grown organically for two decades. It is reliable,
deeply integrated with `emhttpd`, and full of hard-earned operational knowledge — but it is also a 500 KB
JavaScript bundle, 134 server-rendered `.page` templates and a network of INI state files. Aurora keeps the
operational model and rebuilds the experience around it:

- **Typed Go API** with a small, explicit surface instead of page-scoped PHP includes.
- **React 19 + Tailwind v4** single-page app with live SSE telemetry, command palette and dark/light themes.
- **Native host authentication**: the `root` account is validated against the host shadow database, exactly
  like the original webGUI. There is no application password, no user database and no demo bypass.
- **A deliberate visual language**: one accent, precise typography, restrained motion, dense but readable data.
- **Bilingual interface**: English by default, with a full French translation selectable in Settings.
- **Visual chassis map**: drag 3D server/desktop/NAS panels, group drives into labelled sections, and keep a server-side, visual-only location plan.
- **Safe by default**: whitelisted actions, HMAC sessions, per-IP login throttling, CSRF header + origin
  checks, CSP and read-only mode.
- **Live host only**: Aurora reads the real Unraid state (`/var/local/emhttp`) and real system services. If it
  is not running on an Unraid host, it does not start.

## Screenshots

> Illustrative captures using anonymized sample data. No real server details are exposed.

| | |
|---|---|
| ![Dashboard](docs/screenshots/dashboard.png) | ![Storage](docs/screenshots/storage.png) |
| **Dashboard** — health, CPU, memory, array, network, workload | **Storage** — disks, I/O, capacity and parity |
| ![Containers](docs/screenshots/docker.png) | ![Virtual machines](docs/screenshots/vms.png) |
| **Containers** — live resources, ports and lifecycle | **Virtual machines** — KVM workloads and passthrough |
| ![Container security](docs/screenshots/docker-security.png) | ![Security by CVE](docs/screenshots/docker-security-cve.png) |
| **Container security** — merged Trivy findings, grouped by image | **CVE view** — affected images and containers |
| ![Network](docs/screenshots/network.png) | ![Notifications](docs/screenshots/notifications.png) |
| **Network** — interfaces, tunnels and services | **Notifications** — a calm operational inbox |
| ![Applications](docs/screenshots/apps.png) | ![Scheduler](docs/screenshots/scheduler.png) |
| **Applications** — Community Applications with a review step | **Scheduler** — parity, mover, TRIM and display formats |
| ![Settings](docs/screenshots/settings.png) | ![Login](docs/screenshots/login-native.png) |
| **Settings** — themes, accents and native auth | **Login** — root credentials checked against the host |
| ![Historique](docs/screenshots/history.png) | ![Équilibrage](docs/screenshots/balance.png) |
| **Historique** — configuration snapshots, unified diffs and rollback | **Équilibrage** — empty a disk with live per-file progress |
| ![Tools](docs/screenshots/tools.png) | ![Mover](docs/screenshots/storage-mover.png) |
| **Tools** — diagnostics archive and host summary | **Mover** — live progress, throughput and per-file history |

More captures in [`docs/screenshots`](docs/screenshots): shares, pools, parity history, container logs,
command palette, light theme, users, plugins, logs and mobile layouts.

## Safe operations

### Configuration history and rollback

Every configuration file written by Aurora is snapshotted in
`$AURORA_CONFIG_DIR/config-history/<id>/` before being modified: container templates, the Docker
autostart list and future file-backed settings. The **Historique** page lists each change with:

- the action and a human-readable description;
- the affected files and their previous size;
- a line-by-line **Différence** view (context included);
- a **Rollback** button that restores the previous contents. The current state is snapshotted first,
  so a rollback can itself be reverted.

History is capped at 200 entries and survives restarts.

### Disk rebalancing

The **Équilibrage** page empties a disk or moves selected folders/files to other disks:

- source and destination disks are detected from the mounted `/mnt` entries;
- a folder/file browser lets you choose a subset, or empty the whole disk;
- **dry-run** is enabled by default; nothing is written or deleted in that mode;
- files are copied, size-verified, then the source is deleted (never the other way around);
- symlinks and hardlinks are preserved, ownership/mode/timestamps are carried over;
- jobs run in the background with live phases (scanning, copying, cleaning), bytes, files, speed,
  ETA, errors and a per-file status list;
- the last 20 jobs are persisted in `$AURORA_CONFIG_DIR/balance-history.json`.

### Live mover

The mover card now exposes its real phases (searching, calculating, moving, finishing), the current
share/file, throughput and ETA. Each run records the observed moved files, and the history appears
live as soon as the mover exits.

## Documentation

Full page-by-page guide with screenshots: [`docs/USER_GUIDE.md`](docs/USER_GUIDE.md).

## Requirements

- Unraid 7.x (the state files and `/etc/shadow` must be readable).
- A TLS certificate bundle at `/boot/config/ssl/certs/<server>_unraid_bundle.pem`, where `<server>` is the Unraid server name (standard self-signed cert).
- Go 1.27+ and Node 22+ to build.
- Root privileges at runtime: shadow access, Docker socket, `emcmd`, `mdcmd`, `virsh`.
- Optionally, a TLS reverse proxy if the UI must be exposed under a public hostname.

Aurora listens on `:8484` by default and serves **HTTPS only**. The certificate is resolved from the Unraid
server name (`NAME` in `var.ini`, then the host name) as
`/boot/config/ssl/certs/<server>_unraid_bundle.pem`. It is a separate control plane; it does not replace the
stock webGUI on port 80/443.

## Quick start (on Unraid)

```bash
git clone https://github.com/djerfy/unraid-aurora.git
cd unraid-aurora
make install      # front-end dependencies
make build        # build the SPA, embed it, build the Go binary
sudo ./bin/aurora
```

Open **`https://<server>:8484`** and sign in with the **root password** of the Unraid host. The certificate
is self-signed, so the browser will ask you to accept it once.

### Development

```bash
make test         # Go unit + HTTP integration tests (run anywhere)
make vet
make dev-ui       # Vite on :5173, proxies /api to :8484
make dev-api      # requires Unraid state + the TLS certificate bundle
make build-tmp    # amd64 binary for Unraid → /tmp/aurora-linux-amd64
```

The backend deliberately has no fixture or demo mode. UI-only work can use the API contract and the typed
models; integration tests use an in-memory provider double confined to `_test.go` files.

## Releases

A push to a `v*` tag builds the embedded UI and publishes a single **linux/amd64** binary with
GoReleaser:

```bash
git tag v0.0.4
git push origin v0.0.4
```

GitHub Actions owns publication for tagged releases: push the tag and let the workflow finish instead
of running `goreleaser release` locally, otherwise both publishers race for the same assets. The
configuration tolerates an existing release, but a single publisher keeps the run history clean.

The workflow can also be started manually (`workflow_dispatch`); it then produces a snapshot build and
uploads `aurora_*_linux_amd64` plus `checksums.txt` as a workflow artifact. The local equivalent is:

```bash
make build-tmp      # /tmp/aurora-linux-amd64
```

## Authentication

Authentication is **native to the operating system**:

1. The only accepted username is `root`, matching the Unraid webGUI.
2. The shadow entry is read through `getent shadow root`, with a direct `/etc/shadow` fallback.
3. The password is verified locally against the crypt(3) hash (`$6$` SHA-512, `$5$` SHA-256 or `$1$` MD5).
4. Locked accounts, missing entries and expired accounts are rejected before verification.
5. Three failed attempts from one address trigger a 15 minute cooldown, like Unraid.
6. A successful login issues an HMAC-SHA256 signed, `HttpOnly`, `SameSite=Lax`, `Secure` cookie (24 h by default).
7. Passwords are never stored, logged or sent anywhere.

Unraid Connect SSO tokens are **not** implemented. Aurora is intentionally limited to local root credentials.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `AURORA_ADDR` | `:8484` | Listen address |
| `AURORA_STATE_DIR` | `/var/local/emhttp` | Unraid state directory (must contain `var.ini`) |
| `AURORA_BOOT_DIR` | `/boot` | Flash/boot directory |
| `AURORA_CONFIG_DIR` | `<AURORA_BOOT_DIR>/config/aurora` | Durable JSON config/history (settings, layout, history) |
| `AURORA_TRIVY_DIR` | `<AURORA_CONFIG_DIR>/trivy` | Persistent Trivy binary, vulnerability and Java DB cache, and scan results |
| `AURORA_TRIVY_RUNTIME_DIR` | `/usr/local/aurora/trivy` then `/tmp/aurora-trivy` | Writable, executable directory used to run the persistent Trivy binary (the flash mount is often noexec) |
| `AURORA_DOCKER_SOCKET` | `/var/run/docker.sock` | Docker Engine socket |
| `AURORA_DATA_DIR` | `/var/lib/aurora` | Runtime cache and mover output (tmpfs on Unraid) |
| `AURORA_TLS_CERT` | derived | Explicit PEM bundle path (default: `/boot/config/ssl/certs/<server>_unraid_bundle.pem`) |
| `AURORA_TLS_CERT_DIR` | `/boot/config/ssl/certs` | Directory used for certificate lookup |
| `AURORA_TLS_KEY` | same as the bundle | Optional separate private key path |
| `AURORA_SERVER_NAME` | `var.ini` / hostname | Override the `<server>` part of the certificate filename |
| `AURORA_SESSION_TTL` | `24h` | Session lifetime (`time.ParseDuration`) |
| `AURORA_AUDIT_LIMIT` | `5000` | In-memory audit entries retained (100–50000) |
| `AURORA_READ_ONLY` | `false` | Disable every mutation |
| `AURORA_LOG_LEVEL` | `info` | `debug` for verbose logs |
| `AURORA_LOG_FILE` | stdout | Append logs to this file |
| `AURORA_LOG_FORMAT` | `text` | `text` or `json` |
| `AURORA_METRICS_TOKEN` | empty | Dedicated bearer token for the Prometheus `/metrics` endpoint. Scoped API tokens also work; with no dedicated token, only a valid API token is accepted |
| `AURORA_DAEMON` | `false` | Detach and run in the background |

Flags override the environment: `-addr`, `-log-file`, `-log-format`, `-daemon`, `-version`.
`make build` injects the latest reachable Git tag, and `/api/v1/health` exposes the same version to
the About page.

Durable configuration and history are stored under `AURORA_CONFIG_DIR`, on the Unraid flash by
default, so they survive a reboot. `AURORA_DATA_DIR` is intentionally tmpfs: it only holds runtime
caches and the mover output log, which can be regenerated.

```bash
# Print the build version
./aurora -version

# Foreground, JSON logs in a file
./aurora -log-format json -log-file /var/log/aurora.log

# Detached; defaults to /var/log/aurora.log
./aurora -daemon

# Detached with explicit JSON logging
./aurora -daemon -log-format json -log-file /var/log/aurora.log
```

A daemon writes its standard streams to the log file (or `/dev/null` when logging is disabled)
and prints the child PID before exiting.

## Data sources

| Data | Source |
|---|---|
| System, array, disks, shares | `/var/local/emhttp/{var,disks,shares}.ini`, `/proc`, `/sys` |
| Docker | `/var/run/docker.sock` (`/containers/json`, `/containers/{id}/logs`) |
| Virtual machines | `virsh list --all`, `virsh domstate` |
| Notifications | `/tmp/notifications/{unread,archive}/*.notify` |
| Parity history | `/boot/config/parity-checks.log` |
| UPS | `apcaccess status` (built-in) or `upsc` (NUT plugin) |
| Logs | `/var/log/syslog`, `/var/log/docker.log`, `/var/log/nginx/*.log`, `/var/log/libvirt/libvirtd.log`, `/var/log/maillog` |
| TLS | `/boot/config/ssl/certs/<server>_unraid_bundle.pem` |
| Icons | Docker icon cache, `templates-user` XML, `dynamix.vm.manager/templates/images` |
| Settings | `$AURORA_CONFIG_DIR/settings.json` (persistent on flash) |
| Credentials | `getent shadow root`, `/etc/shadow` |

Actions use fixed binaries and validated targets: `emcmd`, `mdcmd`, `docker`, `virsh` and the stock
notification folders. They are a convenience layer, not a replacement for `emhttpd` or the Unraid API.

## Architecture

```
Browser ── React 19 SPA (embedded) ── /api/v1/* ── Go net/http
                                               │
                                      provider.Live ── /var/local/emhttp, procfs, sysfs,
                                               │        Docker socket, libvirt, apcupsd/NUT
                                               │
                                      metrics.Hub (SSE, 2 s)
```

- **No framework**: the Go server uses the standard library (`net/http` routing, `embed`, `slog`).
- **One binary**: `frontend/dist` is embedded; there is nothing else to deploy.
- **HTTPS only**: TLS is terminated in-process with Unraid's `<server>_unraid_bundle.pem`.
- **SSE, not polling**: `/api/v1/events` streams metrics every two seconds; React Query refreshes the
  coarse snapshot every twelve seconds and after every mutation.
- **Typed contract**: `internal/model` is the single source of truth, mirrored by
  `frontend/src/lib/types.ts`.
- **Read-only first**: live mutations call fixed binaries with validated targets, and `AURORA_READ_ONLY=1`
  disables every write path.

Deep dives:

- [`docs/ANALYSIS.md`](docs/ANALYSIS.md) — architecture, state model, nchan, plugins, security, UX audit.
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — Go packages, authentication, live mapping.
- [`docs/API.md`](docs/API.md) — HTTP API reference: authentication, token roles, endpoints and examples.
  The live host also serves a rendered Redoc reference at `/api/docs` and the OpenAPI document at `/api/openapi.json`.
- [`docs/DESIGN.md`](docs/DESIGN.md) — visual system, tokens, components, accessibility and motion.

## Feature map

| Area | Aurora |
|---|---|
| Dashboard | Health score, CPU load graph and per-core matrix, package power, memory, GPU, array capacity, parity progress, network, UPS, containers, shares, activity |
| Storage | Array devices with live I/O and spin controls, background-cached share usage, cache pools, parity check + history |
| Containers | Docker workloads with real icons, CPU/memory, health status, ports, network, IP address, network calls, a merged Trivy security overview grouped by image/container/CVE/package/severity, container terminal, log search/export, bulk start/stop/restart, autostart toggle, image update checks, live pull/recreate progress, Trivy image scanning with a Security column and finding details, create from an Unraid XML template, advanced devices/capabilities/sysctls/ulimits/resources/MAC fields, remove, lifecycle, logs and a settings editor for paths, ports, environment and restart policy |
| Applications | Community Applications catalog for Docker apps and plugins, search/filter/sort, installed/update badges, dependency hints, template review before creation, and plugin installation through the official plugin manager |
| Scheduler | Parity, mover and SSD TRIM schedules with generated cron preview, `update_cron` activation, plus shared date/time display formats |
| Virtual machines | KVM inventory, vCPU/RAM/disk usage, GPU passthrough, VM creation, editable libvirt XML, guided hardware editor (CPU pinning, network/MAC, sound, virtiofs shared folders, PCI/USB passthrough with IOMMU groups), live USB passthrough, clone, vDisk expansion, snapshots, autostart toggle, lifecycle actions and a VNC/SPICE console with clipboard paste |
| Network | Interfaces, bridges, WireGuard peers, listening services, traffic chart, and guarded per-interface editing (DHCP/static, DNS, MTU, bonding, bridging) with live state, link up/down, backups, revert and delayed apply |
| GPUs | NVIDIA and AMD (ROCm) utilisation, VRAM, temperature, power, clocks, vendor logo and compute processes |
| ZFS | Pools, datasets with compression/quota/recordsize, scrub and snapshot create/rollback/clone |
| UPS | Battery telemetry, apcupsd shutdown policy and NUT connection/threshold settings |
| User scripts | User Scripts plugin inventory, source editing, root execution output and plugin-native schedules (hourly/daily/weekly/monthly, array start/stop, first boot, custom cron) |
| Audit log | Durable record of every mutating API call, filterable by method/status/user, exportable as CSV, with configurable retention |
| About | Product description, author, repository, runtime and AGPL-3.0 license |
| Users | Accounts, roles, share access matrix, add/edit/password/delete through Unraid account management |
| Notifications | Inbox, unread/archived filters, acknowledge all, severity and source, minimum-importance, muted-source and event-category filters |
| Plugins | Real .plg metadata, update checks, streamed update/install/remove through the official plugin manager |
| Logs | Syslog, Docker, Nginx, Libvirt, Maillog and parity tails with level + text filters, follow mode and export |
| Tools | Diagnostics ZIP (system, Docker, libvirt, configs, logs) with credentials redacted, redacted configuration export, host summary |
| Configuration history | `$AURORA_CONFIG_DIR/config-history` (backup files, metas, unified diffs) |
| Balance | Mounts under `/mnt`, `statfs`, `du`; copy → verify → delete pipeline |
| Terminal | Full-page root shell over an authenticated WebSocket, openable in its own window |
| Storage operations | Start/stop array with passphrase or keyfile, SMART health/attributes and short self-test, background mover with live phases and per-file history |
| Shares | Create, rename and edit shares: comment, free-space floor, cache mode, primary/secondary storage, allocator, split level, include/exclude disks, SMB and NFS export/security |
| Équilibrage | Empty a disk or move folders/files to other disks with dry-run, live progress, per-file status and history |
| Historique | Automatic snapshots of every configuration file written by Aurora, unified diffs, one-click rollback |
| Settings | Dark/light/system, five accents, density, units, refresh cadence, session info, notification channels, UPS policy, API tokens, Unraid OS updates/downgrade, Unraid API process management and Trivy installation/database scheduling |
| Global | ⌘K command palette with full-text search over pages, containers and VMs, keyboard navigation, toasts, responsive layout, light theme |

## Security notes

- Aurora serves HTTPS directly and has **no plaintext HTTP listener**; `upgrade-insecure-requests` is sent
  on every response. HSTS is intentionally not enabled because the certificate is self-signed and may be
  regenerated by Unraid.
- The default certificate is Unraid's self-signed bundle at
  `/boot/config/ssl/certs/<server>_unraid_bundle.pem`; override it with `AURORA_TLS_CERT` / `AURORA_TLS_KEY`.
  If the exact name is missing but exactly one `*_unraid_bundle.pem` exists, that bundle is used.
- Keep the default `:8484` binding or bind to loopback and reverse-proxy if you need a public hostname.
- The login throttle is in memory: restarting the process clears the cooldown, as intended for a local
  appliance. The account itself is never locked.
- `X-Forwarded-For` is only trusted when the immediate peer is loopback (a local reverse proxy).
- Aurora needs root privileges for the same reasons the stock webGUI does. Review the action whitelist before
  exposing mutations; run with `AURORA_READ_ONLY=1` to make the UI observational only.

## Scope

Aurora focuses on day-to-day observability and lifecycle operations. It deliberately does **not**
implement Unraid Connect SSO or the full Unraid GraphQL API. Those remain in Unraid itself.

## Project layout

```
frontend/                  React 19 · TypeScript · Vite 8 · Tailwind v4
  src/app/                 theme, query/session state, toasts
  src/components/          design system, layout, charts, overlays
  src/pages/               one file per route
  src/lib/                 API client, types, formatting, hooks
backend/
  cmd/aurorad/             entry point
  internal/api/            routing, middleware, handlers, SSE
  internal/auth/           shadow authentication + signed sessions
  internal/config/         environment configuration
  internal/metrics/        sampling hub
  internal/model/          shared transport types
  internal/provider/       live Unraid provider, INI parsing
  internal/trivy/          managed Trivy binary, database and image scans
  internal/webui/          embedded SPA
docs/                      analysis, architecture, design
```

## Tests

```bash
make test                         # Go unit + HTTP integration tests
cd frontend && npm run build      # TypeScript + production bundle
```

The authentication tests verify SHA-512, SHA-256 and MD5 shadow hashes, locked/expired accounts, rate limiting,
session expiry and tampering. The live provider tests build a synthetic Unraid state tree in a temporary
directory; they are not a runtime mode.

`.github/workflows/ci.yml` runs `go vet` and the Go test suite on every push (any branch or tag) and on
pull requests, so a regression is reported before a release workflow starts.

## Credits

- [Unraid webGUI](https://github.com/unraid/webgui) — the original interface and the state contracts Aurora reads.
- [Unraid documentation](https://docs.unraid.net) — array, parity and Docker behaviour.
- [Lucide](https://lucide.dev) icons, [Inter](https://rsms.me/inter/) and [JetBrains Mono](https://www.jetbrains.com/lp/mono/).
- [GehirnInc/crypt](https://github.com/GehirnInc/crypt) — pure-Go crypt(3) verification for shadow hashes.

## License

Aurora is free software released under the **GNU Affero General Public License v3.0**. See
[`LICENSE`](LICENSE) for the full text.

Unraid and the Unraid logo are trademarks of Lime Technology, Inc. Aurora is an independent project, not
affiliated with or endorsed by Lime Technology. The original Unraid webGUI is distributed under GPL-2.0; its
state formats and public documentation were used as a compatibility reference.
