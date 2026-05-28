export function ArchitectureDiagram() {
  return (
    <div className="overflow-hidden rounded-xl border border-zinc-800 bg-zinc-950 p-8">
      <svg
        viewBox="0 0 720 360"
        className="h-auto w-full"
        role="img"
        aria-label="sched architecture: SDK workers talk to the engine over gRPC; the engine persists state to Postgres and dispatches tasks via Redis Streams."
      >
        <defs>
          <marker
            id="arrow"
            viewBox="0 0 10 10"
            refX="8"
            refY="5"
            markerWidth="6"
            markerHeight="6"
            orient="auto-start-reverse"
          >
            <path d="M 0 0 L 10 5 L 0 10 z" fill="oklch(60% 0.16 145)" />
          </marker>
          <marker
            id="arrow-muted"
            viewBox="0 0 10 10"
            refX="8"
            refY="5"
            markerWidth="6"
            markerHeight="6"
            orient="auto-start-reverse"
          >
            <path d="M 0 0 L 10 5 L 0 10 z" fill="oklch(50% 0 0)" />
          </marker>
        </defs>

        <Group x={36} y={50} w={140} h={56} label="Worker" sub="SDK · Go" />
        <Group x={36} y={150} w={140} h={56} label="Worker" sub="SDK · Go" />
        <Group x={36} y={250} w={140} h={56} label="Worker" sub="SDK · Go" />

        <Group
          x={290}
          y={140}
          w={170}
          h={80}
          label="Engine"
          sub="gRPC · streaming"
          accent
        />

        <Group x={560} y={50} w={140} h={56} label="Postgres" sub="state + history" />
        <Group x={560} y={150} w={140} h={56} label="Redis" sub="task streams" />
        <Group x={560} y={250} w={140} h={56} label="Prometheus" sub="metrics + OTel" />

        {/* Worker → Engine */}
        <Arrow d="M 180 78 C 230 78, 240 165, 286 165" />
        <Arrow d="M 180 178 L 286 178" />
        <Arrow d="M 180 278 C 230 278, 240 195, 286 195" />

        {/* Engine → Postgres / Redis / Prom */}
        <Arrow d="M 462 165 C 510 165, 520 78, 558 78" />
        <Arrow d="M 462 180 L 558 180" />
        <Arrow d="M 462 195 C 510 195, 520 278, 558 278" muted />

        <text
          x={232}
          y={138}
          textAnchor="middle"
          className="fill-zinc-500 font-mono"
          style={{ fontSize: 10 }}
        >
          poll · stream
        </text>
        <text
          x={510}
          y={138}
          textAnchor="middle"
          className="fill-zinc-500 font-mono"
          style={{ fontSize: 10 }}
        >
          persist · dispatch
        </text>
      </svg>
    </div>
  );
}

function Group({
  x,
  y,
  w,
  h,
  label,
  sub,
  accent,
}: {
  x: number;
  y: number;
  w: number;
  h: number;
  label: string;
  sub: string;
  accent?: boolean;
}) {
  const stroke = accent ? "oklch(68% 0.16 145)" : "oklch(30% 0 0)";
  const fill = accent ? "oklch(20% 0.04 145)" : "oklch(15% 0 0)";
  return (
    <g>
      <rect
        x={x}
        y={y}
        width={w}
        height={h}
        rx={10}
        ry={10}
        fill={fill}
        stroke={stroke}
        strokeWidth={accent ? 1.5 : 1}
      />
      <text
        x={x + w / 2}
        y={y + 26}
        textAnchor="middle"
        className="fill-zinc-100"
        style={{ fontSize: 14, fontWeight: 500 }}
      >
        {label}
      </text>
      <text
        x={x + w / 2}
        y={y + 44}
        textAnchor="middle"
        className="fill-zinc-500 font-mono"
        style={{ fontSize: 10 }}
      >
        {sub}
      </text>
    </g>
  );
}

function Arrow({ d, muted = false }: { d: string; muted?: boolean }) {
  return (
    <path
      d={d}
      fill="none"
      stroke={muted ? "oklch(50% 0 0)" : "oklch(60% 0.16 145)"}
      strokeWidth={1.25}
      strokeOpacity={muted ? 0.5 : 0.7}
      markerEnd={`url(#${muted ? "arrow-muted" : "arrow"})`}
    />
  );
}
