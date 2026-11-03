import { useSearchParams } from "react-router-dom";
import { History as HistoryIcon, ScrollText, ShieldCheck } from "lucide-react";
import { Tabs } from "../components/ui/primitives";
import { useI18n } from "../i18n";
import { LogsPage } from "./Logs";
import { AuditPage } from "./Audit";
import { HistoryPage } from "./History";

export function LogsHubPage() {
  const { t } = useI18n();
  const [params, setParams] = useSearchParams();
  const tabParam = params.get("tab");
  const tab = tabParam === "audit" ? "audit" : tabParam === "history" ? "history" : "logs";
  const setTab = (value: string) => {
    const next = new URLSearchParams(params);
    if (value === "logs") next.delete("tab");
    else next.set("tab", value);
    setParams(next, { replace: true });
  };

  return (
    <div className="space-y-4">
      <Tabs
        items={[
          { value: "logs", label: t("Logs"), icon: <ScrollText className="size-3.5" /> },
          { value: "audit", label: t("Audit log"), icon: <ShieldCheck className="size-3.5" /> },
          { value: "history", label: t("History"), icon: <HistoryIcon className="size-3.5" /> },
        ]}
        value={tab}
        onChange={setTab}
      />
      {tab === "audit" ? <AuditPage /> : tab === "history" ? <HistoryPage /> : <LogsPage />}
    </div>
  );
}
