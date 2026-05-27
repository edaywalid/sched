import { useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import type { WorkflowEvent } from "../api/types";

const EVENT_TONE: Record<string, string> = {
  WorkflowStarted: "bg-accent-500",
  WorkflowCompleted: "bg-emerald-400",
  WorkflowFailed: "bg-rose-400",
  WorkflowTimedOut: "bg-amber-400",
  WorkflowCanceled: "bg-zinc-400",
  WorkflowCancelRequested: "bg-zinc-500",
  WorkflowTaskYielded: "bg-zinc-500",
  ActivityScheduled: "bg-sky-400",
  ActivityCompleted: "bg-emerald-400",
  ActivityFailed: "bg-rose-400",
  ActivityRetryScheduled: "bg-amber-400",
  TimerScheduled: "bg-violet-400",
  TimerFired: "bg-violet-300",
  SignalReceived: "bg-sky-300",
};

export function HistoryTimeline({ events }: { events: WorkflowEvent[] }) {
  if (events.length === 0) {
    return (
      <div className="rounded-md border border-zinc-900 bg-zinc-950 px-4 py-8 text-center text-sm text-zinc-500">
        No history yet.
      </div>
    );
  }

  return (
    <ol className="overflow-hidden rounded-lg border border-zinc-900 bg-zinc-950">
      <AnimatePresence initial={false}>
        {events.map((ev, i) => (
          <motion.div
            key={`${ev.timestamp}-${i}`}
            initial={{ opacity: 0, x: -6 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ duration: 0.18 }}
          >
            <HistoryRow
              event={ev}
              isFirst={i === 0}
              isLast={i === events.length - 1}
            />
          </motion.div>
        ))}
      </AnimatePresence>
    </ol>
  );
}

function HistoryRow({
  event,
  isFirst,
  isLast,
}: {
  event: WorkflowEvent;
  isFirst: boolean;
  isLast: boolean;
}) {
  const [expanded, setExpanded] = useState(false);
  const tone = EVENT_TONE[event.eventType] ?? "bg-zinc-500";
  const hasDetails = Boolean(event.details && event.details !== "null");

  return (
    <li
      className={[
        "relative px-4 py-3",
        isFirst ? "" : "border-t border-zinc-900",
      ].join(" ")}
    >
      <div className="flex items-start gap-3">
        <div className="relative flex shrink-0 flex-col items-center">
          <span className={`mt-1.5 size-2 rounded-full ${tone}`} />
          {!isLast ? (
            <span className="absolute left-1/2 top-4 h-full w-px -translate-x-1/2 bg-zinc-900" />
          ) : null}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-baseline justify-between gap-3">
            <span className="text-sm text-zinc-100">{event.eventType}</span>
            <span className="font-mono text-xs text-zinc-500">
              {formatTime(event.timestamp)}
            </span>
          </div>
          {hasDetails ? (
            <button
              type="button"
              onClick={() => setExpanded((v) => !v)}
              className="mt-1 text-xs text-zinc-500 hover:text-zinc-300"
            >
              {expanded ? "Hide details" : "Show details"}
            </button>
          ) : null}
          {expanded && hasDetails ? (
            <pre className="mt-2 overflow-auto rounded-md border border-zinc-900 bg-black/30 p-3 font-mono text-xs text-zinc-300">
              {formatJSON(event.details)}
            </pre>
          ) : null}
        </div>
      </div>
    </li>
  );
}

function formatTime(ts: number): string {
  if (!ts) return "";
  const d = new Date(ts);
  return d.toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function formatJSON(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}
