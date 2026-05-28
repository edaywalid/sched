import { createFileRoute, Outlet } from "@tanstack/react-router";
import { MDXProvider } from "@mdx-js/react";
import { DocsShell } from "../../components/docs/DocsShell";
import { mdxComponents } from "../../components/docs/MDXComponents";

export const Route = createFileRoute("/docs")({
  component: DocsLayout,
});

function DocsLayout() {
  return (
    <MDXProvider components={mdxComponents}>
      <DocsShell>
        <Outlet />
      </DocsShell>
    </MDXProvider>
  );
}
