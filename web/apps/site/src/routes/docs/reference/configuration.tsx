import { createFileRoute } from "@tanstack/react-router";
import Content from "../../../content/docs/configuration.mdx";

export const Route = createFileRoute("/docs/reference/configuration")({
  component: () => <Content />,
});
