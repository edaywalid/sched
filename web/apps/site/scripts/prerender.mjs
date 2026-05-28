import { mkdir, writeFile } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, "..");
const clientDir = join(root, "dist", "client");

const ROUTES = [
  "/",
  "/docs",
  "/docs/get-started/quickstart",
  "/docs/concepts/workflows",
  "/docs/architecture/overview",
];

const { default: server } = await import(join(root, "dist", "server", "server.js"));

let written = 0;
for (const path of ROUTES) {
  const url = new URL(`http://localhost${path}`);
  const res = await server.fetch(new Request(url));
  if (!res.ok) {
    console.error(`FAIL ${path}: ${res.status}`);
    process.exitCode = 1;
    continue;
  }
  const html = await res.text();
  const outFile =
    path === "/" ? join(clientDir, "index.html") : join(clientDir, path.replace(/^\//, ""), "index.html");
  await mkdir(dirname(outFile), { recursive: true });
  await writeFile(outFile, html, "utf8");
  written++;
  console.log(`  ${path}  ->  ${outFile.replace(root, ".")}`);
}

console.log(`\nprerendered ${written}/${ROUTES.length} routes`);
