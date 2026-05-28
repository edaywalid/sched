import { createFileRoute } from "@tanstack/react-router";
import Content from "../../../content/docs/install.mdx";

export const Route = createFileRoute("/docs/get-started/install")({
  component: () => <Content />,
});
