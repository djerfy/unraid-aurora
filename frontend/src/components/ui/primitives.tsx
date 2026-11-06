import {
  forwardRef,
  useId,
  useState,
  type ButtonHTMLAttributes,
  type ElementType,
  type HTMLAttributes,
  type ReactNode,
} from "react";
import { ChevronRight, Loader2 } from "lucide-react";
import { useI18n } from "../../i18n";

export function cn(...values: Array<string | false | null | undefined>): string {
  return values.filter(Boolean).join(" ");
}

/* -------------------------------------------------------------------------- */
/* Button                                                                      */
/* -------------------------------------------------------------------------- */

type ButtonVariant = "primary" | "secondary" | "ghost" | "danger" | "soft";
type ButtonSize = "sm" | "md" | "lg" | "icon";

const BUTTON_SIZES: Record<ButtonSize, string> = {
  sm: "h-8 rounded-lg px-3 text-xs gap-1.5",
  md: "h-9 rounded-[7px] px-3.5 text-[13px] gap-2",
  lg: "h-11 rounded-xl px-5 text-sm gap-2",
  icon: "size-9 rounded-[7px]",
};

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  size?: ButtonSize;
  loading?: boolean;
  icon?: ReactNode;
};

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = "secondary", size = "md", loading, icon, className, children, disabled, ...props },
  ref,
) {
  const base =
    "relative inline-flex select-none items-center justify-center font-medium tracking-[-0.01em] transition-all duration-200 active:scale-[0.985] disabled:pointer-events-none disabled:opacity-45";
  const variants: Record<ButtonVariant, string> = {
    primary: "text-[var(--accent-contrast)] shadow-[0_1px_0_rgba(255,255,255,0.18)_inset,0_8px_24px_-12px_var(--accent)] hover:brightness-[1.06]",
    secondary:
      "border border-line bg-[var(--surface-2)] text-soft hover:border-line-strong hover:bg-[var(--surface-3)] hover:text-ink",
    ghost: "text-muted hover:bg-[color-mix(in_srgb,var(--muted)_10%,transparent)] hover:text-ink",
    danger:
      "border border-[color-mix(in_srgb,var(--danger)_26%,transparent)] bg-[var(--danger-soft)] text-[var(--danger)] hover:bg-[color-mix(in_srgb,var(--danger)_18%,transparent)]",
    soft: "border border-[var(--accent-line)] bg-[var(--accent-soft)] text-[var(--accent)] hover:bg-[color-mix(in_srgb,var(--accent)_18%,transparent)]",
  };
  const style =
    variant === "primary"
      ? { backgroundImage: "linear-gradient(135deg, var(--accent), var(--accent-2))" }
      : undefined;

  return (
    <button
      ref={ref}
      data-variant={variant}
      className={cn(base, BUTTON_SIZES[size], variants[variant], className)}
      style={style}
      disabled={disabled || loading}
      {...props}
    >
      {loading ? <Loader2 className="size-4 animate-spin" /> : icon}
      {children}
    </button>
  );
});

type IconButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  label: string;
  size?: "sm" | "md";
};

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(function IconButton(
  { label, size = "md", className, children, ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      type="button"
      aria-label={label}
      title={label}
      className={cn(
        "relative grid place-items-center rounded-[7px] text-muted transition-all duration-200 hover:bg-[color-mix(in_srgb,var(--muted)_10%,transparent)] hover:text-ink active:scale-95 disabled:pointer-events-none disabled:opacity-40",
        size === "md" ? "size-9" : "size-8",
        className,
      )}
      {...props}
    >
      {children}
    </button>
  );
});

/* -------------------------------------------------------------------------- */
/* Badge                                                                       */
/* -------------------------------------------------------------------------- */

type Tone = "success" | "warning" | "danger" | "info" | "violet" | "accent" | "neutral";

const TONE_VAR: Record<Tone, string> = {
  success: "--success",
  warning: "--warning",
  danger: "--danger",
  info: "--info",
  violet: "--violet",
  accent: "--accent",
  neutral: "--muted",
};

export function Badge({
  tone = "neutral",
  dot = false,
  children,
  className,
  pulse = false,
}: {
  tone?: Tone;
  dot?: boolean;
  children: ReactNode;
  className?: string;
  pulse?: boolean;
}) {
  const variable = TONE_VAR[tone];
  return (
    <span
      className={cn(
        "inline-flex h-[22px] items-center gap-1.5 rounded-full border px-2 text-[11px] font-semibold tracking-[0.01em]",
        className,
      )}
      style={{
        color: `var(${variable})`,
        backgroundColor: `color-mix(in srgb, var(${variable}) 11%, transparent)`,
        borderColor: `color-mix(in srgb, var(${variable}) 24%, transparent)`,
      }}
    >
      {dot ? (
        <span
          className={cn("relative flex size-1.5 rounded-full", pulse && "animate-pulse")}
          style={{ backgroundColor: `var(${variable})` }}
        />
      ) : null}
      {children}
    </span>
  );
}

