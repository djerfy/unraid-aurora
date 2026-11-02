import type { ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  ArrowUpRight,
  BookOpen,
  Github,
  Heart,
  Scale,
  Server,
  Sparkles,
  UserRound,
} from "lucide-react";
import { PageHeader } from "../components/layout/PageHeader";
import { AuroraMark } from "../components/layout/Brand";
import { Badge, Card } from "../components/ui/primitives";
import { useApp } from "../app/AppDataProvider";
import { useI18n } from "../i18n";
import { api } from "../lib/api";

const REPO = "https://github.com/djerfy/unraid-aurora";
const AUTHOR = "https://github.com/djerfy";
const LICENSE = "https://github.com/djerfy/unraid-aurora/blob/main/LICENSE";

function ExternalLink({ href, icon, children }: { href: string; icon: ReactNode; children: ReactNode }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className="group flex items-center justify-between gap-3 rounded-xl border border-line bg-[var(--surface-2)]/40 px-3 py-2.5 text-[11px] text-soft transition hover:border-line-strong hover:bg-[var(--surface-2)] hover:text-ink"
    >
      <span className="flex min-w-0 items-center gap-2">
        <span className="grid size-7 shrink-0 place-items-center rounded-lg border border-line bg-[var(--surface)] text-muted">
          {icon}
        </span>
        <span className="truncate">{children}</span>
      </span>
      <ArrowUpRight className="size-3.5 shrink-0 text-subtle transition group-hover:text-ink" />
    </a>
  );
}

function AboutRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 border-b border-line/60 py-1.5 last:border-0">
      <span className="text-[10px] text-subtle">{label}</span>
      <span className="num truncate font-mono text-[10px] text-soft">{value}</span>
    </div>
  );
}

