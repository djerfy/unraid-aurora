import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../lib/api";
import type {
  MetricPoint,
  Metrics,
  SessionInfo,
  Snapshot,
} from "../lib/types";

type AppDataValue = {
  snapshot: Snapshot | null;
  session: SessionInfo | null;
  isLoading: boolean;
  error: Error | null;
  metrics: Metrics | null;
  history: MetricPoint[];
  live: boolean;
  refresh: () => void;
};

const AppDataContext = createContext<AppDataValue | null>(null);

function mergeHistory(previous: MetricPoint[], incoming: MetricPoint[]): MetricPoint[] {
  const byTime = new Map<number, MetricPoint>();
  for (const point of previous) byTime.set(point.t, point);
  for (const point of incoming) byTime.set(point.t, point);
  return [...byTime.values()].sort((a, b) => a.t - b.t).slice(-900);
}

export function AppDataProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();

  const sessionQuery = useQuery({
    queryKey: ["session"],
    queryFn: api.session,
    retry: false,
    staleTime: 30_000,
  });

  const authenticated = sessionQuery.data?.authenticated ?? false;

  const bootstrapQuery = useQuery({
    queryKey: ["bootstrap", "light"],
    queryFn: () => api.bootstrap(true),
    enabled: authenticated,
    refetchInterval: 12_000,
    staleTime: 4_000,
    placeholderData: (previous) => previous,
  });
  const pluginsQuery = useQuery({
    queryKey: ["plugins"],
    queryFn: api.plugins,
    enabled: authenticated,
    refetchInterval: 60_000,
    staleTime: 30_000,
  });
  const notificationsQuery = useQuery({
    queryKey: ["notifications"],
    queryFn: api.notifications,
    enabled: authenticated,
    refetchInterval: 15_000,
    staleTime: 10_000,
  });

  const [metrics, setMetrics] = useState<Metrics | null>(null);
  const [history, setHistory] = useState<MetricPoint[]>([]);
  const [live, setLive] = useState(false);

  useEffect(() => {
    if (bootstrapQuery.data) {
      setMetrics(bootstrapQuery.data.metrics);
      setHistory((prev) => mergeHistory(prev, bootstrapQuery.data.history));
    }
  }, [bootstrapQuery.data]);

  useEffect(() => {
    if (!authenticated) return;
    const source = new EventSource("/api/v1/events");

    source.addEventListener("hello", () => setLive(true));
    source.addEventListener("metrics", (event) => {
      try {
        const next = JSON.parse((event as MessageEvent<string>).data) as Metrics;
        setMetrics(next);
        setHistory((prev) =>
          mergeHistory(prev, [
            {
              t: next.t,
              cpu: next.cpu,
              memory: next.memory,
              netRx: next.netRx,
              netTx: next.netTx,
              diskRead: next.diskRead,
              diskWrite: next.diskWrite,
              temp: next.temp,
            },
          ]),
        );
      } catch {
        // Ignore malformed frames; the next sample will arrive shortly.
      }
    });
    source.onerror = () => setLive(false);
    return () => {
      source.close();
      setLive(false);
    };
  }, [authenticated]);

  const snapshot = useMemo(() => {
    const base = bootstrapQuery.data;
    if (!base) return null;
    return {
      ...base,
      plugins: pluginsQuery.data ?? base.plugins,
      notifications: notificationsQuery.data ?? base.notifications,
    };
  }, [bootstrapQuery.data, notificationsQuery.data, pluginsQuery.data]);

  const refresh = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: ["session"] });
    void queryClient.invalidateQueries({ queryKey: ["bootstrap"] });
    void queryClient.invalidateQueries({ queryKey: ["plugins"] });
    void queryClient.invalidateQueries({ queryKey: ["notifications"] });
  }, [queryClient]);

  const value = useMemo<AppDataValue>(
    () => ({
      snapshot,
      session: sessionQuery.data ?? null,
      isLoading: sessionQuery.isLoading || (authenticated && bootstrapQuery.isLoading),
      error: (sessionQuery.error as Error | null) ?? (bootstrapQuery.error as Error | null),
      metrics,
      history,
      live,
      refresh,
    }),
    [
      snapshot,
      bootstrapQuery.error,
      bootstrapQuery.isLoading,
      sessionQuery.data,
      sessionQuery.error,
      sessionQuery.isLoading,
      metrics,
      history,
      live,
      refresh,
      authenticated,
    ],
  );

  return <AppDataContext.Provider value={value}>{children}</AppDataContext.Provider>;
}

export function useApp() {
  const ctx = useContext(AppDataContext);
  if (!ctx) throw new Error("useApp must be used inside AppDataProvider");
  return ctx;
}

/** Convenience hook for pages that always render after bootstrap. */
export function useSnapshot(): Snapshot {
  const { snapshot } = useApp();
  if (!snapshot) throw new Error("snapshot is not loaded yet");
  return snapshot;
}
