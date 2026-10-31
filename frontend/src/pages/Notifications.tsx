import { useMemo, useState, type ReactNode } from "react";
import {
  Bell,
  BellRing,
  Boxes,
  CheckCheck,
  CircleAlert,
  Cloud,
  HardDrive,
  Inbox,
  Mail,
  RefreshCw,
  Search,
  TriangleAlert,
  BatteryCharging,
  Archive,
  Check,
} from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { Drawer, Menu } from "../components/ui/overlays";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  IconButton,
  InputBase,
  Tabs,
  cn,
} from "../components/ui/primitives";
import { useApp } from "../app/AppDataProvider";
import { useAction } from "../lib/useAction";
import { formatDateTime, relativeTime } from "../lib/format";
import type { Notification } from "../lib/types";

type Filter = "all" | "unread" | "archived";

const IMPORTANCE = {
  alert: { tone: "danger" as const, icon: TriangleAlert, label: "Alert" },
  warning: { tone: "warning" as const, icon: CircleAlert, label: "Warning" },
  normal: { tone: "info" as const, icon: Bell, label: "Info" },
};

const SOURCE_ICONS: Record<string, typeof HardDrive> = {
  array: HardDrive,
  smart: HardDrive,
  docker: Boxes,
  connect: Cloud,
  ups: BatteryCharging,
};

