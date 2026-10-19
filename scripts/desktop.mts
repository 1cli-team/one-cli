import { spawnSync } from "node:child_process";
import { mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import {
  MissingMacAppTicketError,
  notarizeMacDMG,
  signMacSidecar,
  verifyMacApp,
  withMacSigning,
} from "./desktop-macos.mts";
import { collectInstallers, platforms } from "./desktop-release.mts";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const desktop = resolve(root, "apps/desktop");
const native = resolve(desktop, "src-tauri");
const command = process.argv[2] ?? "prepare";
function run(bin: string, args: string[], cwd = root, env = process.env) {
  const result = spawnSync(bin, args, { cwd, env, stdio: "inherit" });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`${bin} exited with ${result.status} / 执行失败`);
}
function output(bin: string, args: string[]) {
  const result = spawnSync(bin, args, { cwd: root, encoding: "utf8" });
  if (result.error || result.status !== 0)
    throw new Error(result.stderr || `${bin} failed / 执行失败`);
  return result.stdout.trim();
}
function hash(path: string) {
  return createHash("sha256").update(readFileSync(path)).digest("hex");
}

if (!["prepare", "build", "dev"].includes(command))
  throw new Error(`Unknown desktop operation / 未知桌面操作: ${command}`);
const target = process.env.ONE_DESKTOP_TARGET ?? output("rustc", ["--print", "host-tuple"]);
const platform = platforms[target];
if (!platform) throw new Error(`Unsupported desktop target / 不支持的桌面目标: ${target}`);
const version = process.env.RELEASE_VERSION ?? "0.0.0-dev";
const commit = output("git", ["rev-parse", "HEAD"]);
const release = process.env.ONE_DESKTOP_RELEASE === "1";
if (
  release &&
  (!/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.test(version) ||
    output("git", ["status", "--porcelain"]))
) {
  throw new Error(
    "Release builds require a stable version and clean committed source / 正式构建需要稳定版本和干净提交",
  );
}
if (release && process.argv.length > 3)
  throw new Error("Release build options are fixed / 正式构建不接受额外打包参数");
const updater = Boolean(
  process.env.ONE_DESKTOP_UPDATER_PUBLIC_KEY && process.env.TAURI_SIGNING_PRIVATE_KEY,
);
if (
  release &&
  Boolean(process.env.ONE_DESKTOP_UPDATER_PUBLIC_KEY) !==
    Boolean(process.env.TAURI_SIGNING_PRIVATE_KEY)
) {
  throw new Error("Configure both updater keys or neither / 更新公钥和私钥必须成对配置");
}

