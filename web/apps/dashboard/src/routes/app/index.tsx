import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { useState, type ReactNode } from "react";
import { AnimatePresence, motion } from "motion/react";
import { ChevronRight, RefreshCw } from "lucide-react";
import { api } from "../../api/client";
import type { WorkflowStatus, WorkflowSummary } from "../../api/types";
import { StatusBadge } from "../../components/StatusBadge";
import { RelativeTime } from "../../components/RelativeTime";
import { MetricsCards } from "../../components/MetricsCards";
import { StartWorkflowForm } from "../../components/StartWorkflowForm";

export const Route = createFileRoute("/app/")({
  component: WorkflowsPage,
});

const STATUS_TABS: Array<{ label: string; value: WorkflowStatus | "ALL" }> = [
  { label: "All", value: "ALL" },
  { label: "Running", value: "RUNNING" },
  { label: "Completed", value: "COMPLETED" },
  { label: "Failed", value: "FAILED" },
  { label: "Timed out", value: "TIMED_OUT" },
  { label: "Canceled", value: "CANCELED" },
];

function WorkflowsPage() {
  const [status, setStatus] = useState<WorkflowStatus | "ALL">("ALL");
  const list = useQuery({
    queryKey: ["workflows", status],
    queryFn: () => api.listWorkflows({ status }),
    refetchInterval: 4_000,
  });

  return (
    <div className="space-y-10">
      <PageHeader onRefresh={() => list.refetch()} refreshing={list.isFetching} />

      <Section title="Overview" muted="Live counters refresh every 5 seconds.">
        <MetricsCards />
      </Section>

      <Section title="Start a workflow" muted="Queue any registered workflow on the default task queue.">
        <StartWorkflowForm />
      </Section>

      <Section
        title="Recent activity"
        muted="Most recent runs first."
        right={
          <div className="flex flex-wrap gap-1">
            {STATUS_TABS.map((tab) => (
              <button
                key={tab.value}
                type="button"
                onClick={() => setStatus(tab.value)}
                className={[
                  "rounded-md border px-2.5 py-1 text-xs transition-colors",
                  tab.value === status
                    ? "border-zinc-700 bg-zinc-900 text-zinc-100"
                    : "border-transparent text-zinc-400 hover:border-zinc-800 hover:text-zinc-200",
                ].join(" ")}
              >
                {tab.label}
              </button>
            ))}
          </div>
        }
      >
        <WorkflowTable
          items={list.data ?? []}
          loading={list.isLoading}
          error={list.error}
        />
      </Section>
    </div>
  );
}

function PageHeader({
  onRefresh,
  refreshing,
}: {
  onRefresh: () => void;
  refreshing: boolean;
}) {
  return (
    <header className="flex items-end justify-between gap-6">
      <div>
        <h1 className="text-2xl font-medium tracking-tight text-zinc-50">
          Workflows
        </h1>
        <p className="mt-1 text-sm text-zinc-400">
          Inspect, start, and cancel workflow runs across every queue.
        </p>
      </div>
      <button
        type="button"
        onClick={onRefresh}
        className="inline-flex items-center gap-1.5 rounded-md border border-zinc-800 px-2.5 py-1.5 text-xs text-zinc-300 transition-colors hover:border-zinc-700 hover:text-zinc-100"
      >
        <RefreshCw
          className={`size-3.5 ${refreshing ? "animate-spin" : ""}`}
          strokeWidth={1.75}
        />
        Refresh
      </button>
    </header>
  );
}

function Section({
  title,
  muted,
  right,
  children,
}: {
  title: string;
  muted?: string;
  right?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="text-sm font-medium text-zinc-100">{title}</h2>
          {muted ? (
            <p className="mt-0.5 text-xs text-zinc-500">{muted}</p>
          ) : null}
        </div>
        {right}
      </div>
      {children}
    </section>
  );
}

function WorkflowTable({
  items,
  loading,
  error,
}: {
  items: WorkflowSummary[];
  loading: boolean;
  error: unknown;
}) {
  if (loading) return <TableSkeleton />;
  if (error) {
    return (
      <p className="text-sm text-rose-300">
        Failed to load workflows:{" "}
        {error instanceof Error ? error.message : String(error)}
      </p>
    );
  }
  if (items.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-zinc-900 bg-zinc-950 px-4 py-16 text-center">
        <p className="text-sm text-zinc-400">No workflows yet.</p>
        <p className="mt-1 text-xs text-zinc-600">
          Use the form above to queue your first run.
        </p>
      </div>
    );
  }

  return (
    <div className="overflow-hidden rounded-lg border border-zinc-900">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-zinc-900 bg-zinc-950 text-xs uppercase tracking-wider text-zinc-500">
            <Th>Workflow</Th>
            <Th>Status</Th>
            <Th>Started</Th>
            <Th>Duration</Th>
            <Th className="w-px" />
          </tr>
        </thead>
        <tbody>
          <AnimatePresence initial={false}>
            {items.map((wf) => (
              <motion.tr
                key={wf.workflowId}
                initial={{ opacity: 0, y: 4 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.18 }}
                className="border-b border-zinc-900 last:border-0 transition-colors hover:bg-zinc-900/40"
              >
                <Td>
                  <Link
                    to="/app/workflows/$id"
                    params={{ id: wf.workflowId }}
                    className="block text-zinc-100 hover:text-accent-300"
                  >
                    {wf.workflowName}
                  </Link>
                  <div className="font-mono text-xs text-zinc-500">
                    {wf.workflowId.slice(0, 8)}
                  </div>
                </Td>
                <Td>
                  <StatusBadge status={wf.status} />
                </Td>
                <Td className="text-zinc-300">
                  <RelativeTime ts={wf.startTime} />
                </Td>
                <Td className="text-zinc-300">{formatDuration(wf)}</Td>
                <Td>
                  <Link
                    to="/app/workflows/$id"
                    params={{ id: wf.workflowId }}
                    className="flex items-center justify-end text-zinc-500 hover:text-zinc-200"
                  >
                    <ChevronRight className="size-4" strokeWidth={1.75} />
                  </Link>
                </Td>
              </motion.tr>
            ))}
          </AnimatePresence>
        </tbody>
      </table>
    </div>
  );
}

function Th({
  children,
  className = "",
}: {
  children?: ReactNode;
  className?: string;
}) {
  return (
    <th className={`px-4 py-2 text-left font-medium ${className}`}>{children}</th>
  );
}

function Td({
  children,
  className = "",
}: {
  children?: ReactNode;
  className?: string;
}) {
  return <td className={`px-4 py-3 align-middle ${className}`}>{children}</td>;
}

function TableSkeleton() {
  return (
    <div className="space-y-1">
      {Array.from({ length: 4 }).map((_, i) => (
        <div
          key={i}
          className="h-12 animate-pulse rounded-lg border border-zinc-900 bg-zinc-950"
        />
      ))}
    </div>
  );
}

function formatDuration(wf: WorkflowSummary): string {
  if (!wf.startTime) return "-";
  const end = wf.endTime || Date.now();
  const ms = end - wf.startTime;
  if (ms < 1000) return `${ms} ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)} s`;
  return `${(ms / 60_000).toFixed(1)} m`;
}
