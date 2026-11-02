import { Link } from "react-router-dom";
import { ArrowLeft, Compass } from "lucide-react";
import { AuroraMark } from "../components/layout/Brand";
import { Button } from "../components/ui/primitives";
import { useI18n } from "../i18n";

export function NotFoundPage() {
  const { t } = useI18n();
  return (
    <div className="relative grid min-h-dvh place-items-center overflow-hidden bg-[var(--bg)] px-6">
      <div className="app-atmosphere" />
      <div className="relative text-center">
        <div className="mb-8 flex justify-center">
          <AuroraMark size={44} />
        </div>
        <p className="font-mono text-[76px] font-semibold leading-none tracking-[-0.06em] text-ink/10 sm:text-[112px]">404</p>
        <div className="-mt-6 sm:-mt-9">
          <h1 className="text-xl font-semibold text-ink">{t("This page drifted out of orbit")}</h1>
          <p className="mx-auto mt-2 max-w-sm text-xs leading-relaxed text-muted">
            {t("The route you are looking for does not exist, or it was moved while you were away.")}
          </p>
          <div className="mt-6 flex justify-center gap-2">
            <Link
              to="/"
              className="inline-flex h-9 items-center gap-2 rounded-[7px] px-3.5 text-[13px] font-medium text-[var(--accent-contrast)] shadow-[0_8px_24px_-12px_var(--accent)]"
              style={{ backgroundImage: "linear-gradient(135deg, var(--accent), var(--accent-2))" }}
            >
              <ArrowLeft className="size-3.5" /> {t("Back to dashboard")}
            </Link>
            <Button variant="secondary" icon={<Compass className="size-3.5" />} onClick={() => (window.location.href = "/storage")}>
              {t("Explore storage")}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
