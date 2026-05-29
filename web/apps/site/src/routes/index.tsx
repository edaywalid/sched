import { createFileRoute } from "@tanstack/react-router";
import {
  Activity,
  Boxes,
  Database,
  GitBranch,
  Layers,
  RefreshCcw,
  Repeat,
  Workflow,
} from "lucide-react";
import { Logo } from "@sched/design";
import type { ReactNode } from "react";
import { WorkflowDemo } from "../components/WorkflowDemo";
import { SDKTabs } from "../components/SDKTabs";
import { ArchitectureDiagram } from "../components/ArchitectureDiagram";
import { FAQ } from "../components/FAQ";
import { Reveal } from "../components/Reveal";

export const Route = createFileRoute("/")({
  component: LandingPage,
});

function LandingPage() {
  return (
    <>
      <SiteHeader />
      <Hero />
      <Pillars />
      <SDKSection />
      <Architecture />
      <Capabilities />
      <Quickstart />
      <FAQSection />
      <SiteFooter />
    </>
  );
}

function SiteHeader() {
  return (
    <header className="sticky top-0 z-50 border-b border-zinc-900/80 bg-zinc-950/80 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-6xl items-center px-6">
        <Logo size="sm" />
        <nav className="ml-auto flex items-center gap-5 text-sm text-zinc-400">
          <a href="/docs" className="hover:text-zinc-100">
            Docs
          </a>
          <a href="/blog" className="hover:text-zinc-100">
            Blog
          </a>
          <a
            href="https://github.com/edaywalid/sched"
            className="hover:text-zinc-100"
            target="_blank"
            rel="noreferrer"
          >
            GitHub
          </a>
          <a
            href="/docs/get-started/quickstart"
            className="rounded-md border border-zinc-800 bg-zinc-900 px-3 py-1 text-zinc-100 transition-[border-color,transform] duration-150 ease-[cubic-bezier(0.23,1,0.32,1)] hover:border-zinc-700 active:scale-[0.97]"
          >
            Get started
          </a>
        </nav>
      </div>
    </header>
  );
}

function Hero() {
  return (
    <section className="relative overflow-hidden border-b border-zinc-900">
      <BackgroundGrid />
      <div className="relative mx-auto grid max-w-6xl items-center gap-14 px-6 py-24 lg:grid-cols-[1.1fr_1fr] lg:py-32">
        <div>
          <Reveal>
            <h1 className="text-5xl font-medium leading-[1.05] tracking-tight text-zinc-50 sm:text-6xl">
              Workflows that don&rsquo;t lose state.
            </h1>
          </Reveal>
          <Reveal delay={0.1}>
            <p className="mt-6 max-w-xl text-lg leading-relaxed text-zinc-400">
              A workflow engine in Go that runs your functions reliably across
              restarts, retries, and worker failure. Postgres-backed history,
              Redis-stream dispatch, replay on yield, standby HA. Operate it
              yourself.
            </p>
          </Reveal>
          <Reveal delay={0.15}>
            <div className="mt-9 flex flex-wrap items-center gap-3">
              <a
                href="/docs/get-started/quickstart"
                className="inline-flex items-center rounded-md bg-zinc-50 px-4 py-2.5 text-sm font-medium text-zinc-950 transition-[background-color,transform] duration-150 ease-[cubic-bezier(0.23,1,0.32,1)] hover:bg-white active:scale-[0.97]"
              >
                Get started
              </a>
              <a
                href="https://github.com/edaywalid/sched"
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center rounded-md border border-zinc-800 px-4 py-2.5 text-sm text-zinc-200 transition-[border-color,transform] duration-150 ease-[cubic-bezier(0.23,1,0.32,1)] hover:border-zinc-700 active:scale-[0.97]"
              >
                Star on GitHub
              </a>
            </div>
          </Reveal>
          <Reveal delay={0.2}>
            <div className="mt-10 flex items-center gap-6 text-xs text-zinc-500">
              <Stat label="Apache 2.0" />
              <Stat label="Go 1.25+" />
              <Stat label="Postgres + Redis" />
            </div>
          </Reveal>
        </div>

        <Reveal delay={0.18}>
          <WorkflowDemo />
        </Reveal>
      </div>
    </section>
  );
}

