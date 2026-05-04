import { useParams, Link } from "react-router";

export function WorkflowDetailPage() {
  const { id } = useParams();
  return (
    <div className="space-y-6">
      <Link
        to="/app"
        className="text-xs text-zinc-500 hover:text-zinc-200"
      >
        ← Workflows
      </Link>
      <h1 className="font-mono text-sm text-zinc-300">{id}</h1>
      <p className="text-sm text-zinc-500">
        History, signal, and cancel UI ships in Phase 6.3.
      </p>
    </div>
  );
}
