# Aurora — design system

Aurora is a **dark-first instrument panel**. The goal is not "dashboard glamour" but a surface that makes
dense infrastructure legible at a glance, with enough craft that people enjoy opening it every day.

Reference points: Linear (hierarchy and restraint), Vercel (contrast and typography), Raycast (density and
command palette), Arc (soft depth), Teenage Engineering (precise instrumentation).

## 1. Principles

1. **One accent, everywhere.** Ember orange is reserved for the primary action, live state, focus and charts.
   Everything else is graphite and neutral.
2. **Data before decoration.** Numbers use tabular monospace, labels are quiet, charts are thin. Glow and
   gradients never compete with values.
3. **Depth through light, not borders.** Cards use a 1 px hairline plus a top highlight and a soft ambient
   shadow. No heavy outlines, no bevels.
4. **A calm canvas.** A near-black background with a barely visible grid and film grain; a single aurora
   radial glow behind the content.
5. **Motion is confirmation, never decoration.** 200–520 ms, ease-out, no bounce, no looping except live
   indicators.
6. **One screen, one story.** The dashboard reads left-to-right: health → compute → storage → network →
   workloads → activity.

## 2. Colour

All colours are CSS custom properties in `frontend/src/styles/globals.css`, flipped by the `.dark` class and
mapped into Tailwind with `@theme inline`.

| Token | Dark | Light | Use |
|---|---|---|---|
| `--bg` | `#07080c` | `#f4f4f6` | Page canvas |
| `--surface` | `#0c0e14` | `#ffffff` | Cards |
| `--surface-2` | `#10131a` | `#fafafb` | Inputs, chips, nested blocks |
| `--surface-3` | `#151923` | `#f1f2f5` | Hover, tracks, active nav |
| `--text` | `#f3f4f7` | `#101116` | Primary text |
| `--muted` | `#969dae` | `#646a76` | Secondary text |
| `--subtle` | `#656c7c` | `#8b909c` | Labels, metadata |
| `--border` | white 7.5% | black 8% | Hairlines |
| `--accent` | `#ff6b4a` | `#f4502f` | Primary, live, chart |
| `--accent-2` | `#ff3d67` | `#ff7a3d` | Gradient end |
| `--success` | `#3ddc97` | `#0f9f6e` | Healthy, running |
| `--warning` | `#ffb020` | `#c47a00` | Attention, updates |
| `--danger` | `#ff5765` | `#dc3545` | Errors, destructive |
| `--info` | `#5aa9ff` | `#2f7ce0` | Network, read-only |
| `--violet` | `#9b7bff` | `#7c5cfc` | Parity, bridges, secondary series |

**Accents** (`data-accent` on `<html>`): `ember` (default), `violet`, `cyan`, `emerald`, `amber`. Each
defines `--accent`, `--accent-2`, soft backgrounds, line and glow. Semantic colours never change with the
accent.

**Semantic badges** use `color-mix(in srgb, var(--tone) 11%, transparent)` for the fill and 24% for the
border, so they stay readable in both modes without extra classes.

## 3. Typography

- **UI / display:** Inter Variable (`@fontsource-variable/inter`), tracking `-0.025em` on headings.
- **Numbers / code:** JetBrains Mono Variable, `font-variant-numeric: tabular-nums`, tracking `-0.04em`.
  Size is often one step smaller than the label because monospace reads larger.
- **Eyebrow:** 11 px, 600 weight, `0.14em` tracking, uppercase, `--subtle`.
- **Page title:** 26–30 px, 600, `-0.035em`; the greeting is sentence case with a period.
- **Card title:** 12.5–13 px, 600.
- **Body:** 11–12 px for data-dense rows, 13 px for descriptions.
- **Micro:** 9–10.5 px for axis labels, timestamps, port numbers.

The scale is intentionally compact. The layout gives it air through 16–24 px cards and 48 px section
rhythm, not through oversized type.

## 4. Spacing, radii and elevation

- 4 px base grid; common steps 6, 8, 10, 12, 16, 20, 24, 32, 48.
- Card radius 22 px, controls 10 px, pills 999 px, small tiles 8–12 px.
- Card padding 20 px mobile / 24 px desktop.
- Elevation: `0 1px 0 rgba(255,255,255,.03) inset, 0 24px 60px -32px rgba(0,0,0,.9)`.
- Popovers use a stronger shadow and a 1 px border; they never rely on shadow alone.

## 5. Layout

- **Sidebar** 264 px, collapsible to 82 px, fixed; groups *Overview*, *Infrastructure*, *System*.
- **Topbar** 64 px, frosted, breadcrumb left, command trigger, live pill, theme toggle, notifications, avatar.
- **Content** max 1560 px, 16/24/32 px gutters, 8 px section gap.
- **Dashboard** 12-column bento at ≥1280 px, 2 columns at ≥768 px, 1 column below.
- **Mobile** uses a slide-in navigation drawer; the command palette remains available from the topbar.

Card spans on the dashboard: health 4, CPU 5, memory 3 / array 5, network 4, UPS 3 / containers 5, shares 4,
activity 3.

## 6. Components

### Surfaces

- `.card` — surface gradient, hairline, top highlight, radius 22 px.
- `.glass` — 72% surface + 20 px blur, used by the sidebar/topbar and popovers.
- **Glass surface mode** (opt-in, `data-glass="on"`) — 42 % surface (58 % light) with a 28 px blur at
  180 % saturation, a specular top-left sheen and brighter rims. Enabled from Settings → Appearance →
  Surfaces; stored per device. Falls back to opaque surfaces when `prefers-reduced-transparency` is set.
