export type WorkflowStatus =
  | "RUNNING"
  | "COMPLETED"
  | "FAILED"
  | "TIMED_OUT"
  | "CANCELED";

export interface WorkflowSummary {
  workflowId: string;
  runId: string;
  workflowName: string;
  status: WorkflowStatus;
  startTime: number;
  endTime: number;
  result: string;
  error: string;
}

export interface WorkflowEvent {
  eventType: string;
  timestamp: number;
  details: string;
}

export interface WorkflowDetails {
  execution: WorkflowSummary;
  history: WorkflowEvent[];
}

export interface WorkflowMetrics {
  totalWorkflows: number;
  runningWorkflows: number;
  completedWorkflows: number;
  failedWorkflows: number;
  avgExecutionTimeMs: number;
  workflowsByType: Record<string, number>;
}