function build(identity?: string) {
  const channel = release ? "release" : "development";
  const bins = resolve(native, "binaries");
  mkdirSync(bins, { recursive: true });
  run("go", ["run", "./packages/cli/tools/sync-resources", "bundled"]);
  const flags = [
    "-s",
    "-w",
    `-X main.version=${version}`,
    `-X main.commit=${commit}`,
    `-X github.com/torchstellar-team/one-cli/packages/one-client.buildVersion=${release ? version : ""}`,
    `-X github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/updatecheck.buildChannel=${channel}`,
  ];
  if (release)
    flags.push(
      `-X github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/skills.bundledSourceRef=${commit}`,
    );
  const hashes: Record<string, string> = {};
  const signedFiles: Record<string, string> = {};
  for (const [name, entry] of [
    ["one-api", "./services/server/cmd/one-api"],
    ["one", "./packages/cli/cmd/one"],
  ]) {
    const path = resolve(bins, `${name}-${target}${platform.os === "windows" ? ".exe" : ""}`);
    run("go", ["build", "-trimpath", "-ldflags", flags.join(" "), "-o", path, entry], root, {
      ...process.env,
      CGO_ENABLED: "0",
      GOOS: platform.os,
      GOARCH: platform.arch,
    });
    if (identity) {
      signMacSidecar(path, identity);
      signedFiles[`MacOS/${name}`] = path;
    }
    hashes[name] = hash(path);
  }
  const manifest = {
    version,
    commit,
    channel,
    protocol: 2,
    api: 1,
    target,
    sha256: hashes["one-api"],
    cliSha256: hashes.one,
  };
  writeFileSync(resolve(native, "bundle-manifest.json"), JSON.stringify(manifest, null, 2) + "\n");
  const bundle: Record<string, unknown> = { createUpdaterArtifacts: release && updater };
  if (identity) {
    // Tauri signs externalBin again, changing timestamp/hash after the manifest
    // is compiled into Rust. Custom files retain their precomputed signature.
    bundle.externalBin = [];
    bundle.macOS = { files: signedFiles, signingIdentity: identity, hardenedRuntime: true };
  }
  const config: Record<string, unknown> = { version, bundle };
  if (release && updater)
    config.plugins = { updater: { pubkey: process.env.ONE_DESKTOP_UPDATER_PUBLIC_KEY } };
  writeFileSync(resolve(native, "build-config.json"), JSON.stringify(config, null, 2) + "\n");
  if (command === "prepare") return;
  const args = ["exec", "tauri", command, "--config", "src-tauri/build-config.json"];
  const bundleDir = resolve(native, "target", target, "release/bundle");
  if (command === "build") {
    args.push(
      "--target",
      target,
      "--bundles",
      platform.os === "darwin" ? "app,dmg" : platform.os === "windows" ? "nsis" : "appimage,deb",
    );
    if (release) args.push("--ci");
  }
  args.push(...process.argv.slice(3));
  if (release && command === "build") args.push("--", "--locked");
  const env = { ...process.env };
  if (identity) {
    // Use our unlocked temporary keychain; do not import the p12 a second time.
    delete env.APPLE_CERTIFICATE_BASE64;
    delete env.APPLE_CERTIFICATE_PASSWORD;
    delete env.APPLE_CERTIFICATE; // Tauri's built-in import alias, if set by a local shell.
    env.APPLE_SIGNING_IDENTITY = identity;
  }
  for (let attempt = 0; ; attempt++) {
    if (release && command === "build") rmSync(bundleDir, { recursive: true, force: true });
    run("pnpm", args, desktop, env);
    if (command !== "build") return;
    if (platform.os !== "darwin") break;
    const app = resolve(bundleDir, "macos/One Desktop.app");
    for (const [name, digest] of Object.entries(hashes)) {
      if (hash(resolve(app, "Contents/MacOS", name)) !== digest)
        throw new Error(`Bundled binary hash changed / 包内二进制摘要变化: ${name}`);
    }
    if (release) {
      try {
        verifyMacApp(app);
      } catch (error) {
        if (!(error instanceof MissingMacAppTicketError) || attempt >= 2) throw error;
        // Tauri 2.12.2 ignores stapler's exit status. Rebuild both bundles so
        // the DMG contains the app's ticket; never repair only the outer app.
        const delay = attempt === 0 ? 5 : 15;
        console.warn(
          `App ticket missing; rebuilding app and DMG in ${delay}s (${attempt + 1}/2) / 应用缺少票据，${delay} 秒后重新生成应用和 DMG`,
        );
        Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, delay * 1000);
        continue;
      }
      const dmgs = readdirSync(resolve(bundleDir, "dmg")).filter((name) => name.endsWith(".dmg"));
      if (dmgs.length !== 1) throw new Error("Expected one DMG / 需要唯一的 DMG");
      notarizeMacDMG(resolve(bundleDir, "dmg", dmgs[0]));
    }
    break;
  }
  if (release) collectInstallers(root, manifest);
}

if (release && command === "build" && platform.os === "darwin") {
  if (process.platform !== "darwin")
    throw new Error("macOS releases must be built on macOS / macOS 正式版必须在 macOS 构建");
  withMacSigning(build);
} else {
  build(
    platform.os === "darwin" && command === "build"
      // Bind Info.plist and the bundle identifier even in local builds. The
      // linker's bare executable signature cannot identify the app to macOS
      // local network privacy. Release builds use Developer ID above.
      ? (process.env.APPLE_SIGNING_IDENTITY ?? "-")
      : undefined,
  );
}
