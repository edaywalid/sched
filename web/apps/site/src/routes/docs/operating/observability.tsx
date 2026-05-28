import { createFileRoute } from "@tanstack/react-router";
import Content from "../../../content/docs/observability.mdx";

export const Route = createFileRoute("/docs/operating/observability")({
  component: () => <Content />,
});
