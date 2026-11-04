import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Boxes,
  Bug,
  ChevronDown,
  Container as ContainerIcon,
  ExternalLink,
  Layers,
  Package,
  RefreshCw,
  Search,
  ShieldAlert,
  ShieldCheck,
} from "lucide-react";
import { useI18n, type TranslationParams } from "../../i18n";
import { api } from "../../lib/api";
import { Badge, Button, Card, EmptyState, IconButton, InputBase, Tabs, cn } from "../ui/primitives";
import { EntityIcon, containerIconURL } from "../ui/entity-icon";
import { SecurityCounts, severityKey, SeverityBadge, type SeverityKey } from "./security";
import type {
  Container,
  ContainerSecurity,
  ContainerVulnerability,
  SecurityContainerRef,
  SecurityImage,
  SecurityOverview,
} from "../../lib/types";

type GroupBy = "image" | "container" | "cve" | "package" | "severity";
type SeverityFilter = "all" | SeverityKey;
type Translate = (key: string, params?: TranslationParams) => string;

type ImageRef = { image: string; imageId?: string };

type AggregatedFinding = ContainerVulnerability & {
  key: string;
  images: ImageRef[];
  containers: SecurityContainerRef[];
};

type SecurityGroup = {
  id: string;
  kind: GroupBy;
  label: string;
  subtitle: string;
  security: ContainerSecurity;
  findings: AggregatedFinding[];
  images: ImageRef[];
  containers: SecurityContainerRef[];
  showLocations: boolean;
  recompute: boolean;
  container?: SecurityContainerRef;
};

type RawGroup = {
  id: string;
  kind: GroupBy;
  label: string;
  subtitle: string;
  security: ContainerSecurity;
  occurrences: Map<string, Occurrence>;
  images: Map<string, ImageRef>;
  containers: Map<string, SecurityContainerRef>;
  showLocations: boolean;
  recompute: boolean;
  container?: SecurityContainerRef;
};

type Occurrence = {
  key: string;
  finding: ContainerVulnerability;
  images: Map<string, ImageRef>;
  containers: Map<string, SecurityContainerRef>;
};

function imageRefKey(image: SecurityImage): string {
  return image.imageId || image.image;
}

function occurrenceKey(finding: ContainerVulnerability): string {
  return [finding.id, finding.package, finding.installedVersion, finding.fixedVersion, finding.target].join("|");
}

function emptySecurity(): ContainerSecurity {
  return {
    enabled: true, available: true, scanning: false, stale: false,
    critical: 0, high: 0, medium: 0, low: 0, unknown: 0, total: 0,
  };
}

function securityFromFindings(findings: AggregatedFinding[]): ContainerSecurity {
  const security = emptySecurity();
  for (const finding of findings) {
    security[severityKey(finding.severity)] += 1;
    security.total += 1;
  }
  return security;
}

function createRawGroup(
  kind: GroupBy,
  id: string,
  label: string,
  subtitle: string,
  security: ContainerSecurity,
  showLocations: boolean,
  recompute: boolean,
  container?: SecurityContainerRef,
): RawGroup {
  return {
    id, kind, label, subtitle, security, showLocations, recompute, container,
    occurrences: new Map(), images: new Map(), containers: new Map(),
  };
}

function addOccurrence(group: RawGroup, finding: ContainerVulnerability, image: SecurityImage, containers: SecurityContainerRef[]) {
  const key = occurrenceKey(finding);
  let occurrence = group.occurrences.get(key);
  if (!occurrence) {
    occurrence = { key, finding, images: new Map(), containers: new Map() };
    group.occurrences.set(key, occurrence);
  }
  const imageKey = imageRefKey(image);
  occurrence.images.set(imageKey, { image: image.image, imageId: image.imageId });
  group.images.set(imageKey, { image: image.image, imageId: image.imageId });
  for (const container of containers) {
    occurrence.containers.set(container.id, container);
    group.containers.set(container.id, container);
  }
}

function countLabel(t: Translate, count: number, singular: string, plural: string): string {
  return count === 1 ? t(singular) : t(plural, { count });
}

