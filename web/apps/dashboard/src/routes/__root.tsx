import { Outlet, createRootRoute } from "@tanstack/react-router";
import { Link } from "@tanstack/react-router";

export const Route = createRootRoute({
  component: RootComponent,
  notFoundComponent: NotFound,
});

function RootComponent() {
  return <Outlet />;
}

function NotFound() {
  return (
    <main className="mx-auto flex h-full max-w-2xl flex-col items-start justify-center gap-3 px-6">
      <div className="font-mono text-xs uppercase tracking-wider text-zinc-500">
        404
      </div>
      <h1 className="text-2xl font-medium text-zinc-100">Nothing here.</h1>
      <p className="text-sm text-zinc-400">
        The page you were looking for does not exist.
      </p>
      <Link
        to="/"
        className="mt-2 text-sm text-accent-300 underline-offset-4 hover:underline"
      >
        Back to home
      </Link>
    </main>
  );
}
