import { useCallback, useEffect, useState } from "react";
import type { NetInterface } from "./types";

const STORAGE_KEY = "aurora.network.interface";

function defaultInterface(interfaces: NetInterface[]): string {
  return interfaces.find((iface) => iface.name === "br0")?.name
    ?? interfaces.find((iface) => iface.state === "up")?.name
    ?? interfaces[0]?.name
    ?? "";
}

export function useNetworkInterface(interfaces: NetInterface[]) {
  const [selectedName, setSelectedName] = useState(() => {
    try {
      return localStorage.getItem(STORAGE_KEY) ?? "";
    } catch {
      return "";
    }
  });

  useEffect(() => {
    if (interfaces.length === 0) return;
    if (selectedName && interfaces.some((iface) => iface.name === selectedName)) return;
    const fallback = defaultInterface(interfaces);
    if (!fallback) return;
    setSelectedName(fallback);
    try {
      localStorage.setItem(STORAGE_KEY, fallback);
    } catch {
      // Storage can be unavailable; the fallback still applies for this session.
    }
  }, [interfaces, selectedName]);

  const selectInterface = useCallback((name: string) => {
    setSelectedName(name);
    try {
      localStorage.setItem(STORAGE_KEY, name);
    } catch {
      // Storage can be unavailable; the choice still applies for this session.
    }
  }, []);

  const selected =
    interfaces.find((iface) => iface.name === selectedName) ??
    interfaces.find((iface) => iface.name === defaultInterface(interfaces)) ??
    interfaces[0];

  return { selected, selectedName: selected?.name ?? "", selectInterface };
}
