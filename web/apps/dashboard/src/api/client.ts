import type {
  WorkflowDetails,
  WorkflowMetrics,
  WorkflowStatus,
  WorkflowSummary,
} from "./types";

async function jsonFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
      ...(init?.headers ?? {}),
    },
  });
  if (!resp.ok) {
    const body = await resp.text().catch(() => "");
    throw new Error(`HTTP ${resp.status}${body ? `: ${body}` : ""}`);
  }
  return (await resp.json()) as T;
}

export interface ListWorkflowsOpts {
  status?: WorkflowStatus | "ALL";
  limit?: number;
}

export const api = {
  listWorkflows(opts: ListWorkflowsOpts = {}): Promise<WorkflowSummary[]> {
    const params = new URLSearchParams();
    if (opts.status && opts.status !== "ALL") params.set("status", opts.status);
    if (opts.limit) params.set("limit", String(opts.limit));
    const qs = params.toString();
    return jsonFetch<{ workflows: WorkflowSummary[] }>(
      `/api/workflows${qs ? `?${qs}` : ""}`,
    ).then((r) => r.workflows ?? []);
  },

  getWorkflow(id: string): Promise<WorkflowDetails> {
    return jsonFetch<WorkflowDetails>(
      `/api/workflows/${encodeURIComponent(id)}`,
    );
  },

  startWorkflow(input: {
    workflowName: string;
    input?: string;
    executionTimeoutSeconds?: number;
  }): Promise<{ workflowId: string; runId: string }> {
    return jsonFetch("/api/workflows", {
      method: "POST",
      body: JSON.stringify(input),
    });
  },

  cancelWorkflow(id: string, reason: string): Promise<void> {
    return jsonFetch(`/api/workflows/${encodeURIComponent(id)}/cancel`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    });
  },

  metrics(): Promise<WorkflowMetrics> {
    return jsonFetch<WorkflowMetrics>("/api/metrics");
  },
};
