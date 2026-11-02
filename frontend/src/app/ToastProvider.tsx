import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { CheckCircle2, Info, TriangleAlert, X, XCircle } from "lucide-react";
import { useI18n } from "../i18n";

type ToastKind = "success" | "error" | "info" | "warning";

type ToastItem = {
  id: number;
  kind: ToastKind;
  title: string;
  description?: string;
};

type ToastInput = { title: string; description?: string; kind?: ToastKind; duration?: number };

type ToastContextValue = {
  toast: (input: ToastInput) => void;
};

const ToastContext = createContext<ToastContextValue | null>(null);

const ICONS = {
  success: CheckCircle2,
  error: XCircle,
  warning: TriangleAlert,
  info: Info,
};

const COLORS: Record<ToastKind, string> = {
  success: "text-[var(--success)]",
  error: "text-[var(--danger)]",
  warning: "text-[var(--warning)]",
  info: "text-[var(--info)]",
};

export function ToastProvider({ children }: { children: ReactNode }) {
  const { t } = useI18n();
  const [items, setItems] = useState<ToastItem[]>([]);
  const nextId = useRef(1);

  const dismiss = useCallback((id: number) => {
    setItems((prev) => prev.filter((item) => item.id !== id));
  }, []);

  const toast = useCallback(
    ({ title, description, kind = "info", duration = 4200 }: ToastInput) => {
      const id = nextId.current++;
      setItems((prev) => [...prev.slice(-2), { id, kind, title, description }]);
      window.setTimeout(() => dismiss(id), duration);
    },
    [dismiss],
  );

  const value = useMemo(() => ({ toast }), [toast]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="pointer-events-none fixed bottom-5 right-5 z-[90] flex w-[calc(100vw-2.5rem)] max-w-sm flex-col gap-2.5">
        {items.map((item) => {
          const Icon = ICONS[item.kind];
          return (
            <div
              key={item.id}
              role="status"
              className="animate-toast-in pointer-events-auto relative overflow-hidden rounded-2xl border border-line bg-[var(--surface)] p-4 shadow-[var(--shadow-pop)]"
            >
              <div className="flex gap-3">
                <Icon className={`mt-0.5 size-[18px] shrink-0 ${COLORS[item.kind]}`} strokeWidth={2} />
                <div className="min-w-0 flex-1">
                  <p className="text-[13.5px] font-semibold text-ink">{item.title}</p>
                  {item.description ? (
                    <p className="mt-0.5 text-xs leading-relaxed text-muted">{item.description}</p>
                  ) : null}
                </div>
                <button
                  type="button"
                  aria-label={t("Dismiss notification")}
                  onClick={() => dismiss(item.id)}
                  className="-mr-1 -mt-1 grid size-6 place-items-center rounded-md text-subtle transition hover:bg-[var(--surface-3)] hover:text-ink"
                >
                  <X className="size-3.5" />
                </button>
              </div>
              <div className="absolute inset-x-0 bottom-0 h-px bg-line">
                <div className="h-px w-full origin-left bg-[var(--accent)]/60" />
              </div>
            </div>
          );
        })}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used inside ToastProvider");
  return ctx;
}
