# Unraid webGUI — full source analysis

This document records what the [unraid/webgui](https://github.com/unraid/webgui) repository actually does,
how its pieces fit together, and which decisions shaped the Aurora rebuild. It is based on a shallow clone
of the public repository (October 2026, Unraid 7.2.x era), read file by file across `emhttp/`, `sbin/`,
`etc/rc.d/` and the bundled plugins.

## 1. Size and shape

| Metric | Value |
|---|---|
| Repository size (working tree) | ~33 MB |
| `emhttp/` web root | ~25 MB |
| PHP files / lines | 158 / 38,509 |
| `.page` templates / lines | 134 / 32,974 |
| JavaScript files / lines | 385 / 227,427 (includes bundled libraries and noVNC/SPICE) |
| CSS lines | 13,039 (including themes and vendor styles) |
| Plugins | 7 directories, 1,092 files |
| Shell/PHP executables without extension | 111 |

The repository is **not** a standalone application. It is the `/usr/local/emhttp` tree extracted into an
Unraid image at build time. The actual state engine, `emhttpd`, is a closed-source binary started by
`/usr/local/sbin/emhttp`; the webGUI is a client of the files it writes.

```
emhttp/
  auth-request.php, login.php, update.php        entry points
  plugins/dynamix/                                main GUI (113 pages, JS, CSS, includes)
  plugins/dynamix.docker.manager/                 Docker UI + Docker API client
  plugins/dynamix.vm.manager/                     libvirt UI + noVNC/SPICE clients
  plugins/dynamix.plugin.manager/                 .plg installation and updates
  plugins/dynamix.apcupsd/                        UPS pages
  plugins/dynamix.gui.search/                     global search
  plugins/unRAIDServer/                           OS update/registration surface
etc/rc.d/                                         nginx, php-fpm, docker, libvirt, samba...
sbin/                                             emhttp, mdcmd, mover, diagnostics, helpers
```

## 2. Runtime topology

```
Browser
  │  HTTP / WebSocket
  ▼
nginx  ── /            → 302 /$START_PAGE
       ── /login       → php-fpm (login.php)
       ── /[A-Z]*      → template.php  (every page)
       ── /plugins/*   → static + PHP
       ── /sub/<chan>  → nchan subscriber (WebSocket / SSE / long-poll)
       ── /pub/<chan>  → nchan publisher on a Unix socket
       ── /graphql     → unix:/var/run/unraid-api.sock   (Unraid API, separate repo)
       ── /wsproxy/*   → TCP proxy for noVNC/SPICE/terminal
       ── /update.htm  → unix:/var/run/emhttpd.socket
       ── /logging.htm → unix:/var/run/emhttpd.socket
       ── /webterminal → unix:/var/run/<tag>.sock (ttyd)
  │
  ├── php-fpm (PHP 8)
  ├── emhttpd (closed source) → writes /var/local/emhttp/*.ini
  ├── unraid-api (Node/GraphQL, optional)
  ├── nchan worker scripts (PHP/bash pollers)
  ├── dockerd (Unix socket)
  └── libvirtd (virsh/qemu)
```

Important detail: the **webGUI never talks to the kernel directly for array state**. It renders INI files
written by `emhttpd` and asks `emhttpd` to perform operations through `emcmd`. That inversion is the key
compatibility contract Aurora preserves.

## 3. Request lifecycle

1. nginx matches the request. Hidden files are denied; `/login`, `/logout`, static assets and a small
   whitelist bypass authentication.
2. Authentication is enforced by nginx `auth_request` calling `auth-request.php`. The PHP session must
   contain `unraid_login`; the timestamp is refreshed on activity. A failed check returns 401 and nginx
   redirects to `/login`.
3. `/login` renders a layout with the locale from `dynamix.cfg`; a successful POST creates the session and
   redirects to `$START_PAGE` (usually `Main`), unless the license needs attention (`Tools/Registration`).
4. Any path matching `^/[A-Z].*` is routed to `template.php`. That file:
   - starts the session and loads `Helpers.php`, `Wrappers.php`, translations;
   - parses `state/{var,disks,shares,users,sec,sec_nfs,devs,network}.ini`;
   - scans `webGui/*.page` and every `plugins/*/*.page` (`PageBuilder.php`);
   - resolves the current task from the URL, then includes `DefaultPageLayout.php`.
5. `DefaultPageLayout.php` builds the HTML shell (theme, fonts, CSS, JS, header, navigation, footer),
   starts nchan pollers declared by pages, and evals the selected page body.
6. Page bodies are PHP embedded in `.page` files. They emit raw HTML with helpers such as `_()` for
   translations, `_var()` for safe array access, `my_scale()` for units and `mk_option()` for selects.
   Table/form submissions post to `/update.php` or to page-specific PHP endpoints under
   `/plugins/dynamix/include/`.

## 4. The `.page` system

Every page starts with an INI-like metadata header followed by a PHP body:

```
Menu="Main:5"
Title="Array Operation"
Tag="snowflake-o"
Nchan="device_list,disk_load,parity_list"
---
<?PHP ... ?>
```

Metadata keys observed across the 134 pages:

| Key | Meaning |
|---|---|
| `Menu` | Menu path and rank, e.g. `Settings:2`, `Disk Share`, `Buttons:3` |
| `Type` | `menu`, `xmenu` (menu with sub-pages), `tabs`, `xmenu` |
| `Title` | Display title; may contain `$variables` resolved by `processTitle()` |
| `Icon` / `Tag` | Font Awesome, `icon-*` font, or PNG icon |
| `Tabs` | Force tabbed or tabless rendering |
| `Code` | Legacy menu code |
| `Load` | Client refresh interval in seconds |
| `Nchan` | Comma-separated poller scripts to start |
| `Cond` | Conditional visibility expression |
| `Markdown` | Render the body through the bundled Markdown parser |
| `Eval` | Evaluate body instead of including it |
| `Link`, `Href` | Render as an external button instead of a page |
| `Lock` | Enable sortable-layout lock |
| `Name`, `root`, `file`, `text` | Populated by the page scanner |

Pages are grouped into tasks (`Dashboard`, `Main`, `Shares`, `Settings`, `Users`, `Plugins`, `Tools`,
`NetworkSettings`, …). A task is the first URL segment; sub-pages are tabs/panels. There is no front
controller or router: **the filesystem is the route table**, and page metadata is the navigation model.

### Consequences

- Adding a feature usually means adding a `.page`, a menu entry and one or more PHP endpoints.
- The same page can be rendered as a tab, a panel, a menu or a standalone route depending on context.
- Page bodies freely read and mutate global variables (`$var`, `$disks`, `$display`, `$myPage`), which makes
  refactoring risky and testing difficult.
- Markup, business logic, translations and shell calls are interleaved in the same files.

## 5. State model

`emhttpd` and helpers write a small family of INI files. They are the de-facto public API of the system.

| File | Content |
|---|---|
| `var.ini` | Array state, parity progress, registration, ports, CSRF token, start page, model, license |
| `disks.ini` | One section per device: `device`, `type`, `fsType`, `fsSize`, `fsUsed`, `temp`, `smart`, `status`, `color`, `spinupGroup`, serial, firmware, pool |
| `shares.ini` | One section per share: allocation, split level, cache mode, pool, export flags, security, size/free |
| `users.ini`, `sec.ini`, `sec_nfs.ini` | Accounts and SMB/NFS access rules |
| `devs.ini` | Physical device inventory |
| `network.ini`, `nginx.ini` | Interfaces, bridges, DNS, ports, TLS |
| `monitor.ini`, `diskload.ini` | Runtime monitoring and per-disk I/O rates |
| `/tmp/notifications/**/*.notify` | Notification INI files (`timestamp`, `event`, `subject`, `description`, `importance`) |
| `/var/local/emhttp/tasks/*.json` | Scheduler jobs |

Notable conventions:

- Sizes in `disks.ini` are KiB although the UI labels them TB/GB.
- Temperatures are Celsius; `*` means unavailable/spun down.
- Colors such as `green-on`, `yellow-blink` encode array/disk state, not CSS.
- `mdResync*` fields describe parity rebuild/check progress; `sbSynced*` the last completed check.
- `csrf_token` in `var.ini` is published over nchan so tabs can refresh their CSRF token.

Helpers such as `my_scale()`, `my_temp()`, `my_time()` and `usage_color()` centralise display rules, but
they are procedural globals rather than a model layer.

## 6. Realtime: nchan

Unraid ships nginx with the **nchan** module. Realtime data is not WebSocket-first from PHP; instead small
poller scripts publish to channels, and nchan fans out to browsers over WebSocket, SSE or long-poll
(`NchanSubscriber` in `dynamix.js` negotiates).

Publishers use `publish.php`:

```
curl → http://localhost/pub/<channel>?buffer_length=N
       via unix:/var/run/nginx.socket
```

Channels observed in `plugins/dynamix/nchan/` and plugin managers:

| Channel | Producer | Payload |
|---|---|---|
| `notify` | `notify_poller` | Unread notification JSON, republished when the hash changes |
| `session` | `session_check` | CSRF token |
| `device_list` | `device_list` | Device inventory and state |
| `disk_load` | `disk_load` | Per-disk read/write bytes, polled from `/proc/diskstats` every 2 s |
| `parity_list` | `parity_list` | Parity sync/check progress |
| `ups_status` | `ups_status` | UPS state and measurements |
| `vm_dashusage` | `vm_dashusage` | VM utilisation |
| `docker` | Docker manager | Container events |
| `vmaction` | VM manager | VM actions |
| `plugins` | Plugin manager | Install/update progress |
| `tasks` | Task manager | Scheduler notifications |
| `file_manager`, `wlan0`, `wireguard` | various | File operations, Wi-Fi, WireGuard |

`DefaultPageLayout.php` starts the pollers announced by the current pages, tracks their PIDs in
`/var/run/nchan.pid`, and each poller exits after ~30 s without subscribers. This is efficient, but it ties
realtime behaviour to page navigation and PHP CLI processes.

## 7. Front end

The classic UI is server-rendered HTML plus a large imperative layer:

| Asset | Role |
|---|---|
| `javascript/dynamix.js` (534 KB) | Menus, tabs, forms, dialogs, sorting, charts, nchan, toasts, formatting |
| jQuery + jQuery UI | DOM, dialogs, sortable dashboard |
| ApexCharts (`jquery.apexcharts.js`, 466 KB) | Dashboard and statistics charts |
| Smoothie | Legacy realtime line charts |
| tablesorter | Tables, pagination, filters |
| SweetAlert | Confirmation dialogs |
| Font Awesome + custom icon fonts | Iconography |
| `default-base.css`, `default-dynamix.css` | ~126 KB of hand-written CSS |
| Themes `black`, `white`, `gray`, `azure` | CSS-variable palettes selected in settings |
| `default-fonts.css` | Clear Sans / Bitstream Vera |
| `Translations.php` + `languages/*` | ~30 locales, `_()` lookups, cached serialized dictionaries |

In 7.2 the header (and sidebar array-usage widget) are **web components** mounted as `<unraid-header>`.
The PHP layout includes a light-DOM fallback and the component is served from
`plugins/dynamix.my.servers/unraid-components`; comments in `Header.php` reference Vue SFCs and Tailwind
units, showing that the newest surface is Vue-based while the rest of the UI remains PHP/jQuery.

### UX audit

Strengths worth preserving:

- Extremely dense operational information: array, disks, shares, parity, Docker, VMs and network on one
  dashboard.
- Colour semantics are consistent (green/yellow/red orbs, usage bars, SMART states).
- Every action has a server-rendered fallback; the UI works with JavaScript partially disabled.
- Deep integration with the actual system: no synthetic abstractions between the user and `emhttpd`.
- Themes and font scaling are first-class settings.

Weaknesses that motivated a rebuild:

- Visual language is dated: gradients, beveled panels, mixed icon families and inconsistent spacing.
- Information hierarchy is flat; dashboards are grids of tiles rather than a designed narrative.
- Mobile support is an afterthought; many pages assume a desktop table.
- 500 KB+ of blocking JavaScript, jQuery plugins and global state.
- No design tokens: colours are spread between palettes, CSS and inline styles.
- Accessibility is limited (icon-only controls, low-contrast secondary text, div-based controls).
- Server-rendered tabs reload the whole shell; only a few widgets are live.
- Coupling to INI files, shell commands and closed-source `emhttpd` makes testing and extension hard.

## 8. Feature domains

### Array and disks

`Main`, `ArrayOperation`, `Disk`, `DiskList`, `DeviceInfo`, `Selftest` and friends render `disks.ini` and
`var.ini`. Start/stop, spin-up/down, parity check, SMART tests and filesystem checks are issued via
`emcmd`/`mdcmd` scripts. Disk rows include temperature, SMART status, filesystem usage, spin state and
per-device throughput from the `disk_load` channel.

### Shares

`ShareList`, `ShareEdit`, `ShareData`, `SecuritySMB`, `SecurityNFS` map `shares.ini` and export rules.
Allocation strategies (`highwater`, `mostfree`, `fill-up`), split levels, cache modes (`yes`, `no`, `only`,
`prefer`) and include/exclude disk lists are edited in place and written back by `Dispatcher.php`.

### Docker

`dynamix.docker.manager` is effectively a Docker client in PHP:

- `DockerClient.php` (49 KB) talks to `/var/run/docker.sock` directly.
- `CreateDocker.php` (83 KB) builds containers from Unraid XML templates with port/volume/env UI.
- Containers are annotated through labels (`net.unraid.docker.icon`, `webui`, etc.).
- Updates, rebuilds and log streaming are shell/HTTP hybrids; the `docker` nchan channel carries events.
- The UI exposes per-container CPU/memory, autostart, network, ports, logs and a console.

### Virtual machines

`dynamix.vm.manager` uses libvirt:

- `virsh` for lifecycle, XML for hardware definitions.
- noVNC and SPICE clients are bundled (most of the 8.8 MB plugin size).
- Templates, vDisk management, PCI/USB passthrough and VM usage stats are separate pages.
- `vm_dashusage` streams utilisation.

### Plugins and OS updates

`.plg` files are shell/XML packages installed by `plugin.manager`. The store is Community Applications
(external). Plugin pages can contribute their own `.page` files, menu entries, nchan channels and PHP.
`unRAIDServer` handles the OS image, registration and update notes.

### Notifications

`plugins/dynamix/scripts/notify` is a full notification daemon in bash: it writes `.notify` INI files,
sends mail through `ssmtp`, and fans out to agents (Discord, Slack, Pushover, ntfy, Gotify, Telegram,
Pushbullet…). The web inbox is a poller over the unread folder plus a browser channel.

### Terminal and logs

`ttyd` instances are proxied through nginx (`/webterminal`, `/logterminal`). Logs and SSH sessions are
therefore WebSocket streams behind the same session cookie. `diagnostics` bundles logs, SMART data,
PCI/USB inventories and anonymised config.

## 9. Security model

- **Session authentication** via PHP sessions checked by nginx `auth_request`; `/logout` destroys the
  session. A small whitelist allows fonts, icons, manifest and `robots.txt` without auth.
- **CSRF**: a token in `var.ini`, exposed through the `session` nchan channel; mutating PHP endpoints check
  it. The token rotates with the array state, not per request.
- **Path validation**: `auth-request.php` canonicalises paths and checks they stay inside the docroot
  before allowing public assets or web-component files.
- **CSP**: pages set `block-all-mixed-content`; the server does not ship a strict script CSP.
- **Input handling**: helpers (`InputSecurity.php`, `escapeshellarg`, `safe_filename`) exist, but hundreds
  of endpoints mix `$_POST`, shell calls and file paths. Security depends on every page author.
- **Privilege**: PHP runs as root on the host. A plugin or a page bug is a host-level compromise.
- **Rate limiting**: nginx `limit_req` protects `/login`; no account lockout at the application level.
- **No API tokens**: the classic UI is session-only; the separate `unraid-api` adds GraphQL and keys.

Aurora's threat model is deliberately smaller: no shell execution from the browser, no plugin code, no
arbitrary file access, and every mutation is an enumerated action.

## 10. Coupling and risk points

| Area | Coupling |
|---|---|
| `emhttpd` | Closed source, undocumented INI schema and `emcmd` verbs |
| nginx/nchan | Realtime depends on the patched nginx build and Unix sockets |
| PHP pages | Global variables, filesystem routing, server-rendered tabs |
| Plugins | Contribute PHP, shell, pages and channels with full root access |
| JavaScript | One 534 KB file with implicit globals and jQuery assumptions |
| Assets | Icons/fonts mixed between PNG, Font Awesome and custom fonts |

Any rewrite must either **reimplement the state engine** (large, risky) or **read the same contracts**
(safer, what Aurora does). Aurora takes the second path.

## 11. Rebuild mapping

| Original | Aurora |
|---|---|
| `.page` metadata + PHP body | React route + typed React components |
| `template.php` page scan | Explicit route table in `App.tsx` |
| `DefaultPageLayout.php` | `AppShell`, `Sidebar`, `Topbar` |
| `var.ini`, `disks.ini`, `shares.ini` | `provider.Live` INI parsing → `model.Snapshot` |
| `publish.php` + nchan | `metrics.Hub` → SSE `/api/v1/events` |
| `notify_poller` | `GET /api/v1/notifications` + mutation actions |
| `disk_load` bash poller | `/proc/diskstats` deltas in `metrics.Hub` |
| `docker` manager | Docker Engine HTTP over Unix socket |
| `virsh` VM manager | `virsh` process adapter (read-only inventory) |
| `dynamix.js` behaviours | React state, custom hooks, design-system primitives |
| ApexCharts | Hand-built SVG area charts, rings, sparklines, core matrix |
| Themes | CSS variables, five accents, dark/light/system |
| jQuery dialogs | Accessible drawer/modal/palette components |
| PHP sessions + CSRF token | Native shadow login, HMAC cookie, per-request CSRF header, origin check |
| Root PHP endpoints | Whitelisted Go actions with validated targets |

## 12. What was intentionally left out

- **Share/user editing** — writes touch Samba, NFS and `emhttpd`; the read-only mirror is safer.
- **VM consoles and terminal** — WebSocket PTY/noVNC/SPICE is a separate security surface.
- **Plugin installation and OS updates** — these run arbitrary root code; out of scope for a UI study.
- **A replacement for `emhttpd`** — Aurora is a control plane, not a new array engine.
- **Unraid GraphQL parity** — the official API is a better integration point for automation.

## 13. Metrics summary

| | Original | Aurora |
|---|---|---|
| Server language | PHP 8 + bash | Go 1.27 (stdlib + crypt) |
| Transport | nginx + PHP-FPM | In-process Go HTTPS (TLS 1.2+) |
| Routing | Filesystem + nginx | Explicit Go 1.22+ `ServeMux` patterns |
| UI | PHP templates + jQuery | React 19 + TypeScript |
| Realtime | nchan + PHP pollers | SSE + in-process sampler |
| Page count | 134 `.page` files | 11 React routes |
| Client JS | ~500 KB minified core | ~516 KB (141 KB gzip) including React |
| CSS | ~130 KB hand-written | ~80 KB generated (17 KB gzip) |
| Theme | 4 fixed palettes | 2 modes × 5 accents + density |
| API | Page-scoped PHP endpoints | `/api/v1/*` JSON contract |
| Tests | `tests/` shell fixtures | Go unit + HTTP integration tests |

The point is not that fewer files is inherently better — the original encodes twenty years of edge cases.
The point is that a typed contract, a small API and a real design system make the same operational
knowledge easier to extend, test and present.
