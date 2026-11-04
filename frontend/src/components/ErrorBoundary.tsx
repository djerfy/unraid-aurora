import { Component, type ErrorInfo, type ReactNode } from "react";
import { AlertTriangle, RefreshCw } from "lucide-react";
import { Button } from "./ui/primitives";
import { useI18n } from "../i18n";

type Props = { children: ReactNode };
type State = { error: Error | null };

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("Aurora view crashed", error, info.componentStack);
  }

  render() {
    if (this.state.error) {
      return <ErrorFallback error={this.state.error} onReset={() => this.setState({ error: null })} />;
    }
    return this.props.children;
  }
}

function ErrorFallback({ error, onReset }: { error: Error; onReset: () => void }) {
  const { t } = useI18n();
  return (
    <div className="grid min-h-[60vh] place-items-center px-6 py-16">
      <div className="max-w-lg rounded-2xl border border-line bg-[var(--surface)] p-6 text-center shadow-[var(--shadow-pop)]">
        <span className="mx-auto grid size-12 place-items-center rounded-2xl border border-[var(--danger)]/25 bg-[var(--danger-soft)] text-[var(--danger)]">
          <AlertTriangle className="size-5" />
        </span>
        <h2 className="mt-4 text-[16px] font-semibold text-ink">{t("This view encountered an error")}</h2>
        <p className="mt-2 text-[11px] leading-relaxed text-muted">{t("Aurora could not render this page. The error was logged in the browser console; no settings were changed.")}</p>
        <pre className="mt-3 max-h-32 overflow-auto rounded-lg border border-line bg-[var(--surface-2)] p-2.5 text-left font-mono text-[9.5px] text-subtle">{error.message}</pre>
        <div className="mt-4 flex justify-center gap-2">
          <Button variant="secondary" onClick={onReset}>{t("Try again")}</Button>
          <Button variant="primary" icon={<RefreshCw className="size-3.5" />} onClick={() => window.location.reload()}>{t("Reload the page")}</Button>
        </div>
      </div>
    </div>
  );
}