function finalizeGroup(raw: RawGroup, t: Translate): SecurityGroup {
  const findings = [...raw.occurrences.values()].map((occurrence) => ({
    ...occurrence.finding,
    key: occurrence.key,
    images: [...occurrence.images.values()],
    containers: [...occurrence.containers.values()],
  }));
  const images = [...raw.images.values()];
  const containers = [...raw.containers.values()];
  const security = raw.recompute ? securityFromFindings(findings) : raw.security;
  let subtitle = raw.subtitle;
  if (raw.kind === "package" || raw.kind === "severity") {
    subtitle = [
      countLabel(t, findings.length, "1 finding", "{count} findings"),
      countLabel(t, images.length, "1 image", "{count} images"),
      countLabel(t, containers.length, "1 container", "{count} containers"),
    ].join(" · ");
  }
  return {
    id: raw.id, kind: raw.kind, label: raw.label, subtitle, security, findings,
    images, containers, showLocations: raw.showLocations, recompute: raw.recompute,
    container: raw.container,
  };
}

function buildSecurityGroups(images: SecurityImage[], groupBy: GroupBy, t: Translate): SecurityGroup[] {
  if (groupBy === "image") {
    return images.map((image) => {
      const containers = image.containers ?? [];
      const group = createRawGroup(
        "image",
        `image:${imageRefKey(image)}`,
        image.image,
        containers.map((container) => container.name).join(" · ") || t("No container"),
        image.security,
        false,
        false,
      );
      for (const finding of image.vulnerabilities ?? []) {
        addOccurrence(group, finding, image, containers);
      }
      return finalizeGroup(group, t);
    });
  }

  const groups = new Map<string, RawGroup>();
  if (groupBy === "container") {
    for (const image of images) {
      for (const container of image.containers ?? []) {
        let group = groups.get(container.id);
        if (!group) {
          group = createRawGroup(
            "container",
            `container:${container.id}`,
            container.name,
            image.image,
            image.security,
            false,
            false,
            container,
          );
          groups.set(container.id, group);
        }
        for (const finding of image.vulnerabilities ?? []) {
          addOccurrence(group, finding, image, [container]);
        }
      }
    }
  } else {
    for (const image of images) {
      for (const finding of image.vulnerabilities ?? []) {
        const key =
          groupBy === "cve"
            ? finding.id || t("Unknown")
            : groupBy === "package"
              ? finding.package || t("Unknown")
              : severityKey(finding.severity);
        let group = groups.get(key);
        if (!group) {
          if (groupBy === "cve") {
            group = createRawGroup(
              "cve", `cve:${key}`, key, finding.title || finding.target || t("No description"),
              emptySecurity(), true, true,
            );
          } else if (groupBy === "package") {
            group = createRawGroup("package", `package:${key}`, key, "", emptySecurity(), true, true);
          } else {
            group = createRawGroup(
              "severity", `severity:${key}`, t(severityLabel(severityKey(finding.severity))), "", emptySecurity(), true, true,
            );
          }
          groups.set(key, group);
        }
        addOccurrence(group, finding, image, image.containers);
      }
    }
  }
  return [...groups.values()].map((group) => finalizeGroup(group, t));
}

function severityLabel(key: SeverityKey): string {
  return key === "critical" ? "Critical" : key === "high" ? "High" : key === "medium" ? "Medium" : key === "low" ? "Low" : "Unknown";
}

function groupRisk(group: SecurityGroup): number {
  const security = group.security;
  return security.critical * 1_000_000_000 +
    security.high * 10_000_000 +
    security.medium * 100_000 +
    security.low * 1_000 +
    security.unknown;
}

function groupPriority(group: SecurityGroup): number {
  if (group.security.error) return 0;
  if (group.security.total > 0) return 1;
  if (group.security.scanning) return 2;
  if (!group.security.available) return 3;
  return 4;
}

function matchesFinding(finding: AggregatedFinding, query: string): boolean {
  return [
    finding.id, finding.package, finding.installedVersion, finding.fixedVersion,
    finding.severity, finding.title, finding.target, finding.type,
  ].join(" ").toLowerCase().includes(query);
}

function filterGroup(group: SecurityGroup, query: string, severity: SeverityFilter, showWhenEmpty: boolean): SecurityGroup | null {
  const needle = query.trim().toLowerCase();
  const headerMatches = !needle || [
    group.label, group.subtitle,
    ...group.images.map((image) => image.image),
    ...group.containers.map((container) => container.name),
  ].some((value) => value.toLowerCase().includes(needle));

  let findings = group.findings;
  if (severity !== "all") {
    findings = findings.filter((finding) => severityKey(finding.severity) === severity);
  }
  if (needle && !headerMatches) {
    findings = findings.filter((finding) => matchesFinding(finding, needle));
  }
  if (findings.length === 0) {
    if (showWhenEmpty && severity === "all" && (!needle || headerMatches)) {
      return { ...group, findings };
    }
    return null;
  }
  const security = group.recompute ? securityFromFindings(findings) : group.security;
  return { ...group, findings, security };
}

