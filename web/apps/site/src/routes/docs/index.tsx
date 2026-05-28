import { createFileRoute, Link } from "@tanstack/react-router";

export const Route = createFileRoute("/docs/")({
  component: DocsIndex,
});

const SECTIONS = [
  {
    title: "Get started",
    body: "Bring sched up on your laptop and run your first workflow.",
    links: [
      { label: "Install", to: "/docs/get-started/install" },
      { label: "Quickstart", to: "/docs/get-started/quickstart" },
    ],
  },
  {
    title: "Concepts",
    body: "How workflows, activities, signals, and timers behave under replay.",
    links: [
      { label: "Workflows", to: "/docs/concepts/workflows" },
      { label: "Activities", to: "/docs/concepts/activities" },
      { label: "Signals", to: "/docs/concepts/signals" },
      { label: "Timers", to: "/docs/concepts/timers" },
    ],
  },
  {
    title: "Architecture",
    body: "The engine, the replay model, and the durable persistence layer.",
    links: [
      { label: "Overview", to: "/docs/architecture/overview" },
      { label: "Replay model", to: "/docs/architecture/replay" },
      { label: "Persistence", to: "/docs/architecture/persistence" },
    ],
  },
  {
    title: "Operating",
    body: "Observability, high availability, and the full configuration surface.",
    links: [
      { label: "Observability", to: "/docs/operating/observability" },
      { label: "High availability", to: "/docs/operating/ha" },
      { label: "Configuration", to: "/docs/reference/configuration" },
    ],
  },
];

function DocsIndex() {
  return (
    <div>
      <h1 className="text-3xl font-medium tracking-tight text-zinc-50">
        Documentation
      </h1>
      <p className="mt-4 max-w-2xl leading-relaxed text-zinc-400">
        A short guide to running sched, writing workflows against it, and
        understanding the moving pieces. The docs grow as the engine does.
      </p>

      <div className="mt-12 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {SECTIONS.map((s) => (
          <div
            key={s.title}
            className="rounded-xl border border-zinc-900 bg-zinc-950 p-5"
          >
            <div className="font-mono text-[10px] uppercase tracking-wider text-accent-500">
              Section
            </div>
            <h2 className="mt-2 text-sm font-medium text-zinc-100">
              {s.title}
            </h2>
            <p className="mt-2 text-sm leading-relaxed text-zinc-400">
              {s.body}
            </p>
            <div className="mt-4 flex flex-col gap-1.5">
              {s.links.map((l) => (
                <Link
                  key={l.to}
                  to={l.to}
                  className="text-sm text-accent-400 hover:underline"
                >
                  {l.label} &rarr;
                </Link>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
