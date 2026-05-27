import { useQuery } from "@tanstack/react-query";
import { Counter } from "@sched/design";
import { CheckCircle2, Play, TriangleAlert, Workflow } from "lucide-react";
import { api } from "../api/client";

const CARDS = [
  { key: "totalWorkflows", label: "Total", icon: Workflow, tone: "text-zinc-300" },
  { key: "runningWorkflows", label: "Running", icon: Play, tone: "text-accent-300" },
  { key: "completedWorkflows", label: "Completed", icon: CheckCircle2, tone: "text-emerald-300" },
  { key: "failedWorkflows", label: "Failed", icon: TriangleAlert, tone: "text-rose-300" },
] as const;

export function MetricsCards() {
  const metrics = useQuery({
    queryKey: ["metrics"],
    queryFn: () => api.metrics(),
    refetchInterval: 5_000,
  });

  return (
    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
      {CARDS.map(({ key, label, icon: Icon, tone }) => (
        <div
          key={key}
          className="group relative overflow-hidden rounded-lg border border-zinc-900 bg-zinc-950 p-4 transition-colors hover:border-zinc-800"
        >
          <div className="flex items-center justify-between">
            <span className="text-xs uppercase tracking-wider text-zinc-500">
              {label}
            </span>
            <Icon className={`size-3.5 ${tone}`} strokeWidth={1.75} />
          </div>
          <div className="mt-2 flex items-baseline gap-2">
            <Counter
              value={metrics.data?.[key] ?? 0}
              className="text-3xl font-medium text-zinc-50"
            />
          </div>
        </div>
      ))}
    </div>
  );
}