function groupIcon(kind: GroupBy): ReactNode {
  switch (kind) {
    case "image":
      return <Boxes className="size-3.5" />;
    case "container":
      return <ContainerIcon className="size-3.5" />;
    case "cve":
      return <Bug className="size-3.5" />;
    case "package":
      return <Package className="size-3.5" />;
    default:
      return <Layers className="size-3.5" />;
  }
}

function stateTone(state: string): "success" | "warning" | "neutral" {
  if (state === "running") return "success";
  if (state === "paused") return "warning";
  return "neutral";
}

export function ContainerSecurityOverview({
  containers,
  onOpenContainer,
}: {
  containers: Container[];
  onOpenContainer: (container: Container) => void;
}) {
  const { t } = useI18n();
  const overview = useQuery<SecurityOverview>({
    queryKey: ["security-overview"],
    queryFn: api.securityOverview,
    refetchInterval: (query) => (query.state.data?.images.some((image) => image.security.scanning) ? 4000 : 20_000),
  });
  const [groupBy, setGroupBy] = useState<GroupBy>("image");
  const [severity, setSeverity] = useState<SeverityFilter>("all");
  const [query, setQuery] = useState("");
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const containerById = useMemo(
    () => new Map(containers.map((container) => [container.id, container])),
    [containers],
  );

  useEffect(() => {
    setExpanded({});
  }, [groupBy]);

  const groups = useMemo(() => {
    const data = overview.data;
    if (!data) return [];
    const showWhenEmpty = groupBy === "image" || groupBy === "container";
    return buildSecurityGroups(data.images ?? [], groupBy, t)
      .map((group) => filterGroup(group, query, severity, showWhenEmpty))
      .filter((group): group is SecurityGroup => group !== null)
      .sort((left, right) => {
        const leftPriority = groupPriority(left);
        const rightPriority = groupPriority(right);
        if (leftPriority !== rightPriority) return leftPriority - rightPriority;
        const leftRisk = groupRisk(left);
        const rightRisk = groupRisk(right);
        if (leftRisk !== rightRisk) return rightRisk - leftRisk;
        if (left.findings.length !== right.findings.length) return right.findings.length - left.findings.length;
        return left.label.localeCompare(right.label);
      });
  }, [groupBy, overview.data, query, severity, t]);

  const data = overview.data;
  const totals = data?.totals;
  const groupItems = [
    { value: "image", label: t("Image"), icon: <Boxes className="size-3.5" /> },
    { value: "container", label: t("Container"), icon: <ContainerIcon className="size-3.5" /> },
    { value: "cve", label: t("CVE"), icon: <Bug className="size-3.5" /> },
    { value: "package", label: t("Package"), icon: <Package className="size-3.5" /> },
    { value: "severity", label: t("Severity"), icon: <Layers className="size-3.5" /> },
  ];
  const filters: { value: SeverityFilter; label: string; count: number }[] = [
    { value: "all", label: t("All"), count: totals?.total ?? 0 },
    { value: "critical", label: t("Critical"), count: totals?.critical ?? 0 },
    { value: "high", label: t("High"), count: totals?.high ?? 0 },
    { value: "medium", label: t("Medium"), count: totals?.medium ?? 0 },
    { value: "low", label: t("Low"), count: totals?.low ?? 0 },
    { value: "unknown", label: t("Unknown"), count: totals?.unknown ?? 0 },
  ];
  const firstGroupID = groups[0]?.id;
  const isExpanded = (group: SecurityGroup) =>
    expanded[group.id] ?? (group.findings.length > 0 && (query.trim() !== "" || group.id === firstGroupID));

  const header = (
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div className="min-w-0">
        <h2 className="flex items-center gap-2 text-[13px] font-semibold text-ink">
          <ShieldCheck className="size-4 text-[var(--accent)]" /> {t("Security overview")}
        </h2>
        <p className="mt-1 max-w-3xl text-[10.5px] leading-relaxed text-muted">
          {t("Merged Trivy findings for every container image. Group by image, container, CVE, package or severity.")}
        </p>
        {totals?.lastScanAt ? (
          <p className="mt-1 text-[9px] text-subtle">
            {t("Last scan {time}", { time: new Date(totals.lastScanAt).toLocaleString() })}
          </p>
        ) : null}
      </div>
      <div className="flex flex-wrap items-center gap-1.5">
        {totals?.scanning ? <Badge tone="info" dot pulse>{t("{count} scanning", { count: totals.scanning })}</Badge> : null}
        {totals?.failed ? <Badge tone="danger" dot>{t("{count} failed", { count: totals.failed })}</Badge> : null}
        <IconButton label={t("Refresh")} size="md" onClick={() => void overview.refetch()} disabled={overview.isFetching}>
          <RefreshCw className={cn("size-4", overview.isFetching && "animate-spin")} />
        </IconButton>
      </div>
    </div>
  );

  if (overview.isLoading) {
    return <Card className="card-pad">{header}<p className="mt-4 text-[10.5px] text-muted">{t("Reading Trivy scans…")}</p></Card>;
  }
  if (overview.isError) {
    return (
      <Card className="card-pad">
        {header}
        <div className="mt-4 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-[var(--danger)]/30 bg-[var(--danger-soft)] p-3 text-[10.5px] text-[var(--danger)]">
          <span className="flex items-start gap-2"><ShieldAlert className="mt-0.5 size-3.5 shrink-0" />{overview.error.message}</span>
          <Button size="sm" variant="secondary" icon={<RefreshCw className="size-3.5" />} onClick={() => void overview.refetch()}>
            {t("Retry")}
          </Button>
        </div>
      </Card>
    );
  }
  if (!data?.enabled) {
    return (
      <Card className="card-pad">
        {header}
        <div className="mt-4 flex items-start gap-2 rounded-xl border border-line bg-[var(--surface-2)]/45 p-3 text-[10.5px] leading-relaxed text-muted">
          <ShieldCheck className="mt-0.5 size-3.5 shrink-0 text-[var(--success)]" />
          <div>
            <p className="font-medium text-soft">{t("Image scanning is disabled")}</p>
            <p className="mt-0.5">{t("Enable Trivy in Settings → Image scanning to merge scan results here.")}</p>
          </div>
        </div>
      </Card>
    );
  }

  return (
    <Card className="card-pad">
      {header}

      <div className="mt-4 grid grid-cols-2 gap-3 lg:grid-cols-4">
        <OverviewStat
          icon={<Boxes className="size-3.5" />}
          label={t("Scanned images")}
          value={`${totals?.scannedImages ?? 0}/${totals?.images ?? 0}`}
          hint={(totals?.images ?? 0) - (totals?.scannedImages ?? 0) > 0
            ? t("{count} not scanned", { count: (totals?.images ?? 0) - (totals?.scannedImages ?? 0) })
            : t("All images scanned")}
        />
        <OverviewStat
          icon={<ContainerIcon className="size-3.5" />}
          label={t("Containers")}
          value={`${totals?.containers ?? 0}`}
          hint={t("{count} scanned", { count: totals?.scannedContainers ?? 0 })}
        />
        <OverviewStat
          icon={<Bug className="size-3.5" />}
          label={t("Findings")}
          value={`${totals?.total ?? 0}`}
          hint={(totals?.stale ?? 0) > 0 ? t("{count} stale", { count: totals?.stale ?? 0 }) : t("Across unique images")}
        />
        <OverviewStat
          icon={<ShieldAlert className="size-3.5" />}
          label={t("Critical")}
          value={`${totals?.critical ?? 0}`}
          hint={t("{count} high", { count: totals?.high ?? 0 })}
          tone={(totals?.critical ?? 0) > 0 ? "danger" : undefined}
        />
      </div>

      <div className="mt-5 flex flex-wrap items-center gap-2">
        <Tabs size="md" items={groupItems} value={groupBy} onChange={(value) => setGroupBy(value as GroupBy)} />
        <div className="ml-auto flex items-center gap-1.5">
          <Button size="sm" variant="ghost" onClick={() => setExpanded(Object.fromEntries(groups.map((group) => [group.id, true])))}>
            {t("Expand all")}
          </Button>
          <Button size="sm" variant="ghost" onClick={() => setExpanded(Object.fromEntries(groups.map((group) => [group.id, false])))}>
            {t("Collapse all")}
          </Button>
        </div>
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-2">
        <InputBase
          value={query}
          onChange={setQuery}
          placeholder={t("Search CVE, package, image or container…")}
          icon={<Search className="size-3.5" />}
          className="w-full sm:w-72"
        />
        <div className="flex flex-wrap gap-1">
          {filters.map((filter) => (
            <button
              key={filter.value}
              type="button"
              onClick={() => setSeverity(filter.value)}
              className={cn(
                "rounded-md border px-2 py-1 text-[9px] font-medium transition",
                severity === filter.value
                  ? "border-[var(--accent-line)] bg-[var(--accent-soft)] text-[var(--accent)]"
                  : "border-line text-subtle hover:text-ink",
              )}
            >
              {filter.label} <span className="font-mono">{filter.count}</span>
            </button>
          ))}
        </div>
      </div>

      <div className="mt-3 space-y-2">
        {(data.images ?? []).length === 0 ? (
          <EmptyState
            icon={<Boxes className="size-5" />}
            title={t("No container image to inspect")}
            description={t("Start a container to see its security posture here.")}
          />
        ) : groups.length === 0 ? (
          <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-line bg-[var(--surface-2)]/35 px-3 py-4 text-[10.5px] text-muted">
            <span>{t("No security result matches these filters.")}</span>
            <Button size="sm" variant="secondary" onClick={() => { setQuery(""); setSeverity("all"); }}>
              {t("Clear filters")}
            </Button>
          </div>
        ) : (
          groups.map((group) => (
            <SecurityGroupCard
              key={group.id}
              group={group}
              open={isExpanded(group)}
              onToggle={() => setExpanded((current) => ({ ...current, [group.id]: !isExpanded(group) }))}
              containerById={containerById}
              onOpenContainer={onOpenContainer}
              t={t}
            />
          ))
        )}
      </div>

      <p className="mt-3 flex items-center gap-1.5 border-t border-line pt-2.5 text-[9px] text-subtle">
        <ShieldCheck className="size-3" />
        {t("Scans are merged by image ID, so containers sharing an image appear once in the totals.")}
      </p>
    </Card>
  );
}

