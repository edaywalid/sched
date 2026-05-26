import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";

export function SystemStatus({ compact = false }: { compact?: boolean }) {
  const health = useQuery({
    queryKey: ["api-health"],
    queryFn: async () => {
      const resp = await fetch("/api/health");
      return resp.ok;
    },
    refetchInterval: 5_000,
    retry: 0,
  });

  const metrics = useQuery({
    queryKey: ["metrics-top"],
    queryFn: () => api.metrics(),
    refetchInterval: 5_000,
  });

  const ok = health.data === true;
  const running = metrics.data?.runningWorkflows ?? 0;
  const total = metrics.data?.totalWorkflows ?? 0;

  if (compact) {
    return (
      <div className="flex items-center gap-2">
        <span
          className={`size-1.5 rounded-full ${
            ok ? "bg-accent-500" : "bg-rose-500"
          }`}
        />
        <span>{ok ? "engine connected" : "engine unreachable"}</span>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-5 text-xs">
      <Stat label="Engine" value={ok ? "connected" : "unreachable"} tone={ok ? "ok" : "error"} />
      <Stat label="Running" value={running.toLocaleString()} tone="info" />
      <Stat label="Total" value={total.toLocaleString()} tone="muted" />
    </div>
  );
}

function Stat({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone: "ok" | "error" | "info" | "muted";
}) {
  const toneClass: Record<typeof tone, string> = {
    ok: "text-accent-300",
    error: "text-rose-300",
    info: "text-zinc-100",
    muted: "text-zinc-400",
  };
  return (
    <div className="flex items-baseline gap-1.5">
      <span className="text-zinc-500">{label}</span>
      <span className={`font-medium ${toneClass[tone]}`}>{value}</span>
    </div>
  );
}
