import { createFileRoute } from "@tanstack/react-router";
import Content from "../../../content/docs/workflows.mdx";

export const Route = createFileRoute("/docs/concepts/workflows")({
  component: () => <Content />,
});