function OverviewStat({
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
  tone?: "danger";
}) {
  return (
    <div className="rounded-xl border border-line bg-[var(--surface-2)]/40 px-3 py-2.5">
      <p className="flex items-center gap-1 text-[9px] font-semibold uppercase tracking-[0.1em] text-subtle">
        {icon} {label}
      </p>
      <p className={cn("num mt-1 text-[18px] font-semibold leading-none", tone === "danger" ? "text-[var(--danger)]" : "text-ink")}>{value}</p>
      <p className="mt-1 text-[9px] text-subtle">{hint}</p>
    </div>
  );
}

function SecurityGroupCard({
  group,
  open,
  onToggle,
  containerById,
  onOpenContainer,
  t,
}: {
  group: SecurityGroup;
  open: boolean;
  onToggle: () => void;
  containerById: Map<string, Container>;
  onOpenContainer: (container: Container) => void;
  t: Translate;
}) {
  const visible = group.findings.slice(0, 200);
  return (
    <div className="overflow-hidden rounded-xl border border-line bg-[var(--surface)]/55">
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={open}
        className="flex w-full items-start gap-3 px-3 py-2.5 text-left transition hover:bg-[var(--surface-2)]/45"
      >
        {group.kind === "container" && group.container ? (
          <EntityIcon src={containerIconURL(group.container.id)} name={group.container.name} size={28} />
        ) : (
          <span className="grid size-7 shrink-0 place-items-center rounded-lg border border-line bg-[var(--surface-2)] text-muted">
            {groupIcon(group.kind)}
          </span>
        )}
        <span className="min-w-0 flex-1">
          <span className="flex flex-wrap items-center gap-1.5">
            <span className="truncate text-[11px] font-semibold text-ink">{group.label}</span>
            {group.container ? <Badge tone={stateTone(group.container.state)}>{group.container.state}</Badge> : null}
            <SecurityCounts security={group.security} />
          </span>
          <span className="mt-0.5 block truncate text-[9.5px] text-subtle" title={group.subtitle}>{group.subtitle}</span>
        </span>
        <span className="mt-1 flex shrink-0 items-center gap-2">
          {group.security.available && !group.security.error ? (
            <span className="hidden text-[9px] text-subtle sm:inline">{countLabel(t, group.security.total, "1 finding", "{count} findings")}</span>
          ) : null}
          <ChevronDown className={cn("size-3.5 text-subtle transition-transform", open && "rotate-180")} />
        </span>
      </button>

      {open ? (
        <div className="border-t border-line/60">
          {group.security.scanning ? (
            <p className="px-3 py-3 text-[10px] text-[var(--info)]">{t("Scanning…")}</p>
          ) : !group.security.available && !group.security.error ? (
            <p className="px-3 py-3 text-[10px] leading-relaxed text-muted">
              {t("No scan result yet. A scan starts automatically after an image pull.")}
            </p>
          ) : group.security.error ? (
            <p className="px-3 py-3 text-[10px] leading-relaxed text-[var(--danger)]">{group.security.error}</p>
          ) : group.findings.length === 0 && group.security.total > 0 ? (
            <p className="flex items-center gap-1.5 px-3 py-3 text-[10px] text-[var(--warning)]">
              <ShieldAlert className="size-3.5" /> {t("Finding details are unavailable for this scan. Run Scan now to refresh it.")}
            </p>
          ) : group.findings.length === 0 ? (
            <p className="flex items-center gap-1.5 px-3 py-3 text-[10px] text-[var(--success)]">
              <ShieldCheck className="size-3.5" /> {t("No known vulnerabilities were found for this image.")}
            </p>
          ) : (
            <>
              {visible.map((finding) => (
                <FindingRow
                  key={finding.key}
                  finding={finding}
                  showLocations={group.showLocations}
                  containerById={containerById}
                  onOpenContainer={onOpenContainer}
                  t={t}
                />
              ))}
              {group.findings.length > visible.length ? (
                <p className="px-3 py-2 text-[9px] text-subtle">
                  {t("Showing the first {shown} of {count} findings.", { shown: visible.length, count: group.findings.length })}
                </p>
              ) : null}
            </>
          )}
        </div>
      ) : null}
    </div>
  );
}

