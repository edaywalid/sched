import { useQuery } from "@tanstack/react-query";
import { api } from "../api/client";

const CARDS = [
  { key: "totalWorkflows", label: "Total" },
  { key: "runningWorkflows", label: "Running" },
  { key: "completedWorkflows", label: "Completed" },
  { key: "failedWorkflows", label: "Failed" },
] as const;

export function MetricsCards() {
  const metrics = useQuery({
    queryKey: ["metrics"],
    queryFn: () => api.metrics(),
    refetchInterval: 5_000,
  });

  return (
    <div className="grid gap-px overflow-hidden rounded-md border border-zinc-900 bg-zinc-900 sm:grid-cols-4">
      {CARDS.map((c) => (
        <div key={c.key} className="bg-zinc-950 p-4">
          <div className="text-xs uppercase tracking-wider text-zinc-500">
            {c.label}
          </div>
          <div className="mt-1 text-2xl font-medium text-zinc-100">
            {formatCount(metrics.data?.[c.key])}
          </div>
        </div>
      ))}
    </div>
  );
}

function formatCount(n: number | undefined): string {
  if (n === undefined) return "—";
  return n.toLocaleString();
}
