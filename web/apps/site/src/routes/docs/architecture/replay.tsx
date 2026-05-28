import { createFileRoute } from "@tanstack/react-router";
import Content from "../../../content/docs/replay.mdx";

export const Route = createFileRoute("/docs/architecture/replay")({
  component: () => <Content />,
});
