import {
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";
import { cn, IconButton, Kbd } from "./primitives";
import { useBodyScrollLock } from "../../lib/useBodyScrollLock";

/* -------------------------------------------------------------------------- */
/* Modal                                                                       */
/* -------------------------------------------------------------------------- */

export function Modal({
  open,
  onClose,
  title,
  description,
  children,
  footer,
  size = "md",
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  children: ReactNode;
  footer?: ReactNode;
  size?: "sm" | "md" | "lg";
}) {
  useBodyScrollLock(open);
  useEffect(() => {
    if (!open) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [open, onClose]);

  if (!open) return null;

  const widths = { sm: "max-w-sm", md: "max-w-lg", lg: "max-w-2xl" };

  return createPortal(
    <div className="fixed inset-0 z-[80] flex items-center justify-center p-4">
      <div
        className="animate-fade-in absolute inset-0 bg-black/55 backdrop-blur-[6px]"
        onClick={onClose}
        aria-hidden="true"
      />
      <div
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className={cn(
          "animate-scale-in relative w-full overflow-hidden rounded-2xl border border-line bg-[var(--surface)] shadow-[var(--shadow-pop)]",
          widths[size],
        )}
      >
        <div className="flex items-start justify-between gap-4 border-b border-line px-5 py-4">
          <div>
            <h2 className="text-[15px] font-semibold text-ink">{title}</h2>
            {description ? <p className="mt-0.5 text-xs text-muted">{description}</p> : null}
          </div>
          <IconButton label="Close" size="sm" onClick={onClose}>
            <X className="size-4" />
          </IconButton>
        </div>
        <div className="max-h-[70vh] overflow-y-auto px-5 py-4">{children}</div>
        {footer ? <div className="flex justify-end gap-2 border-t border-line px-5 py-3.5">{footer}</div> : null}
      </div>
    </div>,
    document.body,
  );
}

/* -------------------------------------------------------------------------- */
/* Drawer                                                                      */
/* -------------------------------------------------------------------------- */

export function Drawer({
  open,
  onClose,
  title,
  subtitle,
  children,
  footer,
  width = "max-w-[520px]",
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  subtitle?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
  width?: string;
}) {
  useBodyScrollLock(open);
  useEffect(() => {
    if (!open) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [open, onClose]);

  if (!open) return null;

  return createPortal(
    <div className="fixed inset-0 z-[80]">
      <div
        className="animate-fade-in absolute inset-0 bg-black/50 backdrop-blur-[3px]"
        onClick={onClose}
        aria-hidden="true"
      />
      <aside
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className={cn(
          "animate-slide-in absolute inset-y-0 right-0 flex w-full flex-col border-l border-line bg-[var(--surface)] shadow-[var(--shadow-pop)]",
          width,
        )}
      >
        <header className="flex items-start justify-between gap-4 border-b border-line px-6 py-5">
          <div className="min-w-0">
            <h2 className="truncate text-base font-semibold text-ink">{title}</h2>
            {subtitle ? <div className="mt-1 text-xs text-muted">{subtitle}</div> : null}
          </div>
          <IconButton label="Close panel" onClick={onClose}>
            <X className="size-4" />
          </IconButton>
        </header>
        <div className="flex-1 overflow-y-auto px-6 py-5">{children}</div>
        {footer ? <footer className="border-t border-line px-6 py-4">{footer}</footer> : null}
      </aside>
    </div>,
    document.body,
  );
}

/* -------------------------------------------------------------------------- */
/* Menu                                                                        */
/* -------------------------------------------------------------------------- */

export type MenuItem = {
  label: string;
  icon?: ReactNode;
  onSelect?: () => void;
  danger?: boolean;
  disabled?: boolean;
  separator?: boolean;
};

export function Menu({
  trigger,
  items,
  align = "end",
  width = "w-48",
}: {
  trigger: ReactNode;
  items: MenuItem[];
  align?: "start" | "end";
  width?: string;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onClick = (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as Node)) setOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onClick);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onClick);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div ref={ref} className="relative">
      <div onClick={() => setOpen((value) => !value)}>{trigger}</div>
      {open ? (
        <div
          role="menu"
          className={cn(
            "animate-scale-in absolute top-[calc(100%+6px)] z-50 overflow-hidden rounded-xl border border-line bg-[var(--surface)] p-1 shadow-[var(--shadow-pop)]",
            width,
            align === "end" ? "right-0 origin-top-right" : "left-0 origin-top-left",
          )}
        >
          {items.map((item, index) =>
            item.separator ? (
              <div key={`sep-${index}`} className="my-1 h-px bg-line" />
            ) : (
              <button
                key={item.label}
                role="menuitem"
                type="button"
                disabled={item.disabled}
                onClick={() => {
                  item.onSelect?.();
                  setOpen(false);
                }}
                className={cn(
                  "flex w-full items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-left text-[12.5px] transition-colors disabled:opacity-40",
                  item.danger
                    ? "text-[var(--danger)] hover:bg-[var(--danger-soft)]"
                    : "text-soft hover:bg-[var(--surface-3)] hover:text-ink",
                )}
              >
                {item.icon ? <span className="shrink-0 text-muted">{item.icon}</span> : null}
                {item.label}
              </button>
            ),
          )}
        </div>
      ) : null}
    </div>
  );
}

/* -------------------------------------------------------------------------- */
/* Command palette                                                             */
/* -------------------------------------------------------------------------- */

