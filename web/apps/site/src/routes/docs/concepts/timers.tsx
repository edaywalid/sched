import { createFileRoute } from "@tanstack/react-router";
import Content from "../../../content/docs/timers.mdx";

export const Route = createFileRoute("/docs/concepts/timers")({
  component: () => <Content />,
});
