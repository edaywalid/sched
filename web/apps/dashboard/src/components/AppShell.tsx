import { Link } from "@tanstack/react-router";
import { Logo } from "@sched/design";
import {
  Activity,
  GitBranch,
  Layers,
  ListChecks,
  Settings,
  Workflow,
} from "lucide-react";
import type { ReactNode } from "react";
import { SystemStatus } from "./SystemStatus";
import { ActivityRail } from "./ActivityRail";

const NAV: Array<{
  label: string;
  to: string;
  icon: typeof Activity;
}> = [
  { label: "Overview", to: "/app", icon: Layers },
  { label: "Workflows", to: "/app", icon: Workflow },
  { label: "Activities", to: "/app", icon: ListChecks },
  { label: "Queues", to: "/app", icon: GitBranch },
  { label: "Settings", to: "/app", icon: Settings },
];

export function AppShell({ children }: { children: ReactNode }) {
  return (
    <div className="grid h-full min-h-0 grid-cols-[15rem_minmax(0,1fr)] bg-zinc-950 text-zinc-100">
      <Sidebar />
      <div className="flex min-h-0 flex-col">
        <TopBar />
        <main className="min-h-0 flex-1 overflow-auto">
          <div className="mx-auto max-w-6xl px-8 py-8">{children}</div>
        </main>
      </div>
    </div>
  );
}

function Sidebar() {
  return (
    <aside className="flex h-full flex-col border-r border-zinc-900 bg-zinc-950">
      <div className="flex h-14 items-center border-b border-zinc-900 px-5">
        <Link to="/" className="flex items-center">
          <Logo size="sm" />
        </Link>
      </div>

      <nav className="flex-1 overflow-auto px-3 py-4">
        <ul className="flex flex-col gap-0.5">
          {NAV.map(({ label, to, icon: Icon }) => (
            <li key={label}>
              <Link
                to={to}
                activeOptions={{ exact: to === "/app" }}
                activeProps={{
                  className:
                    "relative bg-zinc-900/60 text-zinc-100 before:absolute before:inset-y-1 before:left-0 before:w-0.5 before:rounded-r before:bg-accent-500",
                }}
                inactiveProps={{
                  className: "text-zinc-400 hover:bg-zinc-900/40 hover:text-zinc-100",
                }}
                className="flex items-center gap-2.5 rounded-md px-3 py-1.5 text-sm transition-colors"
              >
                <Icon className="size-3.5" strokeWidth={1.75} />
                {label}
              </Link>
            </li>
          ))}
        </ul>

        <div className="mt-8">
          <div className="px-3 pb-2 text-[10px] font-medium uppercase tracking-wider text-zinc-600">
            Activity
          </div>
          <ActivityRail />
        </div>
      </nav>

      <div className="border-t border-zinc-900 px-5 py-3 text-[11px] text-zinc-600">
        <SystemStatus compact />
      </div>
    </aside>
  );
}

function TopBar() {
  return (
    <header className="flex h-14 items-center justify-between border-b border-zinc-900 px-8">
      <div className="font-mono text-xs uppercase tracking-wider text-zinc-500">
        Workspace · default
      </div>
      <SystemStatus />
    </header>
  );
}
