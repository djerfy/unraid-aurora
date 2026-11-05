import { useEffect, useRef, useState } from "react";
import { Check, Copy, Download, RefreshCw, Terminal } from "lucide-react";
import { IconButton, cn } from "./primitives";
import { useI18n } from "../../i18n";
import type { LogLine } from "../../lib/types";

type LogViewerProps = {
  lines?: LogLine[];
  loading?: boolean;
  refreshing?: boolean;
  onRefresh: () => void;
  downloadName: string;
  title: string;
  heightClass?: string;
};

/**
 * Builds the canonical log view for containers and virtual machines: a
 * timestamps + level + message list, a text filter, and copy/download actions.
 */
export function LogViewer({
  lines,
  loading = false,
  refreshing = false,
  onRefresh,
  downloadName,
  title,
  heightClass = "h-[32rem]",
}: LogViewerProps) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const [copied, setCopied] = useState(false);
  const boxRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!lines || !boxRef.current) return;
    boxRef.current.scrollTop = boxRef.current.scrollHeight;
  }, [lines]);

  const filtered = (lines ?? []).filter((line) => {
    const value = query.trim().toLowerCase();
    return !value || `${line.level} ${line.message}`.toLowerCase().includes(value);
  });

  const text = () => filtered.map((line) => `${line.time} ${line.level.toUpperCase()} ${line.message}`).join("\n");

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text());
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1800);
    } catch {
      // Clipboard access can be denied; the log stays selectable.
    }
  };

  const download = () => {
    const blob = new Blob([text()], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = downloadName;
    link.click();
    window.setTimeout(() => URL.revokeObjectURL(url), 1000);
  };

  return (
    <div>
      <div className="flex items-center justify-between gap-3">
        <h3 className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-subtle">
          <Terminal className="size-3.5" /> {title}
        </h3>
        <div className="flex items-center gap-1">
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={t("Filter logs…")}
            aria-label={t("Filter logs")}
            className="h-7 w-36 rounded-md border border-line bg-[var(--surface-2)] px-2 text-[10px] text-ink outline-none focus:border-[var(--accent-line)]"
          />
          <IconButton label={t("Reload logs")} size="sm" onClick={onRefresh}>
            <RefreshCw className={cn("size-3.5", refreshing && "animate-spin")} />
          </IconButton>
          <IconButton label={t("Download logs")} size="sm" onClick={download}>
            <Download className="size-3.5" />
          </IconButton>
          <IconButton label={t("Copy logs")} size="sm" onClick={() => void copy()}>
            {copied ? <Check className="size-3.5 text-[var(--success)]" /> : <Copy className="size-3.5" />}
          </IconButton>
        </div>
      </div>
      <div
        ref={boxRef}
        className={cn(
          "mt-2 overflow-auto rounded-xl border border-[#23252e] bg-[#08090d] p-3 font-mono text-[10px] leading-relaxed",
          heightClass,
        )}
      >
        {loading ? (
          <p className="text-[#6b7280]">{t("Loading logs…")}</p>
        ) : filtered.length ? (
          filtered.map((line, index) => (
            <div key={`${line.time}-${index}`} className="flex gap-2">
              <span className="shrink-0 text-[#4b5563]">
                {new Date(line.time).toLocaleTimeString([], { hour12: false })}
              </span>
              <span
                className={cn(
                  "shrink-0 uppercase",
                  line.level === "error"
                    ? "text-[#ff6b81]"
                    : line.level === "warn"
                      ? "text-[#f5b44a]"
                      : "text-[#5aa9ff]",
                )}
              >
                {line.level}
              </span>
              <span className="min-w-0 whitespace-pre-wrap break-words text-[#c7cbd6]">{line.message}</span>
            </div>
          ))
        ) : (
          <p className="text-[#6b7280]">
            {query.trim() ? t("No log line matches this filter.") : t("No logs available.")}
          </p>
        )}
      </div>
    </div>
  );
}