/* -------------------------------------------------------------------------- */
/* Card                                                                        */
/* -------------------------------------------------------------------------- */

type CardProps = HTMLAttributes<HTMLDivElement> & {
  as?: ElementType;
  hover?: boolean;
  padded?: boolean;
};

export function Card({ as: Tag = "div", hover, padded, className, children, ...props }: CardProps) {
  return (
    <Tag
      className={cn("card", hover && "card-hover", padded && "card-pad", className)}
      {...props}
    >
      {children}
    </Tag>
  );
}

/* -------------------------------------------------------------------------- */
/* Progress                                                                    */
/* -------------------------------------------------------------------------- */

export function Progress({
  value,
  tone,
  className,
  trackClassName,
  height = 6,
  striped = false,
  label,
}: {
  value: number;
  tone?: string;
  className?: string;
  trackClassName?: string;
  height?: number;
  striped?: boolean;
  label?: string;
}) {
  const { t } = useI18n();
  const safe = Math.min(100, Math.max(0, value));
  const background = tone ?? "linear-gradient(90deg, var(--accent), var(--accent-2))";
  return (
    <div
      className={cn("w-full overflow-hidden rounded-full bg-[var(--surface-3)]", trackClassName, className)}
      style={{ height }}
      role="progressbar"
      aria-label={label ?? t("Usage")}
      aria-valuenow={Math.round(safe)}
      aria-valuemin={0}
      aria-valuemax={100}
    >
      <div
        className={cn("h-full rounded-full transition-[width] duration-700 ease-out", striped && "shimmer-line")}
        style={{ width: `${safe}%`, background }}
      />
    </div>
  );
}

/* -------------------------------------------------------------------------- */
/* Switch                                                                      */
/* -------------------------------------------------------------------------- */

export function Switch({
  checked,
  onChange,
  label,
  disabled,
}: {
  checked: boolean;
  onChange: (value: boolean) => void;
  label: string;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={cn(
        "relative inline-flex h-[22px] w-[38px] shrink-0 items-center rounded-full border transition-colors duration-200 disabled:opacity-40",
        checked
          ? "border-transparent"
          : "border-line-strong bg-[var(--surface-3)]",
      )}
      style={checked ? { background: "linear-gradient(135deg, var(--accent), var(--accent-2))" } : undefined}
    >
      <span
        className={cn(
          "block size-[16px] rounded-full bg-white shadow-sm transition-transform duration-200",
          checked ? "translate-x-[19px]" : "translate-x-[3px]",
        )}
      />
    </button>
  );
}

/* -------------------------------------------------------------------------- */
/* Tabs                                                                        */
/* -------------------------------------------------------------------------- */

export type TabItem = { value: string; label: string; count?: number; icon?: ReactNode };

export function Tabs({
  items,
  value,
  onChange,
  size = "md",
  className,
}: {
  items: TabItem[];
  value: string;
  onChange: (value: string) => void;
  size?: "sm" | "md";
  className?: string;
}) {
  return (
    <div
      role="tablist"
      className={cn(
        "inline-flex max-w-full items-center gap-0.5 overflow-x-auto rounded-xl border border-line bg-[var(--surface-2)] p-1",
        className,
      )}
    >
      {items.map((item) => {
        const active = item.value === value;
        return (
          <button
            key={item.value}
            role="tab"
            aria-selected={active}
            type="button"
            onClick={() => onChange(item.value)}
            className={cn(
              "inline-flex shrink-0 items-center gap-1.5 rounded-lg font-medium transition-all duration-200",
              size === "sm" ? "h-7 px-2.5 text-xs" : "h-8 px-2.5 text-[11.5px] sm:px-3 sm:text-[12.5px]",
              active
                ? "bg-[var(--surface)] text-ink shadow-[0_1px_2px_rgba(0,0,0,0.12)] ring-1 ring-line"
                : "text-muted hover:text-ink",
            )}
          >
            {item.icon}
            {item.label}
            {item.count !== undefined ? (
              <span
                className={cn(
                  "rounded-full px-1.5 py-px font-mono text-[10px]",
                  active ? "bg-[var(--surface-3)] text-soft" : "text-subtle",
                )}
              >
                {item.count}
              </span>
            ) : null}
          </button>
        );
      })}
    </div>
  );
}

/* -------------------------------------------------------------------------- */
/* Misc                                                                        */
/* -------------------------------------------------------------------------- */

export function Kbd({ children }: { children: ReactNode }) {
  return (
    <kbd className="inline-flex h-5 min-w-5 items-center justify-center rounded border border-line bg-[var(--surface-2)] px-1 font-mono text-[10px] font-medium text-subtle">
      {children}
    </kbd>
  );
}

