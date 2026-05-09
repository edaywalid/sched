import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router";
import { useState, type ReactNode } from "react";
import { api } from "../api/client";
import type { WorkflowStatus, WorkflowSummary } from "../api/types";
import { StatusBadge } from "../components/StatusBadge";
import { RelativeTime } from "../components/RelativeTime";
import { MetricsCards } from "../components/MetricsCards";
import { StartWorkflowForm } from "../components/StartWorkflowForm";

const STATUS_TABS: Array<{ label: string; value: WorkflowStatus | "ALL" }> = [
  { label: "All", value: "ALL" },
  { label: "Running", value: "RUNNING" },
  { label: "Completed", value: "COMPLETED" },
  { label: "Failed", value: "FAILED" },
  { label: "Timed out", value: "TIMED_OUT" },
  { label: "Canceled", value: "CANCELED" },
];

export function WorkflowsPage() {
  const [status, setStatus] = useState<WorkflowStatus | "ALL">("ALL");
  const list = useQuery({
    queryKey: ["workflows", status],
    queryFn: () => api.listWorkflows({ status }),
    refetchInterval: 4_000,
  });

  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-xl font-medium text-zinc-100">Workflows</h1>
        <p className="mt-1 text-sm text-zinc-500">
          Recent runs across every queue.
        </p>
      </header>

      <MetricsCards />
      <StartWorkflowForm />

      <div className="flex flex-wrap gap-1">
        {STATUS_TABS.map((tab) => (
          <button
            key={tab.value}
            type="button"
            onClick={() => setStatus(tab.value)}
            className={[
              "rounded-md border px-2.5 py-1 text-xs",
              tab.value === status
                ? "border-zinc-700 bg-zinc-900 text-zinc-100"
                : "border-transparent text-zinc-400 hover:border-zinc-800 hover:text-zinc-200",
            ].join(" ")}
          >
            {tab.label}
          </button>
        ))}
      </div>

      <WorkflowTable
        items={list.data ?? []}
        loading={list.isLoading}
        error={list.error}
      />
    </div>
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
        Failed to load workflows: {String(error instanceof Error ? error.message : error)}
      </p>
    );
  }
  if (items.length === 0) {
    return (
      <div className="rounded-md border border-zinc-900 bg-zinc-950 px-4 py-12 text-center text-sm text-zinc-500">
        No workflows yet.
      </div>
    );
  }

  return (
    <div className="overflow-hidden rounded-md border border-zinc-900">
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
          {items.map((wf) => (
            <tr
              key={wf.workflowId}
              className="border-b border-zinc-900 last:border-0 hover:bg-zinc-950/60"
            >
              <Td>
                <Link
                  to={`/app/workflows/${wf.workflowId}`}
                  className="text-zinc-100 hover:text-accent-300"
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
                  to={`/app/workflows/${wf.workflowId}`}
                  className="text-xs text-zinc-500 hover:text-zinc-200"
                >
                  open
                </Link>
              </Td>
            </tr>
          ))}
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
    <th className={`px-4 py-2 text-left font-medium ${className}`}>
      {children}
    </th>
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
          className="h-12 animate-pulse rounded-md border border-zinc-900 bg-zinc-950"
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
