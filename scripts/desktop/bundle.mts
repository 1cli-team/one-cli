import { cpSync, mkdirSync, readFileSync, readdirSync, rmSync } from "node:fs";
import { resolve } from "node:path";
import { createRequire } from "node:module";
import { sha256File, writeJson } from "../lib/files.mts";
import { shell } from "../lib/shell.mts";
import { fail } from "../lib/output.mts";
import { notarizeMacApp, notarizeMacDMG, verifyMacApp } from "./macos/signing.mts";
import type { DesktopConfig } from "./config.mts";
import type { Sidecars } from "./sidecars.mts";

const productName = "One Desktop";

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
  writeJson(resolve(config.native, "resources/bundle-manifest.json"), manifest);
  const require = createRequire(resolve(config.native, "package.json"));
  const buildConfig = {
    electronDist:
      config.platform.os ===
        ({ darwin: "darwin", linux: "linux", win32: "windows" } as Record<string, string>)[
          process.platform
        ] && config.platform.arch === (process.arch === "x64" ? "amd64" : process.arch)
        ? resolve(require.resolve("electron/package.json"), "../dist")
        : undefined,
    appId: "team.1cli.desktop",
    productName,
    executableName: "one-desktop",
    artifactName: `one-desktop_${config.version}_${config.platform.os}_${config.platform.arch}.\${ext}`,
    extraMetadata: { version: config.version },
    directories: { output: config.bundleDirectory, buildResources: "assets" },
    files: ["build/**/*", "assets/**/*", "package.json"],
    extraResources: [{ from: "resources", to: "." }],
    asar: true,
    npmRebuild: false,
    publish: null,
    mac: {
      executableName: productName,
      category: "public.app-category.developer-tools",
      minimumSystemVersion: "13.3",
      icon: "assets/icon.icns",
      identity: identity?.replace(/^Developer ID Application: /, "") ?? null,
      hardenedRuntime: Boolean(identity),
      notarize: false,
      signIgnore: ["Contents/Resources/bin/"],
      extraFiles: [{ from: "assets/infoplist", to: "Resources" }],
    },
    dmg: { sign: Boolean(identity) },
    win: { icon: "assets/icon.ico", signAndEditExecutable: true },
    nsis: {
      guid: "One Desktop",
      include: "assets/installer.nsh",
      oneClick: false,
      perMachine: false,
      allowElevation: false,
      allowToChangeInstallationDirectory: true,
      createDesktopShortcut: true,
      createStartMenuShortcut: true,
      deleteAppDataOnUninstall: false,
    },
    linux: {
      executableName: "one-desktop",
      icon: "assets/icon.png",
      category: "Development",
      syncDesktopName: true,
      maintainer: "One CLI <support@1cli.dev>",
      desktop: { entry: { StartupWMClass: "one_desktop" } },
    },
  };
  writeJson(resolve(config.native, "build-config.json"), buildConfig);
  return manifest;
}

function buildProcesses(config: DesktopConfig) {
  const run = shell({ cwd: config.root, stdio: "inherit", env: config.env });
  run`pnpm --filter @one-desktop/preload build`;
  run`pnpm --filter @one-desktop/renderer build`;
  run`pnpm --filter @one-desktop/main build`;
  cpSync(resolve(config.desktop, "dist"), resolve(config.native, "build/renderer"), {
    recursive: true,
  });
  mkdirSync(resolve(config.native, "build/preload"), { recursive: true });
  cpSync(
    resolve(config.root, "packages/desktop-preload/dist/index.cjs"),
    resolve(config.native, "build/preload/index.cjs"),
  );
}

function verifyBundle(config: DesktopConfig, sidecars: Sidecars, app: string) {
  const resources = resolve(
    app,
    config.platform.os === "darwin" ? "Contents/Resources" : "resources",
  );
  const manifest = JSON.parse(readFileSync(resolve(resources, "bundle-manifest.json"), "utf8"));
  if (
    manifest.version !== config.version ||
    manifest.commit !== config.commit ||
    manifest.target !== config.target
  )
    fail("Packaged manifest differs from the build", "包内清单与构建不一致");
  for (const [name, digest] of Object.entries(sidecars.hashes)) {
    const file = resolve(resources, "bin", name + (config.platform.os === "windows" ? ".exe" : ""));
    if (sha256File(file) !== digest)
      fail(`Bundled binary hash changed: ${name}`, `包内二进制摘要变化：${name}`);
  }
}

export async function buildBundle(config: DesktopConfig, sidecars: Sidecars) {
  buildProcesses(config);
  if (config.command === "dev") return;
  rmSync(config.bundleDirectory, { recursive: true, force: true });
  // Signing credentials remain in the controlled keychain/notary helpers.
  const env = { ...config.env, CSC_IDENTITY_AUTO_DISCOVERY: "false" };
  for (const key of [
    "APPLE_CERTIFICATE_BASE64",
    "APPLE_CERTIFICATE_PASSWORD",
    "APPLE_ID",
    "APPLE_PASSWORD",
    "APPLE_APP_SPECIFIC_PASSWORD",
    "CSC_LINK",
    "CSC_KEY_PASSWORD",
  ])
    delete env[key];
  const run = shell({ cwd: config.native, env, stdio: "inherit" });
  const platform = { darwin: "--mac", windows: "--win", linux: "--linux" }[config.platform.os];
  const arch = config.platform.arch === "amd64" ? "--x64" : "--arm64";
  run`pnpm exec electron-builder --config build-config.json ${platform} ${arch} --dir --publish never`;
  const unpacked =
    config.platform.os === "darwin"
      ? resolve(
          config.bundleDirectory,
          config.platform.arch === "arm64" ? "mac-arm64" : "mac",
          `${productName}.app`,
        )
      : resolve(
          config.bundleDirectory,
          config.platform.os === "windows"
            ? "win-unpacked"
            : config.platform.arch === "arm64"
              ? "linux-arm64-unpacked"
              : "linux-unpacked",
        );
  verifyBundle(config, sidecars, unpacked);
  if (config.platform.os === "darwin" && config.release) {
    notarizeMacApp(unpacked);
    verifyMacApp(unpacked);
  }
  const formats = { darwin: ["dmg"], windows: ["nsis"], linux: ["AppImage", "deb"] }[
    config.platform.os
  ];
  run`pnpm exec electron-builder --config build-config.json --prepackaged ${unpacked} ${platform} ${formats} ${arch} --publish never`;
  if (config.platform.os === "darwin" && config.release) {
    const dmgs = readdirSync(config.bundleDirectory).filter((name) => name.endsWith(".dmg"));
    if (dmgs.length !== 1) fail("Expected one DMG", "需要唯一的 DMG");
    notarizeMacDMG(resolve(config.bundleDirectory, dmgs[0]));
  }
}
