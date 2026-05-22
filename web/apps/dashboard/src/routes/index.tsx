import { Link, createFileRoute } from "@tanstack/react-router";
import { Logo } from "@sched/design";
import { CodeBlock } from "../components/CodeBlock";

export const Route = createFileRoute("/")({
  component: LandingPage,
});

const FEATURES = [
  {
    title: "Durable state",
    body: "Every workflow transition is persisted in Postgres before the RPC returns. A restart loses no work.",
  },
  {
    title: "Replay on yield",
    body: "Workflow functions are pure over (input, history). When a worker dies mid-Sleep, another worker resumes against the same history.",
  },
  {
    title: "Streamed dispatch",
    body: "Bidi gRPC streams replace 60-second long polls. Task delivery latency is sub-millisecond and graceful shutdown is instant.",
  },
  {
    title: "Standby HA",
    body: "Engine instances hold a Postgres advisory lock. Lose the leader and a standby takes over within a single retry interval.",
  },
];

const WORKFLOW_EXAMPLE = `client.RegisterWorkflow("MonthlyReport", func(ctx sdk.WorkflowContext, input any) (any, error) {
    for i := range 3 {
        ctx.QueueActivity("SendEmail", fmt.Sprintf("user%d@example.com", i))
        ctx.Sleep(2 * time.Second)
    }
    return "report complete", nil
})`;

function LandingPage() {
  return (
    <div className="min-h-full">
      <header className="border-b border-zinc-900">
        <div className="mx-auto flex h-14 max-w-5xl items-center px-6">
          <Logo size="sm" />
          <nav className="ml-auto flex items-center gap-5 text-sm text-zinc-400">
            <a
              href="https://github.com/edaywalid/sched"
              className="hover:text-zinc-100"
              target="_blank"
              rel="noreferrer"
            >
              Source
            </a>
            <Link
              to="/app"
              className="rounded-md border border-zinc-800 bg-zinc-900 px-3 py-1 text-zinc-100 hover:border-zinc-700"
            >
              Dashboard
            </Link>
          </nav>
        </div>
      </header>

      <section className="border-b border-zinc-900">
        <div className="mx-auto max-w-3xl px-6 py-24">
          <div className="inline-flex items-center gap-2 rounded-full border border-zinc-800 bg-zinc-950 px-2.5 py-1 text-xs text-zinc-400">
            <span className="size-1.5 rounded-full bg-accent-500" />
            v2 in development
          </div>
          <h1 className="mt-6 text-4xl font-medium leading-tight tracking-tight text-zinc-50 sm:text-5xl">
            Durable workflow orchestration in Go.
          </h1>
          <p className="mt-5 max-w-xl text-zinc-400">
            A workflow engine that runs your Go functions reliably across
            restarts, retries, and failure. Postgres-backed history, Redis
            stream dispatch, replay-on-yield, standby HA.
          </p>
          <div className="mt-8 flex flex-wrap items-center gap-3">
            <Link
              to="/app"
              className="inline-flex items-center rounded-md bg-zinc-100 px-3.5 py-2 text-sm font-medium text-zinc-950 hover:bg-white"
            >
              Open dashboard
            </Link>
          </div>
        </div>
      </section>

      <section className="border-b border-zinc-900">
        <div className="mx-auto max-w-5xl px-6 py-20">
          <div className="grid gap-12 lg:grid-cols-[1fr_2fr]">
            <div>
              <div className="font-mono text-xs uppercase tracking-wider text-zinc-500">
                Example
              </div>
              <h2 className="mt-2 text-2xl font-medium text-zinc-100">
                Write workflows as plain Go.
              </h2>
              <p className="mt-3 text-sm text-zinc-400">
                A workflow is a function. Calls to <code className="rounded bg-zinc-900 px-1 py-0.5 font-mono text-xs text-zinc-200">QueueActivity</code> and <code className="rounded bg-zinc-900 px-1 py-0.5 font-mono text-xs text-zinc-200">Sleep</code> are recorded in durable history.
              </p>
            </div>
            <CodeBlock language="go" code={WORKFLOW_EXAMPLE} />
          </div>
        </div>
      </section>

      <section className="border-b border-zinc-900">
        <div className="mx-auto max-w-5xl px-6 py-20">
          <div className="font-mono text-xs uppercase tracking-wider text-zinc-500">
            Capabilities
          </div>
          <h2 className="mt-2 text-2xl font-medium text-zinc-100">
            What the engine does for you.
          </h2>
          <div className="mt-10 grid gap-px overflow-hidden rounded-md border border-zinc-900 bg-zinc-900 sm:grid-cols-2">
            {FEATURES.map((f) => (
              <article key={f.title} className="bg-zinc-950 p-6">
                <h3 className="text-sm font-medium text-zinc-100">{f.title}</h3>
                <p className="mt-2 text-sm leading-relaxed text-zinc-400">{f.body}</p>
              </article>
            ))}
          </div>
        </div>
      </section>
    </div>
  );
}