function FindingRow({
  finding,
  showLocations,
  containerById,
  onOpenContainer,
  t,
}: {
  finding: AggregatedFinding;
  showLocations: boolean;
  containerById: Map<string, Container>;
  onOpenContainer: (container: Container) => void;
  t: Translate;
}) {
  return (
    <div className="flex items-start gap-2 border-b border-line/60 px-3 py-2 last:border-0">
      <SeverityBadge severity={finding.severity} />
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
          <span className="font-mono text-[10px] font-semibold text-ink">{finding.id}</span>
          {finding.title ? <span className="min-w-0 truncate text-[9.5px] text-subtle" title={finding.title}>{finding.title}</span> : null}
        </div>
        <p className="mt-0.5 text-[9.5px] text-subtle">
          <span className="font-mono text-soft">{finding.package}</span> {finding.installedVersion}
          {finding.fixedVersion
            ? <span className="text-[var(--success)]"> → {finding.fixedVersion}</span>
            : <span className="ml-1 text-[var(--warning)]">{t("No fix available")}</span>}
          {finding.target ? <span> · {finding.target}</span> : null}
        </p>
        {showLocations ? (
          <div className="mt-1 flex flex-wrap items-center gap-1">
            {finding.images.map((image) => (
              <span
                key={image.imageId || image.image}
                title={image.image}
                className="inline-flex max-w-[260px] items-center gap-1 rounded-md border border-line bg-[var(--surface-2)]/55 px-1.5 py-0.5 text-[8.5px] text-subtle"
              >
                <Boxes className="size-2.5 shrink-0" />
                <span className="truncate">{image.image}</span>
              </span>
            ))}
            {finding.containers.map((ref) => {
              const container = containerById.get(ref.id);
              const content = <><ContainerIcon className="size-2.5 shrink-0" />{ref.name}</>;
              return container ? (
                <button
                  key={ref.id}
                  type="button"
                  onClick={() => onOpenContainer(container)}
                  className="inline-flex items-center gap-1 rounded-md border border-line bg-[var(--surface-2)]/55 px-1.5 py-0.5 text-[8.5px] text-subtle transition hover:border-[var(--accent-line)] hover:text-ink"
                >
                  {content}
                </button>
              ) : (
                <span key={ref.id} className="inline-flex items-center gap-1 rounded-md border border-line bg-[var(--surface-2)]/55 px-1.5 py-0.5 text-[8.5px] text-subtle">
                  {content}
                </span>
              );
            })}
          </div>
        ) : null}
      </div>
      {finding.url ? (
        <a href={finding.url} target="_blank" rel="noreferrer" className="mt-0.5 text-subtle hover:text-ink" aria-label={t("Open vulnerability details")}>
          <ExternalLink className="size-3" />
        </a>
      ) : null}
    </div>
  );
}
