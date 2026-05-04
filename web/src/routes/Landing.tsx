import { Link } from "react-router";

// Stub for Phase 6.1. The real landing page is built in Phase 6.2.
export function LandingPage() {
  return (
    <main className="mx-auto max-w-3xl px-6 py-24">
      <div className="font-mono text-xs uppercase tracking-wider text-zinc-500">
        sched / v2-dev
      </div>
      <h1 className="mt-2 text-4xl font-medium tracking-tight text-zinc-100">
        Durable workflow orchestration in Go.
      </h1>
      <p className="mt-4 text-zinc-400">
        Landing page placeholder. Real content arrives in 6.2.
      </p>
      <Link
        to="/app"
        className="mt-8 inline-flex items-center gap-2 rounded-md border border-zinc-800 bg-zinc-900 px-3 py-1.5 text-sm text-zinc-100 hover:border-zinc-700"
      >
        Open dashboard
      </Link>
    </main>
  );
}