export function NotificationsPage() {
  const { snapshot, refresh } = useApp();
  const action = useAction();
  const [filter, setFilter] = useState<Filter>("all");
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<Notification | null>(null);

  if (!snapshot) return null;
  const notifications = snapshot.notifications;
  const unread = notifications.filter((item) => !item.read && !item.archived);
  const archived = notifications.filter((item) => item.archived);

  const filtered = useMemo(
    () =>
      notifications
        .filter((item) => {
          if (filter === "unread") return !item.read && !item.archived;
          if (filter === "archived") return item.archived;
          return !item.archived;
        })
        .filter((item) => {
          if (!query.trim()) return true;
          return `${item.title} ${item.subject} ${item.description}`.toLowerCase().includes(query.toLowerCase());
        })
        .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime()),
    [notifications, filter, query],
  );

  return (
    <>
      <PageHeader
        eyebrow="Overview"
        title="Notifications"
        description="Everything Unraid and your services want you to know, in one calm inbox."
        meta={
          <>
            <Badge tone={unread.length > 0 ? "accent" : "success"} dot pulse={unread.length > 0}>
              {unread.length > 0 ? `${unread.length} unread` : "All caught up"}
            </Badge>
            <span className="chip">{notifications.length} total</span>
          </>
        }
        actions={
          <>
            <Button variant="secondary" icon={<RefreshCw className="size-3.5" />} onClick={refresh}>
              Refresh
            </Button>
            <Button
              variant="primary"
              icon={<CheckCheck className="size-3.5" />}
              disabled={unread.length === 0}
              loading={action.isPending}
              onClick={() => action.mutate({ kind: "notification.ackall", success: "All notifications acknowledged" })}
            >
              Mark all read
            </Button>
          </>
        }
      />

      <div className="mb-5 grid grid-cols-2 gap-3.5 lg:grid-cols-4">
        <Stat icon={<Inbox className="size-4" />} label="Inbox" value={String(notifications.filter((n) => !n.archived).length)} hint="active notifications" />
        <Stat icon={<BellRing className="size-4" />} label="Unread" value={String(unread.length)} hint="waiting for you" tone={unread.length ? "warning" : "success"} />
        <Stat icon={<CircleAlert className="size-4" />} label="Warnings" value={String(notifications.filter((n) => n.importance !== "normal" && !n.archived).length)} hint="warning or alert level" />
        <Stat icon={<Archive className="size-4" />} label="Archived" value={String(archived.length)} hint="kept for reference" />
      </div>

      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <Tabs
          items={[
            { value: "all", label: "Inbox", count: notifications.filter((n) => !n.archived).length },
            { value: "unread", label: "Unread", count: unread.length },
            { value: "archived", label: "Archived", count: archived.length },
          ]}
          value={filter}
          onChange={(value) => setFilter(value as Filter)}
        />
        <InputBase
          value={query}
          onChange={setQuery}
          placeholder="Search notifications…"
          icon={<Search className="size-3.5" />}
          className="w-56"
        />
      </div>

      {filtered.length === 0 ? (
        <Card>
          <EmptyState
            icon={<Bell className="size-5" />}
            title={filter === "unread" ? "No unread notifications" : filter === "archived" ? "Nothing archived yet" : "Your inbox is clear"}
            description="Notifications from the array, Docker, UPS and plugins will show up here."
          />
        </Card>
      ) : (
        <Card>
          <div className="divide-y divide-line/60">
            {filtered.map((item) => {
              const config = IMPORTANCE[item.importance as keyof typeof IMPORTANCE] ?? IMPORTANCE.normal;
              const LevelIcon = config.icon;
              const SourceIcon = SOURCE_ICONS[item.source] ?? Bell;
              return (
                <div
                  key={item.id}
                  className={cn(
                    "group flex cursor-pointer gap-3.5 px-5 py-4 transition hover:bg-[var(--surface-2)]/50",
                    !item.read && !item.archived && "bg-[var(--accent-softer)]/40",
                  )}
                  onClick={() => setSelected(item)}
                >
                  <span
                    className="mt-0.5 grid size-9 shrink-0 place-items-center rounded-xl border"
                    style={{
                      color: `var(--${config.tone})`,
                      backgroundColor: `color-mix(in srgb, var(--${config.tone}) 10%, transparent)`,
                      borderColor: `color-mix(in srgb, var(--${config.tone}) 22%, transparent)`,
                    }}
                  >
                    <LevelIcon className="size-4" />
                  </span>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="truncate text-[12.5px] font-semibold text-ink">{item.title}</p>
                      {!item.read && !item.archived ? <span className="size-1.5 rounded-full bg-[var(--accent)]" /> : null}
                      {item.archived ? <Badge tone="neutral">archived</Badge> : null}
                    </div>
                    <p className="mt-0.5 truncate text-[11px] text-soft">{item.subject}</p>
                    <p className="mt-1 line-clamp-2 text-[10.5px] leading-relaxed text-muted">{item.description}</p>
                    <div className="mt-2 flex flex-wrap items-center gap-2 text-[9.5px] text-subtle">
                      <span className="flex items-center gap-1"><SourceIcon className="size-3" /> {item.source}</span>
                      <span>·</span>
                      <span>{relativeTime(item.timestamp)}</span>
                      <span>·</span>
                      <span>{formatDateTime(item.timestamp)}</span>
                    </div>
                  </div>
                  <div className="flex shrink-0 items-start gap-1" onClick={(event) => event.stopPropagation()}>
                    {!item.read && !item.archived ? (
                      <IconButton
                        label="Mark as read"
                        size="sm"
                        onClick={() => action.mutate({ kind: "notification.ack", target: item.id, success: "Notification acknowledged" })}
                      >
                        <Check className="size-3.5" />
                      </IconButton>
                    ) : null}
                    <Menu
                      trigger={
                        <IconButton label="Notification actions" size="sm">
                          <span className="text-[15px] leading-none">⋯</span>
                        </IconButton>
                      }
                      items={[
                        {
                          label: "Mark as read",
                          icon: <Check className="size-3.5" />,
                          disabled: item.read,
                          onSelect: () => action.mutate({ kind: "notification.ack", target: item.id, success: "Notification acknowledged" }),
                        },
                        {
                          label: "Archive",
                          icon: <Archive className="size-3.5" />,
                          disabled: item.archived,
                          onSelect: () => action.mutate({ kind: "notification.archive", target: item.id, success: "Notification archived" }),
                        },
                      ]}
                    />
                  </div>
                  <Chevron />
                </div>
              );
            })}
          </div>
        </Card>
      )}

      <Drawer
        open={Boolean(selected)}
        onClose={() => setSelected(null)}
        title={selected?.title ?? ""}
        subtitle={selected?.subject}
        footer={
          selected ? (
            <div className="flex gap-2">
              <Button
                variant="secondary"
                className="flex-1"
                icon={<Check className="size-3.5" />}
                disabled={selected.read}
                onClick={() => {
                  action.mutate({ kind: "notification.ack", target: selected.id, success: "Notification acknowledged" });
                  setSelected(null);
                }}
              >
                Mark as read
              </Button>
              <Button
                variant="primary"
                className="flex-1"
                icon={<Archive className="size-3.5" />}
                disabled={selected.archived}
                onClick={() => {
                  action.mutate({ kind: "notification.archive", target: selected.id, success: "Notification archived" });
                  setSelected(null);
                }}
              >
                Archive
              </Button>
            </div>
          ) : null
        }
      >
        {selected ? (
          <div className="space-y-5">
            <div className="flex flex-wrap items-center gap-2">
              <Badge tone={IMPORTANCE[selected.importance as keyof typeof IMPORTANCE]?.tone ?? "info"}>
                {IMPORTANCE[selected.importance as keyof typeof IMPORTANCE]?.label ?? selected.importance}
              </Badge>
              <Badge tone="neutral">{selected.source}</Badge>
              {selected.read ? <Badge tone="success">read</Badge> : <Badge tone="accent" dot>unread</Badge>}
              {selected.archived ? <Badge tone="neutral">archived</Badge> : null}
            </div>
            <div className="rounded-xl border border-line bg-[var(--surface-2)]/50 p-4">
              <p className="text-[12.5px] font-semibold text-ink">{selected.subject}</p>
              <p className="mt-2 whitespace-pre-line text-[11.5px] leading-relaxed text-muted">{selected.description}</p>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <Info label="Received" value={formatDateTime(selected.timestamp)} />
              <Info label="Relative" value={relativeTime(selected.timestamp)} />
              <Info label="Source" value={selected.source} />
              <Info label="Identifier" value={selected.id} />
            </div>
            <div className="flex items-start gap-2 rounded-xl border border-line p-3 text-[10.5px] text-muted">
              <Mail className="mt-0.5 size-3.5 shrink-0 text-subtle" />
              Email and push delivery follow your notification agent settings. Aurora keeps the in-app inbox in
              sync with Unraid.
            </div>
          </div>
        ) : null}
      </Drawer>
    </>
  );
}

