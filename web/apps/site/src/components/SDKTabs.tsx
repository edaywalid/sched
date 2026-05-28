import { useState } from "react";
import { motion } from "motion/react";
import { CodeBlock } from "./CodeBlock";

const TABS = [
  {
    id: "workflow",
    label: "Workflow",
    code: `client.RegisterWorkflow("MonthlyReport", func(ctx sdk.WorkflowContext, input any) (any, error) {
    for i := range 3 {
        ctx.QueueActivity("SendEmail", fmt.Sprintf("user%d@example.com", i))
        ctx.Sleep(2 * time.Second)
    }
    return "report complete", nil
})`,
  },
  {
    id: "activity",
    label: "Activity",
    code: `client.RegisterActivity("SendEmail", func(ctx sdk.ActivityContext, input any) (any, error) {
    addr := input.(string)
    canceled, err := ctx.Heartbeat(nil)
    if err != nil || canceled {
        return nil, err
    }
    return smtp.Send(addr, body)
})`,
  },
  {
    id: "signal",
    label: "Signals",
    code: `client.RegisterWorkflow("ApprovalGate", func(ctx sdk.WorkflowContext, input any) (any, error) {
    name, payload, err := ctx.WaitForSignal(24 * time.Hour)
    if err != nil {
        return nil, err
    }
    if name != "approve" {
        return "rejected", nil
    }
    ctx.QueueActivity("Process", payload)
    return "approved", nil
})`,
  },
  {
    id: "client",
    label: "Client",
    code: `client, _ := sdk.NewClient("localhost:50051", "default")
defer client.Close()

wfID, err := client.StartWorkflow(ctx, "ApprovalGate", map[string]any{
    "amount": 250,
})

// Later, from anywhere with the workflow ID:
client.SignalWorkflow(ctx, wfID, "approve", nil)`,
  },
] as const;

export function SDKTabs() {
  const [active, setActive] = useState<(typeof TABS)[number]["id"]>("workflow");
  const current = TABS.find((t) => t.id === active) ?? TABS[0];

  return (
    <div className="overflow-hidden rounded-xl border border-zinc-800 bg-zinc-950">
      <div className="flex items-center gap-0.5 border-b border-zinc-900 bg-zinc-950/60 p-1">
        {TABS.map((tab) => (
          <button
            key={tab.id}
            type="button"
            onClick={() => setActive(tab.id)}
            className="relative rounded-md px-3 py-1.5 text-xs text-zinc-400 transition-colors hover:text-zinc-100"
          >
            {tab.id === active ? (
              <motion.span
                layoutId="sdk-tab-bg"
                className="absolute inset-0 -z-10 rounded-md bg-zinc-900"
                transition={{ type: "spring", duration: 0.3, bounce: 0.2 }}
              />
            ) : null}
            <span className={tab.id === active ? "text-zinc-100" : ""}>
              {tab.label}
            </span>
          </button>
        ))}
      </div>
      <div className="border-0">
        <CodeBlock code={current.code} bare />
      </div>
    </div>
  );
}