- `.app-atmosphere` — fixed radial aurora glow + 56 px grid mask.
- `.grain` — 2–3.5% SVG noise for texture.

### Controls

- **Button**: primary gradient with dark text in dark mode; secondary surface; ghost; danger; soft. 32/36/44 px
  heights, 0.985 active scale.
- **IconButton**: 36 px square, relative (badges anchor correctly), tooltip via `aria-label` + title.
- **Badge**: 22 px pill, semantic tint, optional pulsing dot.
- **Tabs**: segmented control in a surface-2 track, white active pill with a hairline, horizontal scroll on
  mobile.
- **Switch**, **Progress**, **Avatar**, **Kbd**, **Input**, **Select**: small, consistent, no browser chrome.
- **Menu**, **Modal**, **Drawer**, **CommandPalette**: portalled, blurred scrim, Escape and click-outside,
  body scroll lock.
- **Toast**: bottom-right, max three, semantic icon, auto-dismiss.

### Data display

- **Ring**: SVG gradient arc with a blurred duplicate for glow, 7–10 px stroke, centre slot for the value.
- **AreaChart**: smooth Catmull-Rom path, gradient fill, dashed grid, crosshair, per-series tooltip, optional
  time axis. Downsamples to ≤180 points and always keeps the newest point.
- **Sparkline**: same path engine without axes.
- **CoreMatrix**: one bar per CPU thread; orange intensity encodes load, yellow above 75%.
- **SegmentBar**: stacked capacity segments (one per disk, then free) with hairline separators.
- **DiskBay**: mini drive caddy; fill height = usage, colour = role, amber dot = SMART warning.
- **Stat** patterns: eyebrow label, 21 px monospace value, one line of context.
- **Units**: storage is decimal (KB/MB/GB/TB, like Unraid and drive vendors); memory is binary
  (KiB/MiB/GiB) because that is how DIMMs and Docker limits are reported; throughput is decimal per second.

## 7. Motion

| Token | Duration | Use |
|---|---|---|
| `fade-up` | 520 ms | Page mount, staggered bento cards |
| `fade-in` | 300 ms | Scrims |
| `scale-in` | 240 ms | Menus, palette, modal |
| `slide-in` | 280 ms | Drawer, mobile nav |
| `toast-in` | 280 ms | Toast stack |
| `pulse-ring` | 2.4 s | Live status dot |
| `spin-slow` | 18 s | Login orbit decorations |

All animations respect `prefers-reduced-motion: reduce`. Stagger delays are capped at 340 ms so late cards
never feel slow.

## 8. Accessibility

- Focus-visible ring in the accent colour, 2 px offset.
- Every icon-only control has an `aria-label`; menus use `role="menu"`, tabs `role="tablist"`, switches
  `role="switch"` + `aria-checked`, progress `role="progressbar"` with values.
- Body text and secondary text target ≥ 4.5:1 contrast on both themes; subtle labels are decorative and
  paired with values.
- The command palette is fully keyboard driven (`⌘K`, `/`, arrows, Enter, Escape) and scrolls the active item
  into view.
- Charts include text summaries and tooltips on hover; colour is never the only signal (icons and labels
  accompany every state).
- `prefers-reduced-motion` disables all animation.

## 9. Page recipes

### Dashboard

Greeting + live badge + uptime, optional amber attention strip, then a 12-column bento. Every card carries a
6 px icon tile, a title and a trailing chip/badge. Numbers are 21–38 px monospace; context is 9–11 px.

### Storage

Four summary cards (capacity, protection, cache, throughput), a tab bar, then a device list grouped by role.
Columns collapse progressively; live I/O is optional to reduce noise. Parity gets a dedicated ring, history
chart and table.

### Containers

Summary metrics, filter tabs, search, grid/list switch. Cards expose icon, image, state, CPU/memory, ports and
lifecycle actions; a drawer adds logs with a terminal aesthetic (dark in both themes).

### Network

A wide traffic chart with time axis and a side connection card, then interface cards, WireGuard peers and
listening services. Download is blue, upload violet — never orange, to protect the accent for actions.

### Logs

Always dark terminal surface, traffic-light chrome, monospace timestamps, level colours (error red, warn amber,
info blue, debug grey), follow toggle and export.

### Settings

Vertical section nav, content cards with generous rows, theme preview thumbnails, accent swatches, and a
security section that states the real deployment mode instead of hiding it.

## 10. Copywriting

- Sentence case; no exclamation marks.
- Verbs for actions ("Restart", "Update images"), nouns for state ("Running", "Protected").
- Numbers always include a unit and a context ("43.3 TB of 72.8 TB", "18d 7h uptime").
- Error messages say what happened and what to do next; secrets are never echoed.
- The login screen speaks to the operator, not the product ("Your data. In perfect focus.").

## 11. Don'ts

- No second accent for decoration; semantic colours are reserved for state.
- No full-page gradients, neon shadows or glass everywhere.
- No chart libraries bolted on with their own palette.
- No text smaller than 9 px, no all-caps body copy.
- No silent failures: every mutation ends in a toast, every loading state has a skeleton, every empty state
  explains what would appear there.
