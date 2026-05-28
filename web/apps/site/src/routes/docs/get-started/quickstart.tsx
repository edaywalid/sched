import { createFileRoute } from "@tanstack/react-router";
import Content from "../../../content/docs/quickstart.mdx";

export const Route = createFileRoute("/docs/get-started/quickstart")({
  component: () => <Content />,
});
