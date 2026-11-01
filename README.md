<p align="center">
  <img src="docs/assets/unraid-logo-gradient.svg" alt="Unraid" width="260" />
</p>

<p align="center">
  <a href="https://github.com/djerfy/unraid-aurora/actions/workflows/release.yml"><img src="https://github.com/djerfy/unraid-aurora/actions/workflows/release.yml/badge.svg" alt="Release" /></a>
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
| ![Network](docs/screenshots/network.png) | ![Notifications](docs/screenshots/notifications.png) |
| **Network** — interfaces, tunnels and services | **Notifications** — a calm operational inbox |
| ![Settings](docs/screenshots/settings.png) | ![Login](docs/screenshots/login-native.png) |
| **Settings** — themes, accents and native auth | **Login** — root credentials checked against the host |

More captures in [`docs/screenshots`](docs/screenshots): shares, pools, parity history, container logs,
command palette, light theme, users, plugins, logs and mobile layouts.

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
| `AURORA_DOCKER_SOCKET` | `/var/run/docker.sock` | Docker Engine socket |
| `AURORA_DATA_DIR` | `/var/lib/aurora` | Persisted UI settings |
| `AURORA_TLS_CERT` | derived | Explicit PEM bundle path (default: `/boot/config/ssl/certs/<server>_unraid_bundle.pem`) |
| `AURORA_TLS_CERT_DIR` | `/boot/config/ssl/certs` | Directory used for certificate lookup |
| `AURORA_TLS_KEY` | same as the bundle | Optional separate private key path |
| `AURORA_SERVER_NAME` | `var.ini` / hostname | Override the `<server>` part of the certificate filename |
| `AURORA_SESSION_TTL` | `24h` | Session lifetime (`time.ParseDuration`) |
| `AURORA_READ_ONLY` | `false` | Disable every mutation |
| `AURORA_LOG_LEVEL` | `info` | `debug` for verbose logs |

Flags `-addr` and `-version` are also available.

## Data sources

| Data | Source |
|---|---|
| System, array, disks, shares | `/var/local/emhttp/{var,disks,shares}.ini`, `/proc`, `/sys` |
| Docker | `/var/run/docker.sock` (`/containers/json`, `/containers/{id}/logs`) |
| Virtual machines | `virsh list --all`, `virsh domstate` |
| Notifications | `/tmp/notifications/{unread,archive}/*.notify` |
| Parity history | `/boot/config/parity-checks.log` |
| UPS | `apcaccess status` |
| Logs | `/var/log/syslog`, `/var/log/docker.log` |
| TLS | `/boot/config/ssl/certs/<server>_unraid_bundle.pem` |
| Icons | Docker icon cache, `templates-user` XML, `dynamix.vm.manager/templates/images` |
| Settings | `AURORA_DATA_DIR/settings.json` |
| Credentials | `getent shadow root`, `/etc/shadow` |

Actions use fixed binaries and validated targets: `emcmd`, `mdcmd`, `docker`, `virsh` and the stock
notification folders. They are a convenience layer, not a replacement for `emhttpd` or the Unraid API.

## Architecture

```
Browser ── React 19 SPA (embedded) ── /api/v1/* ── Go net/http
                                               │
                                      provider.Live ── /var/local/emhttp, procfs, sysfs,
                                               │        Docker socket, libvirt, apcupsd
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
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) — Go packages, API surface, authentication, live mapping.
- [`docs/DESIGN.md`](docs/DESIGN.md) — visual system, tokens, components, accessibility and motion.

## Feature map

| Area | Aurora |
|---|---|
| Dashboard | Health score, CPU per-core matrix, memory, array capacity, parity progress, network, UPS, containers, shares, activity |
| Storage | Array devices with live I/O and spin controls, shares, cache pools, parity check + history |
| Containers | Docker workloads with real icons, CPU/memory, ports, updates, lifecycle, logs and a settings editor for paths, ports, environment and restart policy |
| Virtual machines | KVM inventory, vCPU/RAM/disk usage, GPU passthrough, lifecycle actions |
| Network | Interfaces, bridges, WireGuard peers, listening services, traffic chart |
| GPUs | NVIDIA and AMD (ROCm) utilisation, VRAM, temperature, power, clocks, vendor logo and compute processes |
| Users | Accounts, roles, share access matrix, account drawer |
| Notifications | Inbox, unread/archived filters, acknowledge all, severity and source |
| Plugins | Installed extensions, versions, categories, update visibility |
| Logs | Syslog/Docker/parity tail, level + text filters, follow mode, export |
| Terminal | Full-page root shell over an authenticated WebSocket, openable in its own window |
| Storage operations | Start/stop array with passphrase or keyfile, background mover with progress, ETA and run history |
| Settings | Dark/light/system, five accents, density, units, refresh cadence, session info |
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

Aurora focuses on day-to-day observability and lifecycle operations. It deliberately does **not** implement
share editing, VM consoles (noVNC/SPICE), plugin installation, flash backups, Unraid Connect SSO or the full
Unraid GraphQL API. Those remain in Unraid itself.

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
