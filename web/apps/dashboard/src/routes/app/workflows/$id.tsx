import { Link, createFileRoute, useNavigate } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { api } from "../../../api/client";
import { StatusBadge } from "../../../components/StatusBadge";
import { RelativeTime } from "../../../components/RelativeTime";
import { HistoryTimeline } from "../../../components/HistoryTimeline";

export const Route = createFileRoute("/app/workflows/$id")({
  component: WorkflowDetailPage,
});

function WorkflowDetailPage() {
  const { id } = Route.useParams();
  const navigate = useNavigate();
  const qc = useQueryClient();

  const details = useQuery({
    queryKey: ["workflow", id],
    queryFn: () => api.getWorkflow(id),
    refetchInterval: 3_000,
    enabled: id.length > 0,
  });

  const cancel = useMutation({
    mutationFn: () => api.cancelWorkflow(id, "cancelled from dashboard"),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["workflow", id] });
      qc.invalidateQueries({ queryKey: ["workflows"] });
    },
  });

  if (details.isLoading) return <DetailSkeleton />;
  if (details.error) {
    return (
      <ErrorPanel
        title="Workflow not found"
        message={(details.error as Error).message}
        onBack={() => navigate({ to: "/app" })}
      />
    );
  }
  if (!details.data) return null;

  const { execution, history } = details.data;

  return (
    <div className="space-y-8">
      <div>
        <Link to="/app" className="text-xs text-zinc-500 hover:text-zinc-200">
          ← Workflows
        </Link>
        <div className="mt-4 flex items-start justify-between gap-4">
          <div>
            <h1 className="text-xl font-medium text-zinc-100">
              {execution.workflowName}
            </h1>
            <div className="mt-1 font-mono text-xs text-zinc-500">
              {execution.workflowId}
            </div>
          </div>
          <div className="flex items-center gap-3">
            <StatusBadge status={execution.status} />
            {execution.status === "RUNNING" ? (
              <button
                type="button"
                onClick={() => {
                  if (confirm("Cancel this workflow?")) cancel.mutate();
                }}
                disabled={cancel.isPending}
                className="rounded-md border border-rose-900/60 bg-rose-950/40 px-3 py-1 text-xs text-rose-200 hover:border-rose-800 disabled:opacity-60"
              >
                {cancel.isPending ? "Cancelling…" : "Cancel"}
              </button>
            ) : null}
          </div>
        </div>
      </div>

      <Meta execution={execution} />

      {execution.error ? (
        <Panel tone="error" title="Error">
          <pre className="overflow-auto whitespace-pre-wrap break-words font-mono text-xs text-rose-100">
            {execution.error}
          </pre>
        </Panel>
      ) : null}

      {execution.result ? (
        <Panel tone="success" title="Result">
          <pre className="overflow-auto whitespace-pre-wrap break-words font-mono text-xs text-emerald-100">
            {execution.result}
          </pre>
        </Panel>
      ) : null}

      <section>
        <div className="flex items-baseline justify-between">
          <h2 className="text-sm font-medium text-zinc-100">History</h2>
          <span className="text-xs text-zinc-500">{history.length} events</span>
        </div>
        <div className="mt-3">
          <HistoryTimeline events={history} />
        </div>
      </section>
    </div>
  );
}

function Meta({
  execution,
}: {
  execution: { startTime: number; endTime: number; runId: string };
}) {
  return (
    <div className="grid gap-px overflow-hidden rounded-md border border-zinc-900 bg-zinc-900 sm:grid-cols-3">
      <MetaCell label="Started">
        <RelativeTime ts={execution.startTime} />
      </MetaCell>
      <MetaCell label="Duration">{formatDuration(execution)}</MetaCell>
      <MetaCell label="Run ID">
        <span className="font-mono text-xs text-zinc-300">
          {execution.runId || "—"}
        </span>
      </MetaCell>
    </div>
  );
}

function MetaCell({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <div className="bg-zinc-950 p-4">
      <div className="text-xs uppercase tracking-wider text-zinc-500">{label}</div>
      <div className="mt-1 text-sm text-zinc-100">{children}</div>
    </div>
  );
}

function Panel({
  title,
  tone,
  children,
}: {
  title: string;
  tone: "success" | "error";
  children: ReactNode;
}) {
  const border = tone === "error" ? "border-rose-900/60" : "border-emerald-900/60";
  return (
    <section className={`overflow-hidden rounded-md border ${border} bg-zinc-950`}>
      <div className="border-b border-inherit px-4 py-2 text-xs uppercase tracking-wider text-zinc-400">
        {title}
      </div>
      <div className="p-4">{children}</div>
    </section>
  );
}

function ErrorPanel({
  title,
  message,
  onBack,
}: {
  title: string;
  message: string;
  onBack: () => void;
}) {
  return (
    <div className="rounded-md border border-zinc-900 bg-zinc-950 p-6">
      <div className="font-mono text-xs uppercase tracking-wider text-zinc-500">{title}</div>
      <p className="mt-2 text-sm text-rose-300">{message}</p>
      <button
        type="button"
        onClick={onBack}
        className="mt-4 rounded-md border border-zinc-800 px-3 py-1 text-xs text-zinc-300 hover:border-zinc-700"
      >
        Back to workflows
      </button>
    </div>
  );
}

function DetailSkeleton() {
  return (
    <div className="space-y-6">
      <div className="h-4 w-24 animate-pulse rounded bg-zinc-900" />
      <div className="h-7 w-72 animate-pulse rounded bg-zinc-900" />
      <div className="h-24 animate-pulse rounded-md bg-zinc-900" />
      <div className="h-64 animate-pulse rounded-md bg-zinc-900" />
    </div>
  );
}

function formatDuration(exec: { startTime: number; endTime: number }): string {
  if (!exec.startTime) return "—";
  const end = exec.endTime || Date.now();
  const ms = end - exec.startTime;
  if (ms < 1000) return `${ms} ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)} s`;
  return `${(ms / 60_000).toFixed(1)} m`;
}
