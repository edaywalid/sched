import { useEffect, useState } from "react";

export function RelativeTime({ ts }: { ts: number }) {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!ts) return;
    const id = setInterval(() => setNow(Date.now()), 15_000);
    return () => clearInterval(id);
  }, [ts]);

  if (!ts) return <span className="text-zinc-500">-</span>;

  const text = formatRelative(now - ts);
  const absolute = new Date(ts).toLocaleString();
  return <time title={absolute}>{text}</time>;
}

function formatRelative(deltaMs: number): string {
  if (deltaMs < 0) return "just now";
  const s = Math.floor(deltaMs / 1000);
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  const d = Math.floor(h / 24);
  return `${d}d ago`;
}
