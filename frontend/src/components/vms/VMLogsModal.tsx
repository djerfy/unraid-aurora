import { useQuery } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { useI18n } from "../../i18n";
import { Modal } from "../ui/overlays";
import { LogViewer } from "../ui/log-viewer";
import type { VM } from "../../lib/types";

export function VMLogsModal({ vm, onClose }: { vm: VM | null; onClose: () => void }) {
  const { t } = useI18n();
  const logs = useQuery({
    queryKey: ["vm-logs", vm?.id],
    queryFn: () => api.vmLogs(vm?.id ?? "", 240),
    enabled: Boolean(vm),
    refetchInterval: 6000,
  });

  return (
    <Modal
      open={Boolean(vm)}
      onClose={onClose}
      title={vm ? `${vm.name} · ${t("Logs")}` : t("VM logs")}
      description={t("libvirt domain log")}
      size="logs"
    >
      <LogViewer
        lines={logs.data}
        loading={logs.isLoading}
        refreshing={logs.isFetching}
        onRefresh={() => void logs.refetch()}
        downloadName={`${vm?.name ?? "vm"}.log`}
        title={t("VM logs")}
        heightClass="h-[58vh]"
      />
    </Modal>
  );
}
