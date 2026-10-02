import { spawn } from "node:child_process";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { detectDesktopEnvironment } from "./desktop-env.mjs";
import { developmentMessages, messages } from "./dev-messages.mjs";
import { checkUbuntuSandbox } from "./ubuntu-sandbox.mjs";

export async function waitForRenderer(
  url,
  { signal, timeout = 30_000, text = messages["en-US"] } = {},
) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    signal?.throwIfAborted();
    try {
      const response = await fetch(url, {
        signal: AbortSignal.any([
          AbortSignal.timeout(1000),
          ...(signal ? [signal] : []),
        ]),
      });
      if (response.ok) return;
    } catch {
      signal?.throwIfAborted();
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error(text.renderer(url));
}

async function start() {
  const root = path.resolve(import.meta.dirname, "..");
  const controller = new AbortController();
  let child;
  for (const signal of ["SIGINT", "SIGTERM"]) {
    process.once(signal, () => {
      controller.abort();
      child?.kill(signal);
    });
  }
  try {
    const text = await developmentMessages();
    const require = createRequire(import.meta.url);
    const binary = require("electron");
    await checkUbuntuSandbox({ binary, text });
    const env = await detectDesktopEnvironment({ text });
    controller.signal.throwIfAborted();
    await new Promise((resolve, reject) => {
      child = spawn(
        process.platform === "win32" ? "pnpm.cmd" : "pnpm",
        ["run", "build"],
        {
          cwd: root,
          stdio: "inherit",
          shell: process.platform === "win32",
        },
      );
      child.once("error", reject);
      child.once("exit", (code) =>
        code === 0 ? resolve() : reject(new Error(text.build(code))),
      );
    });
    const url = `http://localhost:${process.env.ELECTRON_RENDERER_PORT || 5173}/`;
    await waitForRenderer(url, { signal: controller.signal, text });
    child = spawn(
      binary,
      [path.join(root, "build/main/index.js"), ...process.argv.slice(2)],
      {
        cwd: root,
        stdio: "inherit",
        env: { ...env, NODE_ENV: "development" },
      },
    );
    child.once("error", (error) => {
      console.error(error.message);
      process.exitCode = 1;
    });
    child.once("exit", (code) => {
      process.exitCode = code ?? (controller.signal.aborted ? 0 : 1);
    });
  } catch (error) {
    if (!controller.signal.aborted) {
      console.error(error.message);
      process.exitCode = 1;
    }
  }
}

if (
  process.argv[1] &&
  path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
)
  await start();
