import { createFileRoute } from "@tanstack/react-router";
import Content from "../../../content/docs/signals.mdx";

export const Route = createFileRoute("/docs/concepts/signals")({
  component: () => <Content />,
});