function Stat({ label }: { label: string }) {
  return (
    <span className="font-mono uppercase tracking-wider text-zinc-500">
      {label}
    </span>
  );
}

function BackgroundGrid() {
  return (
    <div
      aria-hidden
      className="absolute inset-0 -z-10"
      style={{
        backgroundImage:
          "radial-gradient(circle at 1px 1px, oklch(30% 0 0) 1px, transparent 0)",
        backgroundSize: "32px 32px",
        maskImage:
          "radial-gradient(ellipse 80% 60% at 50% 30%, black 30%, transparent 80%)",
      }}
    />
  );
}

function Pillars() {
  return (
    <section className="border-b border-zinc-900">
      <div className="mx-auto max-w-6xl px-6 py-24">
        <Reveal>
          <h2 className="max-w-2xl text-3xl font-medium tracking-tight text-zinc-100 sm:text-4xl">
            Three ideas the engine takes seriously.
          </h2>
        </Reveal>

        <div className="mt-14 grid gap-y-10 md:grid-cols-3 md:gap-x-12 md:divide-x md:divide-zinc-900">
          <PillarItem
            icon={Database}
            title="Durable state"
            body="Every state transition persists to Postgres before the RPC returns. Workflows are append-only event logs. A restart loses nothing."
          />
          <PillarItem
            icon={Repeat}
            title="Replay on yield"
            body="Workflow functions are deterministic over their event history. A worker dies, the engine re-dispatches against the same history, recorded commands become no-ops on the second run."
          />
          <PillarItem
            icon={Activity}
            title="Standby HA"
            body="Engine replicas hold a Postgres advisory lock. The leader serves traffic, standbys idle. Lose the leader and one promotes within a single retry interval."
          />
        </div>
      </div>
    </section>
  );
}

function PillarItem({
  icon: Icon,
  title,
  body,
}: {
  icon: typeof Activity;
  title: string;
  body: string;
}) {
  return (
    <div className="md:px-6 md:first:pl-0">
      <Icon className="size-5 text-accent-300" strokeWidth={1.75} />
      <h3 className="mt-5 text-base font-medium text-zinc-100">{title}</h3>
      <p className="mt-3 text-sm leading-relaxed text-zinc-400">{body}</p>
    </div>
  );
}

function SDKSection() {
  return (
    <section className="border-b border-zinc-900">
      <div className="mx-auto max-w-6xl px-6 py-24">
        <div className="grid gap-12 lg:grid-cols-[1fr_1.4fr]">
          <Reveal>
            <h2 className="text-3xl font-medium tracking-tight text-zinc-100 sm:text-4xl">
              Write workflows as plain Go.
            </h2>
            <p className="mt-4 text-zinc-400">
              A workflow is a function. Inside it, the SDK exposes a few
              durable primitives: schedule an activity, sleep, wait for a
              signal. Each one is recorded in history; the engine replays the
              function against that history on re-dispatch.
            </p>
            <p className="mt-4 text-zinc-400">
              No DSL, no YAML, no state machine generator. Just Go with an
              honest set of constraints.
            </p>
          </Reveal>
          <Reveal delay={0.1}>
            <SDKTabs />
          </Reveal>
        </div>
      </div>
    </section>
  );
}

