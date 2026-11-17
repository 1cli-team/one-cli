import { spawn, type ChildProcess } from "node:child_process";
import { createRequire } from "node:module";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import { prepareDesktopGateway } from "../gateway.mts";
import type { DesktopConfig } from "./config.mts";

export async function startDesktop(config: DesktopConfig) {
  const require = createRequire(join(config.native, "package.json"));
  const binary = require("electron") as string;
  const scripts = join(config.native, "scripts");
  const { developmentMessages } = await import(
    pathToFileURL(join(scripts, "dev-messages.mjs")).href
  );
  const { checkUbuntuSandbox } = await import(
    pathToFileURL(join(scripts, "ubuntu-sandbox.mjs")).href
  );
  const { detectDesktopEnvironment } = await import(
    pathToFileURL(join(scripts, "desktop-env.mjs")).href
  );
  const text = await developmentMessages();
  await checkUbuntuSandbox({ binary, text });
  const env = await detectDesktopEnvironment({ text });
  const children = new Set<ChildProcess>();
  let closing = false;
  let serviceError: Error | undefined;
  const launch = (file: string, args: string[], cwd: string, extra = {}) => {
    const child = spawn(file, args, {
      cwd,
      env: { ...env, ...extra },
      stdio: "inherit",
      windowsHide: true,
    });
    children.add(child);
    child.once("close", () => children.delete(child));
    return child;
  };
  const close = () => {
    closing = true;
    for (const child of children) child.kill("SIGTERM");
  };
  const stopWithError = (error: Error) => {
    if (closing) return;
    serviceError = error;
    close();
  };
  process.once("SIGINT", close);
  process.once("SIGTERM", close);
  try {
    const gateway = config.env.ONE_DESKTOP_DEV_GATEWAY === "1"
      ? await prepareDesktopGateway()
      : undefined;
    if (closing) return;
    if (gateway) {
      const child = launch(gateway.binary, gateway.args, config.root, gateway.env);
      const gatewayStopped = (reason: string | number) => stopWithError(
        new Error(`Local Gateway stopped (${reason}).\n本机 Gateway 已停止（${reason}）。`),
      );
      child.once("error", (error) => gatewayStopped(error.message));
      child.once("exit", (code, signal) => gatewayStopped(code ?? signal ?? ""));
    }
    const rendererURL = "http://127.0.0.1:5173";
    const vite = launch(
      process.execPath,
      [join(config.desktop, "node_modules/vite/bin/vite.js")],
      config.desktop,
    );
    const rendererStopped = () => stopWithError(new Error(text.renderer(rendererURL)));
    vite.once("error", rendererStopped);
    vite.once("exit", rendererStopped);
    let ready = false;
    for (let attempt = 0; attempt < 150 && !closing; attempt++) {
      try {
        ready = (await fetch(rendererURL, { signal: AbortSignal.timeout(500) })).ok;
      } catch {
        /* Vite is still starting. */
      }
      if (ready) break;
      await new Promise((done) => setTimeout(done, 200));
    }
    if (serviceError) throw serviceError;
    if (closing) return;
    if (!ready) throw new Error(text.renderer(rendererURL));
    const host = launch(binary, [config.native], config.native, { ONE_DESKTOP_DEV: "1" });
    await new Promise<void>((resolve, reject) => {
      host.once("error", reject);
      host.once("exit", (code) => {
        if (code && !closing) reject(new Error(text.build(code)));
        else resolve();
      });
    });
    if (serviceError) throw serviceError;
  } finally {
    close();
    process.removeListener("SIGINT", close);
    process.removeListener("SIGTERM", close);
  }
}
