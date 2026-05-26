import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { AnimatePresence, motion } from "motion/react";
import { api } from "../api/client";
import type { WorkflowSummary } from "../api/types";

export function ActivityRail() {
  const { data, isLoading } = useQuery({
    queryKey: ["activity-rail"],
    queryFn: () => api.listWorkflows({ limit: 8 }),
    refetchInterval: 3_500,
  });

  if (isLoading) {
    return <SkeletonRail />;
  }

  const items = data ?? [];
  if (items.length === 0) {
    return <p className="px-3 text-[11px] text-zinc-600">No activity yet.</p>;
  }

  return (
    <ul className="flex flex-col">
      <AnimatePresence initial={false}>
        {items.map((wf) => (
          <motion.li
            key={wf.workflowId}
            initial={{ opacity: 0, x: -4 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.18 }}
          >
            <Link
              to="/app/workflows/$id"
              params={{ id: wf.workflowId }}
              className="flex flex-col gap-0.5 rounded-md px-3 py-2 text-[12px] hover:bg-zinc-900/40"
            >
              <div className="flex items-center gap-2">
                <Dot status={wf.status} />
                <span className="truncate text-zinc-200">{wf.workflowName}</span>
              </div>
              <span className="ml-3 truncate font-mono text-[10px] text-zinc-500">
                {wf.workflowId.slice(0, 8)}
              </span>
            </Link>
          </motion.li>
        ))}
      </AnimatePresence>
    </ul>
  );
}

function Dot({ status }: { status: WorkflowSummary["status"] }) {
  const tone: Record<WorkflowSummary["status"], string> = {
    RUNNING: "bg-accent-500",
    COMPLETED: "bg-emerald-400",
    FAILED: "bg-rose-400",
    TIMED_OUT: "bg-amber-400",
    CANCELED: "bg-zinc-500",
  };
  return <span className={`size-1.5 rounded-full ${tone[status]}`} />;
}

function SkeletonRail() {
  return (
    <ul className="flex flex-col gap-1 px-3">
      {Array.from({ length: 4 }).map((_, i) => (
        <li key={i} className="h-8 animate-pulse rounded-md bg-zinc-900/60" />
      ))}
    </ul>
  );
}