function Architecture() {
  return (
    <section className="border-b border-zinc-900">
      <div className="mx-auto max-w-6xl px-6 py-24">
        <Reveal>
          <SectionEyebrow>Architecture</SectionEyebrow>
          <h2 className="mt-3 max-w-2xl text-3xl font-medium tracking-tight text-zinc-100 sm:text-4xl">
            Three processes, two data stores, one binary.
          </h2>
          <p className="mt-4 max-w-2xl text-zinc-400">
            Workers run your code and poll the engine over bidi gRPC streams.
            The engine owns the durable side: it writes history to Postgres,
            queues tasks on Redis Streams, and emits metrics and traces. No
            sidecars, no coordinator, no leader election service beyond a
            Postgres advisory lock.
          </p>
        </Reveal>
        <Reveal delay={0.1} className="mt-12">
          <ArchitectureDiagram />
        </Reveal>
      </div>
    </section>
  );
}

function Capabilities() {
  const items = [
    {
      icon: Workflow,
      title: "Durable workflow execution",
      body: "Workflows survive engine restart, worker crash, and network blips. Every transition is in the event log.",
    },
    {
      icon: Repeat,
      title: "Retries with exponential backoff",
      body: "Failing activities are retried per the registered RetryPolicy. Backoff lives in a real timer, not a sleep loop.",
    },
    {
      icon: RefreshCcw,
      title: "Signals and waits",
      body: "WaitForSignal yields the workflow function. When the signal arrives the engine re-dispatches; replay returns the recorded value.",
    },
    {
      icon: GitBranch,
      title: "Bidi-streamed dispatch",
      body: "Workers open a stream once and ack credits per task. Sub-millisecond delivery latency. Graceful shutdown is instant.",
    },
    {
      icon: Boxes,
      title: "Activity heartbeats",
      body: "Long-running activities heartbeat to extend visibility. Cancellation propagates via the same channel.",
    },
    {
      icon: Layers,
      title: "Full observability",
      body: "Structured slog logs, Prometheus metrics on /metrics, OpenTelemetry tracing across engine/worker/dashboard.",
    },
  ];
  return (
    <section className="border-b border-zinc-900">
      <div className="mx-auto max-w-6xl px-6 py-24">
        <Reveal>
          <h2 className="max-w-2xl text-3xl font-medium tracking-tight text-zinc-100 sm:text-4xl">
            What the engine does for you.
          </h2>
        </Reveal>

        <div className="mt-14 grid grid-cols-1 md:grid-cols-2">
          {items.map((item, i) => (
            <Reveal key={item.title} delay={i * 0.04}>
              <div
                className={[
                  "flex gap-5 py-7",
                  i % 2 === 0 ? "md:pr-10 md:border-r md:border-zinc-900" : "md:pl-10",
                  i >= 2 ? "border-t border-zinc-900" : "",
                  i === 1 ? "border-t border-zinc-900 md:border-t-0" : "",
                ].join(" ")}
              >
                <div className="shrink-0">
                  <span className="flex size-9 items-center justify-center rounded-lg border border-zinc-800 bg-zinc-950">
                    <item.icon className="size-4 text-accent-300" strokeWidth={1.75} />
                  </span>
                </div>
                <div className="min-w-0">
                  <h3 className="text-sm font-medium text-zinc-100">{item.title}</h3>
                  <p className="mt-2 text-sm leading-relaxed text-zinc-400">{item.body}</p>
                </div>
              </div>
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  );
}

function Quickstart() {
  const STEPS = [
    {
      n: "01",
      title: "Clone and bring up the stack.",
      code: `git clone https://github.com/edaywalid/sched.git
cd sched
make up`,
    },
    {
      n: "02",
      title: "Build the dashboard bundle.",
      code: `make web-build
docker compose build dashboard
docker compose up -d dashboard`,
    },
    {
      n: "03",
      title: "Open the dashboard and start a workflow.",
      code: `# Visit http://localhost:8080
# Or queue one via the API:
curl -X POST http://localhost:8080/api/workflows \\
  -H "Content-Type: application/json" \\
  -d '{"workflowName":"HelloWorld","input":"world"}'`,
    },
  ];
  return (
    <section className="border-b border-zinc-900">
      <div className="mx-auto max-w-6xl px-6 py-24">
        <Reveal>
          <h2 className="text-3xl font-medium tracking-tight text-zinc-100 sm:text-4xl">
            Three commands to a running engine.
          </h2>
        </Reveal>
        <div className="mt-12 grid gap-6 lg:grid-cols-3">
          {STEPS.map((step, i) => (
            <Reveal key={step.n} delay={i * 0.05}>
              <div className="rounded-xl border border-zinc-900 bg-zinc-950 p-6">
                <span className="font-mono text-xs uppercase tracking-wider text-accent-500">
                  {step.n}
                </span>
                <h3 className="mt-3 text-sm font-medium text-zinc-100">
                  {step.title}
                </h3>
                <pre className="mt-4 overflow-auto rounded-lg border border-zinc-900 bg-black/40 px-3 py-3 font-mono text-[11.5px] leading-relaxed text-zinc-300">
                  {step.code}
                </pre>
              </div>
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  );
}

function FAQSection() {
  return (
    <section className="border-b border-zinc-900">
      <div className="mx-auto max-w-3xl px-6 py-24">
        <Reveal>
          <SectionEyebrow>Frequently asked</SectionEyebrow>
          <h2 className="mt-3 text-3xl font-medium tracking-tight text-zinc-100 sm:text-4xl">
            Honest answers.
          </h2>
        </Reveal>
        <Reveal delay={0.05} className="mt-12">
          <FAQ />
        </Reveal>
      </div>
    </section>
  );
}

function SectionEyebrow({ children }: { children: ReactNode }) {
  return (
    <div className="font-mono text-xs uppercase tracking-wider text-accent-500">
      {children}
    </div>
  );
}

function SiteFooter() {
  return (
    <footer className="bg-zinc-950">
      <div className="mx-auto max-w-6xl px-6 py-16">
        <div className="grid gap-10 sm:grid-cols-[2fr_1fr_1fr_1fr]">
          <div>
            <Logo />
            <p className="mt-4 max-w-sm text-sm text-zinc-500">
              Durable workflow orchestration in Go. Apache 2.0. Operate it
              yourself.
            </p>
          </div>
          <FooterCol
            title="Product"
            links={[
              { label: "Docs", href: "/docs" },
              { label: "Quickstart", href: "/docs/get-started/quickstart" },
              {
                label: "Changelog",
                href: "https://github.com/edaywalid/sched/blob/main/CHANGELOG.md",
              },
              { label: "Roadmap", href: "/docs/architecture/overview" },
            ]}
          />
          <FooterCol
            title="Resources"
            links={[
              { label: "Blog", href: "/blog" },
              { label: "GitHub", href: "https://github.com/edaywalid/sched" },
              {
                label: "Issues",
                href: "https://github.com/edaywalid/sched/issues",
              },
            ]}
          />
          <FooterCol
            title="Legal"
            links={[
              {
                label: "License",
                href: "https://github.com/edaywalid/sched/blob/main/LICENSE",
              },
            ]}
          />
        </div>
        <div className="mt-14 flex flex-wrap items-center justify-between gap-3 border-t border-zinc-900 pt-8 text-xs text-zinc-600">
          <span>
            © {new Date().getFullYear()} sched contributors. Built in Go.
          </span>
          <span className="font-mono">Apache 2.0</span>
        </div>
      </div>
    </footer>
  );
}

function FooterCol({
  title,
  links,
}: {
  title: string;
  links: Array<{ label: string; href: string }>;
}) {
  return (
    <div>
      <div className="text-xs font-medium uppercase tracking-wider text-zinc-400">
        {title}
      </div>
      <ul className="mt-4 flex flex-col gap-2 text-sm">
        {links.map((link) => (
          <li key={link.href}>
            <a
              href={link.href}
              className="text-zinc-400 transition-colors hover:text-zinc-100"
            >
              {link.label}
            </a>
          </li>
        ))}
      </ul>
    </div>
  );
}