export type Command = {
  id: string;
  label: string;
  hint?: string;
  group: string;
  icon: ReactNode;
  keywords?: string;
  shortcut?: string;
  run: () => void;
};

function limitPerGroup(commands: Command[], limit: number): Command[] {
  const counts = new Map<string, number>();
  return commands.filter((command) => {
    const count = counts.get(command.group) ?? 0;
    if (count >= limit) return false;
    counts.set(command.group, count + 1);
    return true;
  });
}

export function CommandPalette({
  open,
  onClose,
  commands,
}: {
  open: boolean;
  onClose: () => void;
  commands: Command[];
}) {
  useBodyScrollLock(open);
  const [query, setQuery] = useState("");
  const [cursor, setCursor] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    setQuery("");
    setCursor(0);
    window.setTimeout(() => inputRef.current?.focus(), 20);
  }, [open]);

  const filtered = commands.filter((command) => {
    if (!query.trim()) return true;
    const haystack = `${command.label} ${command.hint ?? ""} ${command.group} ${command.keywords ?? ""}`.toLowerCase();
    return query
      .toLowerCase()
      .split(/\s+/)
      .every((word) => haystack.includes(word));
  });

  // The unfiltered list would be dominated by every container and VM, so each
  // group is trimmed until the user actually types something.
  const visible = query.trim() ? filtered : limitPerGroup(filtered, 4);

  useEffect(() => {
    setCursor(0);
  }, [query]);

  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        onClose();
      } else if (event.key === "ArrowDown") {
        event.preventDefault();
        setCursor((c) => Math.min(visible.length - 1, c + 1));
      } else if (event.key === "ArrowUp") {
        event.preventDefault();
        setCursor((c) => Math.max(0, c - 1));
      } else if (event.key === "Enter") {
        event.preventDefault();
        const command = visible[cursor];
        if (command) {
          onClose();
          command.run();
        }
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, visible, cursor, onClose]);

  useEffect(() => {
    const list = listRef.current;
    if (!list) return;
    const active = list.querySelector<HTMLElement>(`[data-index="${cursor}"]`);
    active?.scrollIntoView({ block: "nearest" });
  }, [cursor]);

  if (!open) return null;

  let lastGroup = "";
  let visibleIndex = -1;

  return createPortal(
    <div className="fixed inset-0 z-[95] flex items-start justify-center px-4 pt-[12vh]">
      <div className="animate-fade-in absolute inset-0 bg-black/55 backdrop-blur-[8px]" onClick={onClose} />
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Command palette"
        className="animate-scale-in relative w-full max-w-[620px] overflow-hidden rounded-2xl border border-line bg-[var(--surface)] shadow-[var(--shadow-pop)]"
      >
        <div className="flex items-center gap-3 border-b border-line px-4 py-3.5">
          <span className="eyebrow shrink-0 text-[var(--accent)]">Aurora</span>
          <input
            ref={inputRef}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search pages, containers, VMs, actions…"
            className="h-6 w-full border-0 bg-transparent text-sm text-ink outline-none placeholder:text-subtle"
          />
          <Kbd>esc</Kbd>
        </div>

        <div ref={listRef} className="max-h-[44vh] overflow-y-auto p-2">
          {visible.length === 0 ? (
            <p className="px-3 py-10 text-center text-xs text-muted">No results for “{query}”.</p>
          ) : (
            visible.map((command) => {
              visibleIndex += 1;
              const index = visibleIndex;
              const showGroup = command.group !== lastGroup;
              lastGroup = command.group;
              return (
                <div key={command.id}>
                  {showGroup ? (
                    <p className="px-2.5 pb-1.5 pt-3 text-[10px] font-semibold uppercase tracking-[0.12em] text-subtle">
                      {command.group}
                    </p>
                  ) : null}
                  <button
                    type="button"
                    data-index={index}
                    onMouseEnter={() => setCursor(index)}
                    onClick={() => {
                      onClose();
                      command.run();
                    }}
                    className={cn(
                      "flex w-full items-center gap-3 rounded-xl px-2.5 py-2 text-left transition-colors",
                      index === cursor ? "bg-[var(--surface-3)]" : "hover:bg-[var(--surface-2)]",
                    )}
                  >
                    <span className="grid size-7 shrink-0 place-items-center rounded-lg border border-line bg-[var(--surface-2)] text-muted">
                      {command.icon}
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-[13px] font-medium text-ink">{command.label}</span>
                      {command.hint ? <span className="block truncate text-[11px] text-muted">{command.hint}</span> : null}
                    </span>
                    {command.shortcut ? <Kbd>{command.shortcut}</Kbd> : null}
                    {index === cursor ? <span className="text-[10px] text-subtle">↵</span> : null}
                  </button>
                </div>
              );
            })
          )}
        </div>

        <div className="flex items-center justify-between border-t border-line px-4 py-2.5 text-[10.5px] text-subtle">
          <span className="flex items-center gap-3">
            <span className="flex items-center gap-1"><Kbd>↑</Kbd><Kbd>↓</Kbd> navigate</span>
            <span className="flex items-center gap-1"><Kbd>↵</Kbd> open</span>
          </span>
          <span className="flex items-center gap-1.5">
            <span className="status-dot live" /> {visible.length} results
          </span>
        </div>
      </div>
    </div>,
    document.body,
  );
}
