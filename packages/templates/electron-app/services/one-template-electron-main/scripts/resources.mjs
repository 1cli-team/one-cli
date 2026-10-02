import { access, cp, mkdir, rm } from "node:fs/promises";
import path from "node:path";

// Every component builds in its own directory. Packaging assembles copies in main.
export async function prepareResources({ mainDir, rendererDir, preloadEntry }) {
  await Promise.all([
    access(path.join(mainDir, "build/main/index.js")),
    access(path.join(rendererDir, "index.html")),
    access(preloadEntry),
  ]);
  const rendererTarget = path.join(mainDir, "build/renderer");
  const preloadTarget = path.join(mainDir, "build/preload");
  await rm(rendererTarget, { recursive: true, force: true });
  await cp(rendererDir, rendererTarget, { recursive: true });
  await mkdir(preloadTarget, { recursive: true });
  await cp(preloadEntry, path.join(preloadTarget, "index.cjs"));
}
