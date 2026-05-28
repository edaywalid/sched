import { createFileRoute } from "@tanstack/react-router";
import Content from "../../../content/docs/ha.mdx";

export const Route = createFileRoute("/docs/operating/ha")({
  component: () => <Content />,
});
