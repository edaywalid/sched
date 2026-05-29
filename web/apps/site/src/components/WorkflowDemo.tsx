import { useEffect, useState } from "react";
import { AnimatePresence, motion } from "motion/react";

interface Event {
  id: number;
  type: string;
  detail: string;
}

const SCRIPT: Event[] = [
  { id: 1, type: "WorkflowStarted", detail: "MonthlyReport" },
  { id: 2, type: "ActivityScheduled", detail: "SendEmail · user1" },
  { id: 3, type: "TimerScheduled", detail: "sleep 2s" },
  { id: 4, type: "WorkflowTaskYielded", detail: "" },
  { id: 5, type: "ActivityCompleted", detail: "SendEmail · user1" },
  { id: 6, type: "TimerFired", detail: "" },
  { id: 7, type: "ActivityScheduled", detail: "SendEmail · user2" },
  { id: 8, type: "ActivityCompleted", detail: "SendEmail · user2" },
  { id: 9, type: "WorkflowCompleted", detail: "report complete" },
];

const TONE: Record<string, string> = {
  WorkflowStarted: "bg-accent-500",
  WorkflowCompleted: "bg-emerald-400",
  WorkflowFailed: "bg-rose-400",
  WorkflowTaskYielded: "bg-zinc-500",
  ActivityScheduled: "bg-sky-400",
  ActivityCompleted: "bg-emerald-400",
  TimerScheduled: "bg-violet-400",
  TimerFired: "bg-violet-300",
};

export function WorkflowDemo() {
  const [step, setStep] = useState(0);

  useEffect(() => {
    const id = setInterval(() => {
      setStep((s) => (s + 1) % (SCRIPT.length + 2));
    }, 900);
    return () => clearInterval(id);
  }, []);

  const visible = SCRIPT.slice(0, Math.min(step, SCRIPT.length));
  const isComplete = visible.length === SCRIPT.length;

  return (
    <div className="overflow-hidden rounded-xl border border-zinc-800 bg-zinc-950 shadow-[0_0_0_1px_rgba(255,255,255,0.02),0_20px_60px_-30px_rgba(0,0,0,0.6)]">
      <div className="flex items-center justify-between border-b border-zinc-900 px-4 py-2.5">
        <div className="flex items-center gap-2">
          <span className="font-mono text-xs text-zinc-400">MonthlyReport</span>
          <span className="font-mono text-[10px] text-zinc-400">
            wf-1a2b3c
          </span>
        </div>
        <StatusPill status={isComplete ? "COMPLETED" : "RUNNING"} />
      </div>

      <ol className="flex h-72 flex-col gap-1 overflow-hidden px-4 py-3">
        <AnimatePresence initial={false}>
          {visible.map((ev) => (
            <motion.li
              key={ev.id}
              initial={{ opacity: 0, x: -8, height: 0 }}
              animate={{ opacity: 1, x: 0, height: "auto" }}
              transition={{ duration: 0.22 }}
              className="flex items-baseline gap-2.5 text-xs"
            >
              <span
                className={`mt-1 size-1.5 shrink-0 rounded-full ${
                  TONE[ev.type] ?? "bg-zinc-500"
                }`}
              />
              <span className="text-zinc-200">{ev.type}</span>
              {ev.detail ? (
                <span className="truncate font-mono text-[10px] text-zinc-500">
                  {ev.detail}
                </span>
              ) : null}
              <span className="ml-auto font-mono text-[10px] text-zinc-400">
                +{ev.id * 200}ms
              </span>
            </motion.li>
          ))}
        </AnimatePresence>

        {!isComplete && visible.length > 0 ? (
          <motion.li
            initial={{ opacity: 0 }}
            animate={{ opacity: 0.5 }}
            className="mt-1 flex items-baseline gap-2.5 text-xs text-zinc-400"
          >
            <span className="mt-1 size-1.5 shrink-0 animate-pulse rounded-full bg-accent-500" />
            <span>awaiting…</span>
          </motion.li>
        ) : null}
      </ol>

      <div className="border-t border-zinc-900 px-4 py-2 font-mono text-[10px] text-zinc-400">
        {visible.length}/{SCRIPT.length} events
      </div>
    </div>
  );
}

function StatusPill({ status }: { status: "RUNNING" | "COMPLETED" }) {
  const styles =
    status === "RUNNING"
      ? "border-accent-700 bg-accent-700/15 text-accent-300"
      : "border-emerald-800 bg-emerald-900/30 text-emerald-300";
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-[10px] ${styles}`}
    >
      <motion.span
        className="size-1.5 rounded-full bg-current"
        animate={status === "RUNNING" ? { opacity: [0.4, 1, 0.4] } : { opacity: 1 }}
        transition={
          status === "RUNNING"
            ? { duration: 1.6, repeat: Infinity, ease: "easeInOut" }
            : undefined
        }
      />
      {status === "RUNNING" ? "Running" : "Completed"}
    </span>
  );
}