function Stat({
  icon,
  label,
  value,
  hint,
  tone,
}: {
  icon: ReactNode;
  label: string;
  value: string;
  hint: string;
  tone?: "warning" | "success";
}) {
  return (
    <Card className="card-pad">
      <div className="flex items-center gap-2">
        <span className="grid size-6 place-items-center rounded-md border border-line bg-[var(--surface-2)] text-muted">{icon}</span>
        <p className="text-[10px] font-medium uppercase tracking-[0.1em] text-subtle">{label}</p>
      </div>
      <p className={cn("num mt-3 text-[21px] font-semibold leading-none", tone === "warning" ? "text-[var(--warning)]" : tone === "success" ? "text-[var(--success)]" : "text-ink")}>
        {value}
      </p>
      <p className="mt-1.5 text-[10px] text-muted">{hint}</p>
    </Card>
  );
}

function Info({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-line bg-[var(--surface-2)]/50 px-3 py-2">
      <p className="text-[9px] uppercase tracking-[0.1em] text-subtle">{label}</p>
      <p className="mt-1 truncate font-mono text-[10.5px] text-soft">{value}</p>
    </div>
  );
}

function Chevron() {
  return (
    <span className="hidden self-center text-subtle transition group-hover:translate-x-0.5 group-hover:text-muted sm:block">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
        <path d="m9 18 6-6-6-6" />
      </svg>
    </span>
  );
}
