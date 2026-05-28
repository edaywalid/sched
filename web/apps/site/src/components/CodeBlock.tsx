import { useMemo, useState } from "react";

const GO_KEYWORDS = new Set([
  "func",
  "return",
  "for",
  "range",
  "if",
  "else",
  "var",
  "import",
  "package",
  "interface",
  "struct",
  "map",
  "chan",
  "go",
  "defer",
  "type",
  "const",
  "any",
]);

interface Token {
  text: string;
  kind: "kw" | "str" | "cmt" | "fn" | "num" | "pl";
}

function tokenizeGo(source: string): Token[] {
  const tokens: Token[] = [];
  let i = 0;
  while (i < source.length) {
    const ch = source[i]!;

    if (ch === "/" && source[i + 1] === "/") {
      const end = source.indexOf("\n", i);
      const stop = end === -1 ? source.length : end;
      tokens.push({ text: source.slice(i, stop), kind: "cmt" });
      i = stop;
      continue;
    }

    if (ch === '"') {
      let j = i + 1;
      while (j < source.length && source[j] !== '"') {
        if (source[j] === "\\") j += 2;
        else j += 1;
      }
      j = Math.min(j + 1, source.length);
      tokens.push({ text: source.slice(i, j), kind: "str" });
      i = j;
      continue;
    }

    if (/[A-Za-z_]/.test(ch)) {
      let j = i + 1;
      while (j < source.length && /[A-Za-z0-9_]/.test(source[j]!)) j += 1;
      const word = source.slice(i, j);
      if (GO_KEYWORDS.has(word)) tokens.push({ text: word, kind: "kw" });
      else if (source[j] === "(") tokens.push({ text: word, kind: "fn" });
      else tokens.push({ text: word, kind: "pl" });
      i = j;
      continue;
    }

    if (/[0-9]/.test(ch)) {
      let j = i + 1;
      while (j < source.length && /[0-9._]/.test(source[j]!)) j += 1;
      tokens.push({ text: source.slice(i, j), kind: "num" });
      i = j;
      continue;
    }

    tokens.push({ text: ch, kind: "pl" });
    i += 1;
  }
  return tokens;
}

const KIND_CLASS: Record<Token["kind"], string> = {
  kw: "text-violet-300",
  str: "text-emerald-300",
  cmt: "text-zinc-500 italic",
  fn: "text-sky-300",
  num: "text-amber-300",
  pl: "text-zinc-300",
};

export function CodeBlock({
  code,
  language = "go",
  bare = false,
}: {
  code: string;
  language?: "go";
  bare?: boolean;
}) {
  const tokens = useMemo(
    () => (language === "go" ? tokenizeGo(code) : []),
    [code, language],
  );
  const [copied, setCopied] = useState(false);

  async function onCopy() {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // best-effort
    }
  }

  const body = (
    <pre className="overflow-auto px-4 py-4 font-mono text-[13px] leading-relaxed">
      <code>
        {tokens.map((t, i) => (
          <span key={i} className={KIND_CLASS[t.kind]}>
            {t.text}
          </span>
        ))}
      </code>
    </pre>
  );

  if (bare) {
    return (
      <div className="relative">
        <button
          type="button"
          onClick={onCopy}
          className="absolute right-3 top-2 z-10 rounded px-2 py-0.5 text-xs text-zinc-500 hover:bg-zinc-900 hover:text-zinc-200"
        >
          {copied ? "Copied" : "Copy"}
        </button>
        {body}
      </div>
    );
  }

  return (
    <div className="overflow-hidden rounded-lg border border-zinc-900 bg-zinc-950">
      <div className="flex items-center justify-between border-b border-zinc-900 px-4 py-2 text-xs text-zinc-500">
        <span className="font-mono">{language}</span>
        <button
          type="button"
          onClick={onCopy}
          className="rounded px-2 py-0.5 hover:bg-zinc-900 hover:text-zinc-200"
        >
          {copied ? "Copied" : "Copy"}
        </button>
      </div>
      {body}
    </div>
  );
}
