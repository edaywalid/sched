import type { ComponentPropsWithoutRef, ReactNode } from "react";
import { ArchitectureDiagram } from "../ArchitectureDiagram";
import { CodeViewer } from "./CodeViewer";

export const mdxComponents = {
  h1: (props: ComponentPropsWithoutRef<"h1">) => (
    <h1
      className="mt-2 mb-5 scroll-mt-24 text-[28px] font-semibold tracking-tight text-zinc-50"
      {...props}
    />
  ),
  h2: (props: ComponentPropsWithoutRef<"h2">) => (
    <h2
      className="mt-10 mb-3 scroll-mt-24 text-[19px] font-semibold tracking-tight text-zinc-100"
      {...props}
    />
  ),
  h3: (props: ComponentPropsWithoutRef<"h3">) => (
    <h3
      className="mt-7 mb-2 scroll-mt-24 text-[15px] font-semibold text-zinc-100"
      {...props}
    />
  ),
  p: (props: ComponentPropsWithoutRef<"p">) => (
    <p className="my-4 text-[15px] leading-7 text-zinc-300" {...props} />
  ),
  a: (props: ComponentPropsWithoutRef<"a">) => (
    <a className="text-accent-400 hover:underline" {...props} />
  ),
  ul: (props: ComponentPropsWithoutRef<"ul">) => (
    <ul className="my-4 ml-5 list-disc space-y-1 text-[15px] text-zinc-300 marker:text-zinc-600" {...props} />
  ),
  ol: (props: ComponentPropsWithoutRef<"ol">) => (
    <ol className="my-4 ml-5 list-decimal space-y-1 text-[15px] text-zinc-300 marker:text-zinc-600" {...props} />
  ),
  li: (props: ComponentPropsWithoutRef<"li">) => (
    <li className="leading-7" {...props} />
  ),
  blockquote: (props: ComponentPropsWithoutRef<"blockquote">) => (
    <blockquote
      className="my-6 border-l-2 border-accent-500/60 pl-4 italic text-zinc-400"
      {...props}
    />
  ),
  hr: () => <hr className="my-10 border-zinc-900" />,
  code: ({ children, className, ...rest }: ComponentPropsWithoutRef<"code">) => {
    const isBlock = typeof children !== "string";
    if (isBlock) {
      return (
        <code className={className} {...rest}>
          {children}
        </code>
      );
    }
    return (
      <code
        className="rounded border border-zinc-900 bg-zinc-900/60 px-1.5 py-0.5 font-mono text-[0.85em] text-accent-300"
        {...rest}
      >
        {children}
      </code>
    );
  },
  pre: ({ children, className, style }: ComponentPropsWithoutRef<"pre">) => {
    const isShiki = className?.includes("shiki");
    const lang = className?.match(/language-([a-z0-9_+-]+)/i)?.[1];
    if (isShiki) {
      return (
        <CodeViewer language={lang} className={className} style={style}>
          {children}
        </CodeViewer>
      );
    }
    return (
      <CodeViewer
        language="text"
        style={{ backgroundColor: "rgb(34 39 46)" }}
      >
        <code>{children}</code>
      </CodeViewer>
    );
  },
  table: (props: ComponentPropsWithoutRef<"table">) => (
    <div className="my-6 overflow-x-auto rounded-lg border border-zinc-900">
      <table className="w-full border-collapse text-sm" {...props} />
    </div>
  ),
  th: (props: ComponentPropsWithoutRef<"th">) => (
    <th
      className="border-b border-zinc-900 bg-zinc-950 px-4 py-2 text-left font-medium text-zinc-200"
      {...props}
    />
  ),
  td: (props: ComponentPropsWithoutRef<"td">) => (
    <td className="border-b border-zinc-900 px-4 py-2 align-top text-zinc-300" {...props} />
  ),
  ArchitectureDiagram: () => (
    <div className="my-8">
      <ArchitectureDiagram />
    </div>
  ),
  Callout: ({
    children,
    tone = "info",
  }: {
    children: ReactNode;
    tone?: "info" | "warn";
  }) => {
    const toneClass =
      tone === "warn"
        ? "border-amber-500/30 bg-amber-500/5 text-amber-200"
        : "border-accent-500/30 bg-accent-500/5 text-zinc-200";
    return (
      <div className={`my-6 rounded-lg border px-4 py-3 text-sm ${toneClass}`}>
        {children}
      </div>
    );
  },
};
