import { createFileRoute } from "@tanstack/react-router";
import Content from "../../../content/docs/persistence.mdx";

export const Route = createFileRoute("/docs/architecture/persistence")({
  component: () => <Content />,
});