export function AboutPage() {
  const { snapshot } = useApp();
  const { t } = useI18n();
  const health = useQuery({
    queryKey: ["server-health"],
    queryFn: api.serverHealth,
    staleTime: 60_000,
  });

  return (
    <>
      <PageHeader
        eyebrow={t("System")}
        title={t("About Aurora")}
        description={t("A modern, independent control plane for Unraid.")}
        meta={
          <>
            <Badge tone="success" dot>{t("Open source")}</Badge>
            <span className="chip">{t("License AGPL-3.0")}</span>
            {health.data?.version ? <span className="chip">Aurora {health.data.version}</span> : null}
          </>
        }
      />

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.45fr)_minmax(0,1fr)] xl:items-start">
        <div className="space-y-4">
          <Card className="card-pad">
            <div className="flex flex-col gap-5 sm:flex-row sm:items-start">
              <span className="grid size-14 shrink-0 place-items-center rounded-2xl border border-line bg-[var(--surface-2)]">
                <AuroraMark size={34} />
              </span>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <h2 className="text-[17px] font-semibold tracking-[-0.02em] text-ink">Aurora for Unraid</h2>
                  <Badge tone="neutral">Go · React</Badge>
                </div>
                <p className="mt-2 text-[11.5px] leading-relaxed text-muted">
                  {t(
                    "Aurora is an independent control plane for Unraid. It reads the real host state — array, disks, shares, Docker, virtual machines, network, UPS and telemetry — and presents it through a fast Go API and a React interface.",
                  )}
                </p>
                <p className="mt-2 text-[11.5px] leading-relaxed text-muted">
                  {t(
                    "It is a visual and operational companion, not a replacement for Unraid. Disk assignments, array operations and host configuration stay within Unraid's own rules.",
                  )}
                </p>
                <div className="mt-4 flex flex-wrap gap-1.5">
                  {["Go", "React 19", "TypeScript", "Tailwind CSS", "SSE", "AGPL-3.0"].map((item) => (
                    <span key={item} className="chip">{item}</span>
                  ))}
                </div>
              </div>
            </div>

            <div className="mt-5 grid grid-cols-2 gap-x-4 gap-y-2 border-t border-line pt-4 sm:grid-cols-3">
              {["Array", "Shares", "Containers", "Virtual machines", "Network", "GPUs", "UPS", "Logs", "History"].map((item) => (
                <span key={item} className="flex items-center gap-2 text-[10px] text-muted">
                  <span className="size-1 rounded-full bg-[color-mix(in_srgb,var(--accent)_70%,transparent)]" />
                  {t(item)}
                </span>
              ))}
            </div>

            <div className="mt-5 grid gap-3 border-t border-line pt-4 sm:grid-cols-3">
              <div className="flex items-start gap-2">
                <Sparkles className="mt-0.5 size-3.5 shrink-0 text-[var(--accent)]" />
                <div>
                  <p className="text-[10.5px] font-semibold text-soft">{t("Live host only")}</p>
                  <p className="mt-0.5 text-[9.5px] leading-relaxed text-subtle">
                    {t("Real Unraid state, never a demo fixture.")}
                  </p>
                </div>
              </div>
              <div className="flex items-start gap-2">
                <Server className="mt-0.5 size-3.5 shrink-0 text-[var(--info)]" />
                <div>
                  <p className="text-[10.5px] font-semibold text-soft">{t("Single binary")}</p>
                  <p className="mt-0.5 text-[9.5px] leading-relaxed text-subtle">
                    {t("The React UI is embedded in the Go server.")}
                  </p>
                </div>
              </div>
              <div className="flex items-start gap-2">
                <Heart className="mt-0.5 size-3.5 shrink-0 text-[var(--violet)]" />
                <div>
                  <p className="text-[10.5px] font-semibold text-soft">{t("Made for homelabs")}</p>
                  <p className="mt-0.5 text-[9.5px] leading-relaxed text-subtle">
                    {t("Calm, dense and readable by design.")}
                  </p>
                </div>
              </div>
            </div>
          </Card>

          <Card className="card-pad">
            <h3 className="text-[11px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("Runtime")}</h3>
            <div className="mt-2">
              <AboutRow label={t("Aurora version")} value={health.data?.version ?? "—"} />
              <AboutRow label={t("Unraid version")} value={snapshot?.system.version ?? "—"} />
              <AboutRow label={t("API provider")} value={health.data?.provider ?? "—"} />
              <AboutRow label={t("Host")} value={snapshot?.system.hostname ?? "—"} />
            </div>
          </Card>

          <Card className="card-pad">
            <h3 className="text-[11px] font-semibold uppercase tracking-[0.12em] text-subtle">{t("Independence")}</h3>
            <p className="mt-2 text-[10.5px] leading-relaxed text-muted">
              {t(
                "Aurora is not affiliated with Lime Technology. Unraid is a trademark of Lime Technology, Inc. This project is an independent redevelopment study based on the public Unraid webGUI source.",
              )}
            </p>
            <p className="mt-2 text-[9.5px] leading-relaxed text-subtle">
              {t("Built with respect for the Unraid operational model, its state files and its safety rules.")}
            </p>
          </Card>
        </div>

        <div className="space-y-4">
          <Card className="card-pad">
            <h3 className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-subtle">
              <Github className="size-3.5" /> {t("Project")}
            </h3>
            <div className="mt-3 space-y-2">
              <ExternalLink href={REPO} icon={<Github className="size-3.5" />}>
                {t("Repository")} · djerfy/unraid-aurora
              </ExternalLink>
              <ExternalLink href={`${REPO}/issues`} icon={<BookOpen className="size-3.5" />}>
                {t("Issues and feature requests")}
              </ExternalLink>
              <ExternalLink href={`${REPO}/releases`} icon={<ArrowUpRight className="size-3.5" />}>
                {t("Latest release")}
              </ExternalLink>
            </div>
          </Card>

          <Card className="card-pad">
            <h3 className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-subtle">
              <UserRound className="size-3.5" /> {t("Author")}
            </h3>
            <p className="mt-3 text-[11.5px] leading-relaxed text-muted">
              {t("Designed and developed by Djerfy.")}
            </p>
            <div className="mt-3">
              <ExternalLink href={AUTHOR} icon={<Github className="size-3.5" />}>
                github.com/djerfy
              </ExternalLink>
            </div>
          </Card>

          <Card className="card-pad">
            <h3 className="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.12em] text-subtle">
              <Scale className="size-3.5" /> {t("License")}
            </h3>
            <p className="mt-3 text-[11.5px] leading-relaxed text-muted">
              {t("Aurora is released under the GNU Affero General Public License v3.0.")}
            </p>
            <div className="mt-3">
              <ExternalLink href={LICENSE} icon={<Scale className="size-3.5" />}>
                {t("AGPL-3.0-only")}
              </ExternalLink>
            </div>
          </Card>
        </div>
      </div>
    </>
  );
}
