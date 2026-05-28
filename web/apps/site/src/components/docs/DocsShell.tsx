import { Link, useRouterState } from "@tanstack/react-router";
import { Logo } from "@sched/design";
import type { ReactNode } from "react";

type NavGroup = {
  title: string;
  items: Array<{ label: string; to: string; soon?: boolean }>;
};

const NAV: NavGroup[] = [
  {
    title: "Get started",
    items: [
      { label: "Overview", to: "/docs" },
      { label: "Quickstart", to: "/docs/get-started/quickstart" },
      { label: "Install", to: "/docs/get-started/install", soon: true },
    ],
  },
  {
    title: "Concepts",
    items: [
      { label: "Workflows", to: "/docs/concepts/workflows" },
      { label: "Activities", to: "/docs/concepts/activities", soon: true },
      { label: "Signals", to: "/docs/concepts/signals", soon: true },
      { label: "Timers", to: "/docs/concepts/timers", soon: true },
    ],
  },
  {
    title: "Architecture",
    items: [
      { label: "Overview", to: "/docs/architecture/overview" },
      { label: "Replay model", to: "/docs/architecture/replay", soon: true },
      { label: "Persistence", to: "/docs/architecture/persistence", soon: true },
    ],
  },
  {
    title: "Operating",
    items: [
      { label: "Observability", to: "/docs/operating/observability", soon: true },
      { label: "High availability", to: "/docs/operating/ha", soon: true },
      { label: "Configuration", to: "/docs/reference/configuration", soon: true },
    ],
  },
];

export function DocsShell({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-screen bg-zinc-950 text-zinc-100">
      <DocsHeader />
      <div className="mx-auto grid max-w-6xl gap-10 px-6 py-10 lg:grid-cols-[14rem_1fr]">
        <Sidebar />
        <main className="min-w-0">
          <article>{children}</article>
        </main>
      </div>
    </div>
  );
}

function DocsHeader() {
  return (
    <header className="sticky top-0 z-50 border-b border-zinc-900/80 bg-zinc-950/80 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-6xl items-center px-6">
        <Link to="/" className="flex items-center">
          <Logo size="sm" />
        </Link>
        <span className="ml-3 text-sm text-zinc-500">/ docs</span>
        <nav className="ml-auto flex items-center gap-5 text-sm text-zinc-400">
          <Link to="/blog" className="hover:text-zinc-100">
            Blog
          </Link>
          <a
            href="https://github.com/edaywalid/sched"
            target="_blank"
            rel="noreferrer"
            className="hover:text-zinc-100"
          >
            GitHub
          </a>
        </nav>
      </div>
    </header>
  );
}

function Sidebar() {
  const path = useRouterState({ select: (s) => s.location.pathname });
  return (
    <aside className="hidden lg:block">
      <nav className="sticky top-20 flex flex-col gap-7 text-sm">
        {NAV.map((group) => (
          <div key={group.title}>
            <div className="mb-2 font-mono text-[10px] uppercase tracking-wider text-zinc-500">
              {group.title}
            </div>
            <ul className="flex flex-col gap-0.5">
              {group.items.map((item) => {
                const active = path === item.to;
                return (
                  <li key={item.to}>
                    {item.soon ? (
                      <span className="flex items-center justify-between rounded px-2 py-1 text-zinc-600">
                        <span>{item.label}</span>
                        <span className="text-[9px] uppercase tracking-wider text-zinc-700">
                          soon
                        </span>
                      </span>
                    ) : (
                      <Link
                        to={item.to}
                        className={
                          active
                            ? "block rounded bg-zinc-900 px-2 py-1 text-zinc-100"
                            : "block rounded px-2 py-1 text-zinc-400 hover:bg-zinc-900/60 hover:text-zinc-100"
                        }
                      >
                        {item.label}
                      </Link>
                    )}
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </nav>
    </aside>
  );
}
