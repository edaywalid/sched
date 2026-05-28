import { useRef, useState, type CSSProperties, type ReactNode } from "react";
import { Check, Copy } from "lucide-react";

const LANG_LABEL: Record<string, string> = {
  go: "Go",
  python: "Python",
  py: "Python",
  ts: "TypeScript",
  typescript: "TypeScript",
  js: "JavaScript",
  javascript: "JavaScript",
  bash: "Shell",
  sh: "Shell",
  shell: "Shell",
  yaml: "YAML",
  yml: "YAML",
  json: "JSON",
  sql: "SQL",
  promql: "PromQL",
  text: "Text",
};

export function CodeViewer({
  children,
  language,
  className,
  style,
}: {
  children: ReactNode;
  language?: string;
  className?: string;
  style?: CSSProperties;
}) {
  const ref = useRef<HTMLPreElement>(null);
  const [copied, setCopied] = useState(false);

  const onCopy = async () => {
    const text = ref.current?.innerText ?? "";
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      // Clipboard API blocked (insecure context). Fall back to a manual select.
      const sel = window.getSelection();
      const range = document.createRange();
      if (ref.current && sel) {
        range.selectNodeContents(ref.current);
        sel.removeAllRanges();
        sel.addRange(range);
      }
    }
  };

  const label = language ? LANG_LABEL[language.toLowerCase()] ?? language : "Text";

  return (
    <div
      className="my-5 overflow-hidden rounded-lg border border-zinc-800/80"
      style={{ backgroundColor: style?.backgroundColor }}
    >
      <div className="flex items-center justify-between border-b border-zinc-800/80 bg-black/25 px-3 py-1.5">
        <span className="font-mono text-[10.5px] font-medium uppercase tracking-wider text-zinc-400">
          {label}
        </span>
        <button
          type="button"
          onClick={onCopy}
          aria-label={copied ? "Code copied" : "Copy code"}
          className="flex items-center gap-1.5 rounded px-1.5 py-0.5 text-[11px] text-zinc-400 transition-colors hover:bg-zinc-800/80 hover:text-zinc-100"
        >
          {copied ? <Check className="size-3" /> : <Copy className="size-3" />}
          <span>{copied ? "Copied" : "Copy"}</span>
        </button>
      </div>
      <pre
        ref={ref}
        className={`overflow-x-auto px-4 py-3 font-mono text-[13.5px] leading-6 ${className ?? ""}`}
        style={{ ...style, backgroundColor: "transparent" }}
      >
        {children}
      </pre>
    </div>
  );
}
