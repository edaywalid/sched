import { createFileRoute } from "@tanstack/react-router";
import Content from "../../../content/docs/activities.mdx";

export const Route = createFileRoute("/docs/concepts/activities")({
  component: () => <Content />,
});
