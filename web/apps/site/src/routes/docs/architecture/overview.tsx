import { createFileRoute } from "@tanstack/react-router";
import Content from "../../../content/docs/architecture.mdx";

export const Route = createFileRoute("/docs/architecture/overview")({
  component: () => <Content />,
});
