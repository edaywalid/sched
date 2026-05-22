import { Link, Outlet, createFileRoute } from "@tanstack/react-router";
import { Logo } from "@sched/design";

export const Route = createFileRoute("/app")({
  component: DashboardLayout,
});

function DashboardLayout() {
  return (
    <div className="flex h-full flex-col">
      <header className="border-b border-zinc-900">
        <div className="mx-auto flex h-14 max-w-6xl items-center gap-6 px-6">
          <Link to="/" className="flex items-center">
            <Logo size="sm" />
          </Link>
          <nav className="flex items-center gap-1 text-sm">
            <Link
              to="/app"
              activeOptions={{ exact: true }}
              activeProps={{ className: "text-zinc-100" }}
              inactiveProps={{ className: "text-zinc-400 hover:text-zinc-200" }}
              className="rounded-md px-2 py-1"
            >
              Workflows
            </Link>
          </nav>
          <div className="ml-auto flex items-center gap-3 text-xs text-zinc-500">
            <span className="hidden sm:inline">connected</span>
            <span className="size-1.5 rounded-full bg-accent-500" />
          </div>
        </div>
      </header>
      <main className="flex-1 overflow-auto">
        <div className="mx-auto max-w-6xl px-6 py-8">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
