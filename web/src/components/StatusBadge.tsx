import type { WorkflowStatus } from "../api/types";

const STYLES: Record<WorkflowStatus, string> = {
  RUNNING: "border-accent-700 bg-accent-700/15 text-accent-300",
  COMPLETED: "border-emerald-800 bg-emerald-900/30 text-emerald-300",
  FAILED: "border-rose-800 bg-rose-900/30 text-rose-300",
  TIMED_OUT: "border-amber-800 bg-amber-900/30 text-amber-300",
  CANCELED: "border-zinc-700 bg-zinc-800/60 text-zinc-300",
};

const LABEL: Record<WorkflowStatus, string> = {
  RUNNING: "Running",
  COMPLETED: "Completed",
  FAILED: "Failed",
  TIMED_OUT: "Timed out",
  CANCELED: "Canceled",
};

export function StatusBadge({ status }: { status: WorkflowStatus }) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs ${STYLES[status]}`}
    >
      <span className="size-1.5 rounded-full bg-current opacity-80" />
      {LABEL[status]}
    </span>
  );
}
