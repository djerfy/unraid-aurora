import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Usb } from "lucide-react";
import { Badge, Button } from "../ui/primitives";
import { useToast } from "../../app/ToastProvider";
import { useI18n } from "../../i18n";
import { api } from "../../lib/api";
import type { VM, VMAdvancedConfig } from "../../lib/types";

function deviceKey(device: VMAdvancedConfig["hostDevices"][number]) {
  return `usb:${device.vendor ?? ""}:${device.product ?? ""}`.toLowerCase();
}

export function VMUSBHotplug({ vm }: { vm: VM }) {
  const { t } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const devices = useQuery({ queryKey: ["vm-host-devices"], queryFn: api.vmHostDevices, staleTime: 60_000 });
  const advanced = useQuery({ queryKey: ["vm-advanced", vm.id], queryFn: () => api.vmAdvanced(vm.id) });
  const attached = new Set((advanced.data?.hostDevices ?? []).map(deviceKey));
  const usbDevices = (devices.data ?? []).filter((device) => device.type === "usb" && device.vendor && device.product);

  const change = useMutation({
    mutationFn: ({ device, attach }: { device: { vendor?: string; product?: string }; attach: boolean }) => {
      const vendor = device.vendor ?? "";
      const product = device.product ?? "";
      return attach ? api.attachVMUSB(vm.id, vendor, product) : api.detachVMUSB(vm.id, vendor, product);
    },
    onSuccess: (_result, variables) => {
      toast({
        kind: "success",
        title: variables.attach ? t("USB device attached") : t("USB device detached"),
      });
      void queryClient.invalidateQueries({ queryKey: ["vm-advanced", vm.id] });
      void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
    },
    onError: (error: Error) => toast({ kind: "error", title: t("Unable to change the USB device"), description: error.message }),
  });

  return (
    <div className="rounded-xl border border-line p-3">
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <Usb className="size-3.5 text-[var(--violet)]" />
          <span className="text-[11px] font-medium text-soft">{t("USB passthrough")}</span>
        </div>
        <Badge tone={vm.state === "running" ? "success" : "neutral"}>{t(vm.state === "running" ? "Live" : "Config")}</Badge>
      </div>
      <p className="mt-1 text-[9.5px] text-subtle">
        {t("Attach a host USB device to this VM. Running guests receive it immediately; stopped guests record it for the next boot.")}
      </p>
      {devices.isLoading ? (
        <p className="mt-3 text-[10px] text-muted">{t("Discovering host devices…")}</p>
      ) : usbDevices.length === 0 ? (
        <p className="mt-3 text-[10px] text-subtle">{t("No USB device detected.")}</p>
      ) : (
        <div className="mt-3 space-y-1.5">
          {usbDevices.map((device) => {
            const key = deviceKey(device);
            const isAttached = attached.has(key);
            return (
              <div key={key} className="flex flex-wrap items-center gap-2 rounded-lg border border-line bg-[var(--surface-2)]/45 px-2.5 py-2">
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[10.5px] text-soft">{device.name}</p>
                  <p className="font-mono text-[9px] text-subtle">{device.vendor}:{device.product}</p>
                </div>
                {isAttached ? <Badge tone="success">{t("Attached")}</Badge> : null}
                <Button
                  size="sm"
                  variant={isAttached ? "secondary" : "primary"}
                  loading={change.isPending && change.variables?.device.vendor === device.vendor && change.variables?.device.product === device.product}
                  onClick={() => change.mutate({ device, attach: !isAttached })}
                >
                  {isAttached ? t("Detach") : t("Attach")}
                </Button>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
