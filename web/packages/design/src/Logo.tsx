interface LogoProps {
  size?: "sm" | "md";
  tone?: "default" | "muted";
}

export function Logo({ size = "md", tone = "default" }: LogoProps) {
  const className = [
    "font-mono tracking-tight",
    size === "sm" ? "text-sm" : "text-base",
    tone === "muted" ? "text-zinc-400" : "text-zinc-100",
  ].join(" ");
  return <span className={className}>sched</span>;
}
