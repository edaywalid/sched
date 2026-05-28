import { defineConfig } from "vite";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import mdx from "@mdx-js/rollup";
import remarkGfm from "remark-gfm";
import rehypeShiki from "@shikijs/rehype";
import { tanstackStart } from "@tanstack/react-start/plugin/vite";

export default defineConfig({
  plugins: [
    tailwindcss(),
    {
      enforce: "pre",
      ...mdx({
        remarkPlugins: [remarkGfm],
        rehypePlugins: [
          [
            rehypeShiki,
            {
              theme: "github-dark-dimmed",
              defaultLanguage: "text",
            },
          ],
        ],
        providerImportSource: "@mdx-js/react",
      }),
    },
    tanstackStart({
      tsr: {
        routesDirectory: "src/routes",
        generatedRouteTree: "src/routeTree.gen.ts",
      },
    }),
    react({ include: /\.(jsx|tsx|mdx)$/ }),
  ],
});
