import { useState } from "react";
import { motion, AnimatePresence } from "motion/react";
import { Plus } from "lucide-react";

const ITEMS = [
  {
    q: "Is sched production-ready?",
    a: "Not yet. The engine is in v2 development. Phase 1 through Phase 4.a have shipped (durable state, retries, signals, durable timers, replay-on-yield, leader-election HA). Phase 4.b (sharding) is still ahead. We track every shipped capability in the README's status table.",
  },
  {
    q: "How does this compare to Temporal?",
    a: "Same problem space, much smaller surface. Temporal is a fully-managed multi-tenant platform with namespaces, advanced versioning, and a polyglot SDK story. sched is a single Go SDK against a single engine binary you operate yourself. The replay-on-yield model is intentionally similar; the operational footprint is intentionally not.",
  },
  {
    q: "What does the engine actually persist?",
    a: "Every workflow execution row plus its full event history is in Postgres. Timers live in Postgres so they survive engine restart. The Redis Streams queue carries tasks in-flight; if the engine restarts mid-dispatch the visibility timeout reclaims un-acked work.",
  },
  {
    q: "What happens when a worker crashes mid-workflow?",
    a: "The engine's reclaim loop notices the un-acked task after the visibility timeout, re-dispatches it, and a healthy worker re-runs the workflow function against the same history. The replay machinery short-circuits any commands already recorded, so retries are idempotent.",
  },
  {
    q: "Can I run more than one engine for HA?",
    a: "Yes, in active-passive mode today. Standby engines hold a Postgres advisory lock; whichever one acquires it is the leader. If the leader dies the lease releases and a standby promotes within a single retry interval. True multi-active sharding is Phase 4.b.",
  },
  {
    q: "What's the observability story?",
    a: "Structured slog logs (JSON in prod), Prometheus metrics on /metrics (workflows started, completed by status, activities executed, queue poll latency, activity duration), and OpenTelemetry tracing across engine / worker / dashboard with a Jaeger profile in docker-compose.",
  },
];

export function FAQ() {
  return (
    <div className="divide-y divide-zinc-900 overflow-hidden rounded-xl border border-zinc-900 bg-zinc-950">
      {ITEMS.map((item, i) => (
        <FAQItem key={i} q={item.q} a={item.a} />
      ))}
    </div>
  );
}

function FAQItem({ q, a }: { q: string; a: string }) {
  const [open, setOpen] = useState(false);
  return (
    <div>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center justify-between gap-4 px-6 py-4 text-left transition-colors hover:bg-zinc-900/40"
      >
        <span className="text-sm font-medium text-zinc-100">{q}</span>
        <motion.span
          animate={{ rotate: open ? 45 : 0 }}
          transition={{ duration: 0.2 }}
          className="shrink-0 text-zinc-500"
        >
          <Plus className="size-4" strokeWidth={1.75} />
        </motion.span>
      </button>
      <AnimatePresence initial={false}>
        {open ? (
          <motion.div
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.22 }}
            className="overflow-hidden"
          >
            <p className="px-6 pb-5 text-sm leading-relaxed text-zinc-400">
              {a}
            </p>
          </motion.div>
        ) : null}
      </AnimatePresence>
    </div>
  );
}
