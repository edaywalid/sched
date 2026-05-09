import { useState, type FormEvent } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api/client";

interface State {
  workflowName: string;
  input: string;
  timeoutSeconds: string;
}

const EMPTY: State = { workflowName: "", input: "", timeoutSeconds: "" };

export function StartWorkflowForm() {
  const [state, setState] = useState<State>(EMPTY);
  const qc = useQueryClient();

  const start = useMutation({
    mutationFn: () =>
      api.startWorkflow({
        workflowName: state.workflowName.trim(),
        input: state.input,
        executionTimeoutSeconds: state.timeoutSeconds
          ? Math.max(0, parseInt(state.timeoutSeconds, 10) || 0)
          : 0,
      }),
    onSuccess: () => {
      setState(EMPTY);
      qc.invalidateQueries({ queryKey: ["workflows"] });
      qc.invalidateQueries({ queryKey: ["metrics"] });
    },
  });

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!state.workflowName.trim()) return;
    start.mutate();
  }

  return (
    <form
      onSubmit={onSubmit}
      className="rounded-md border border-zinc-900 bg-zinc-950 p-4"
    >
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-medium text-zinc-100">Start a workflow</h2>
        {start.error ? (
          <span className="text-xs text-rose-300">
            {(start.error as Error).message}
          </span>
        ) : start.isSuccess ? (
          <span className="text-xs text-emerald-300">Started</span>
        ) : null}
      </div>
      <div className="mt-3 grid gap-3 sm:grid-cols-[2fr_2fr_1fr_auto]">
        <Field
          label="Name"
          placeholder="HelloWorld"
          value={state.workflowName}
          onChange={(v) => setState((s) => ({ ...s, workflowName: v }))}
          autoFocus
          required
        />
        <Field
          label="Input"
          placeholder='"jane" or {"k": "v"}'
          value={state.input}
          onChange={(v) => setState((s) => ({ ...s, input: v }))}
        />
        <Field
          label="Timeout (s)"
          placeholder="0"
          inputMode="numeric"
          value={state.timeoutSeconds}
          onChange={(v) => setState((s) => ({ ...s, timeoutSeconds: v }))}
        />
        <div className="flex items-end">
          <button
            type="submit"
            disabled={start.isPending || !state.workflowName.trim()}
            className="h-9 rounded-md bg-zinc-100 px-4 text-sm font-medium text-zinc-950 hover:bg-white disabled:cursor-not-allowed disabled:opacity-50"
          >
            {start.isPending ? "Starting…" : "Start"}
          </button>
        </div>
      </div>
    </form>
  );
}

interface FieldProps {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  required?: boolean;
  autoFocus?: boolean;
  inputMode?: "numeric" | "text";
}

function Field({
  label,
  value,
  onChange,
  placeholder,
  required,
  autoFocus,
  inputMode,
}: FieldProps) {
  return (
    <label className="block">
      <span className="block text-xs uppercase tracking-wider text-zinc-500">
        {label}
      </span>
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        required={required}
        autoFocus={autoFocus}
        inputMode={inputMode}
        className="mt-1 h-9 w-full rounded-md border border-zinc-800 bg-zinc-950 px-2.5 text-sm text-zinc-100 placeholder:text-zinc-600 focus:border-accent-600 focus:outline-none"
      />
    </label>
  );
}