export function Avatar({
  name,
  color,
  size = 32,
  className,
}: {
  name: string;
  color?: string;
  size?: number;
  className?: string;
}) {
  const letters = name
    .replace(/[^a-zA-Z0-9 ]/g, " ")
    .trim()
    .split(/\s+/)
    .map((part) => part[0])
    .slice(0, 2)
    .join("")
    .toUpperCase();
  const tint = color ?? "var(--accent)";
  return (
    <span
      className={cn(
        "inline-grid shrink-0 place-items-center rounded-full border font-semibold",
        className,
      )}
      style={{
        width: size,
        height: size,
        fontSize: size * 0.36,
        color: tint,
        backgroundColor: `color-mix(in srgb, ${tint} 14%, transparent)`,
        borderColor: `color-mix(in srgb, ${tint} 26%, transparent)`,
      }}
    >
      {letters || "?"}
    </span>
  );
}

export function Skeleton({ className }: { className?: string }) {
  return <div className={cn("skeleton rounded-lg", className)} />;
}

export function EmptyState({
  icon,
  title,
  description,
  action,
  className,
}: {
  icon: ReactNode;
  title: string;
  description?: string;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-col items-center justify-center px-6 py-14 text-center", className)}>
      <div className="mb-4 grid size-12 place-items-center rounded-2xl border border-line bg-[var(--surface-2)] text-muted">
        {icon}
      </div>
      <p className="text-sm font-semibold text-ink">{title}</p>
      {description ? <p className="mt-1 max-w-sm text-xs leading-relaxed text-muted">{description}</p> : null}
      {action ? <div className="mt-5">{action}</div> : null}
    </div>
  );
}

export function Tooltip({
  content,
  children,
  side = "top",
}: {
  content: ReactNode;
  children: ReactNode;
  side?: "top" | "bottom";
}) {
  const id = useId();
  return (
    <span className="group/tooltip relative inline-flex">
      <span aria-describedby={id}>{children}</span>
      <span
        id={id}
        role="tooltip"
        className={cn(
          "pointer-events-none absolute left-1/2 z-50 -translate-x-1/2 whitespace-nowrap rounded-md border border-line bg-[var(--surface-inverse)] px-2 py-1 text-[10.5px] font-medium text-[var(--bg)] opacity-0 shadow-lg transition-all duration-150 group-hover/tooltip:opacity-100",
          side === "top" ? "bottom-[calc(100%+6px)] translate-y-1 group-hover/tooltip:translate-y-0" : "top-[calc(100%+6px)] -translate-y-1 group-hover/tooltip:translate-y-0",
        )}
      >
        {content}
      </span>
    </span>
  );
}

export function LinkArrow({ children }: { children: ReactNode }) {
  return (
    <span className="inline-flex items-center gap-0.5 text-xs font-medium text-accent transition-transform duration-200 group-hover:translate-x-0.5">
      {children}
      <ChevronRight className="size-3.5" />
    </span>
  );
}

export function useDisclosure(initial = false) {
  const [open, setOpen] = useState(initial);
  return { open, onOpen: () => setOpen(true), onClose: () => setOpen(false), toggle: () => setOpen((v) => !v) };
}

export function SectionTitle({
  title,
  hint,
  action,
}: {
  title: string;
  hint?: string;
  action?: ReactNode;
}) {
  return (
    <div className="mb-4 flex items-end justify-between gap-4">
      <div>
        <h2 className="text-[15px] font-semibold text-ink">{title}</h2>
        {hint ? <p className="mt-0.5 text-xs text-muted">{hint}</p> : null}
      </div>
      {action}
    </div>
  );
}

/* -------------------------------------------------------------------------- */
/* Input                                                                       */
/* -------------------------------------------------------------------------- */

export function InputBase({
  value,
  onChange,
  placeholder,
  icon,
  className,
  type = "text",
  suffix,
  autoFocus,
  name,
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  icon?: ReactNode;
  className?: string;
  type?: string;
  suffix?: ReactNode;
  autoFocus?: boolean;
  name?: string;
}) {
  return (
    <div
      className={cn(
        "flex h-9 items-center gap-2 rounded-[7px] border border-line bg-[var(--surface-2)] px-3 transition focus-within:border-[var(--accent-line)] focus-within:bg-[var(--surface)]",
        className,
      )}
    >
      {icon ? <span className="shrink-0 text-subtle">{icon}</span> : null}
      <input
        name={name}
        type={type}
        value={value}
        autoFocus={autoFocus}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-full w-full min-w-0 border-0 bg-transparent text-[12px] text-ink outline-none placeholder:text-subtle"
      />
      {suffix}
    </div>
  );
}
