import { readdirSync, rmSync } from "node:fs";
import { resolve } from "node:path";
import { sha256File, writeJson } from "../lib/files.mts";
import { retry } from "../lib/retry.mts";
import { shell } from "../lib/shell.mts";
import { fail } from "../lib/output.mts";
import { MissingMacAppTicketError, notarizeMacDMG, verifyMacApp } from "./macos/signing.mts";
import type { DesktopConfig } from "./config.mts";
import type { Sidecars } from "./sidecars.mts";

export function writeBundleConfig(config: DesktopConfig, sidecars: Sidecars, identity?: string) {
  const manifest = {
    version: config.version,
    commit: config.commit,
    channel: config.channel,
    protocol: 2,
    api: 1,
    target: config.target,
    sha256: sidecars.hashes["one-api"],
    cliSha256: sidecars.hashes.one,
  };
  writeJson(resolve(config.native, "bundle-manifest.json"), manifest);
  const bundle: Record<string, unknown> = {
    createUpdaterArtifacts: config.release && config.updater,
  };
  if (identity) {
    // Tauri must retain the signatures used for the manifest's hashes.
    bundle.externalBin = [];
    bundle.macOS = {
      files: sidecars.signedFiles,
      signingIdentity: identity,
      hardenedRuntime: true,
    };
  }
  const buildConfig: Record<string, unknown> = { version: config.version, bundle };
  if (config.release && config.updater)
    buildConfig.plugins = { updater: { pubkey: config.env.ONE_DESKTOP_UPDATER_PUBLIC_KEY } };
  writeJson(resolve(config.native, "build-config.json"), buildConfig);
  return manifest;
}

function tauriArguments(config: DesktopConfig) {
  const args = ["exec", "tauri", config.command, "--config", "src-tauri/build-config.json"];
  if (config.command === "build") {
    const bundles = { darwin: "app,dmg", windows: "nsis", linux: "appimage,deb" };
    args.push("--target", config.target, "--bundles", bundles[config.platform.os]);
    if (config.release) args.push("--ci");
  }
  args.push(...config.extraArgs);
  if (config.release && config.command === "build") args.push("--", "--locked");
  return args;
}

function bundleEnvironment(config: DesktopConfig, identity?: string) {
  const env = { ...config.env };
  if (identity) {
    delete env.APPLE_CERTIFICATE_BASE64;
    delete env.APPLE_CERTIFICATE_PASSWORD;
    delete env.APPLE_CERTIFICATE;
    env.APPLE_SIGNING_IDENTITY = identity;
  }
  return env;
}

function verifyMacBundle(config: DesktopConfig, sidecars: Sidecars) {
  const app = resolve(config.bundleDirectory, "macos/One Desktop.app");
  for (const [name, digest] of Object.entries(sidecars.hashes)) {
    if (sha256File(resolve(app, "Contents/MacOS", name)) !== digest)
      fail(`Bundled binary hash changed: ${name}`, `包内二进制摘要变化：${name}`);
  }
  if (!config.release) return;
  verifyMacApp(app);
  const dmgs = readdirSync(resolve(config.bundleDirectory, "dmg")).filter((name) =>
    name.endsWith(".dmg"),
  );
  if (dmgs.length !== 1) fail("Expected one DMG", "需要唯一的 DMG");
  notarizeMacDMG(resolve(config.bundleDirectory, "dmg", dmgs[0]));
}

export function buildBundle(config: DesktopConfig, sidecars: Sidecars, identity?: string) {
  const run = shell({
    cwd: config.desktop,
    env: bundleEnvironment(config, identity),
    stdio: "inherit",
  });
  const args = tauriArguments(config);
  retry(
    () => {
      if (config.release && config.command === "build")
        rmSync(config.bundleDirectory, { recursive: true, force: true });
      run`pnpm ${args}`;
      if (config.command === "build" && config.platform.os === "darwin")
        verifyMacBundle(config, sidecars);
    },
    {
      delays: [5000, 15000],
      shouldRetry: (error) => error instanceof MissingMacAppTicketError,
      onRetry: (_error, delay, attempt) =>
        console.warn(
          `App ticket missing; rebuilding app and DMG in ${delay / 1000}s (${attempt}/2) / 应用缺少票据，${delay / 1000} 秒后重新生成应用和 DMG`,
        ),
    },
  );
}
