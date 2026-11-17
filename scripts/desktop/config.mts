import { resolve } from "node:path";
import { platforms } from "../release-assets.mts";
import { git, repositoryRoot } from "../lib/repository.mts";
import { fail } from "../lib/output.mts";

export type DesktopConfig = ReturnType<typeof readDesktopConfig>;

export function readDesktopConfig(args = process.argv.slice(2), env = process.env) {
  const command = args[0] ?? "prepare";
  if (command !== "prepare" && command !== "build" && command !== "dev")
    fail(`Unknown desktop operation: ${command}`, `未知桌面操作：${command}`);

  const hostTargets: Record<string, string> = {
    "darwin-arm64": "aarch64-apple-darwin",
    "darwin-x64": "x86_64-apple-darwin",
    "win32-x64": "x86_64-pc-windows-msvc",
    "linux-x64": "x86_64-unknown-linux-gnu",
    "linux-arm64": "aarch64-unknown-linux-gnu",
  };
  const target = env.ONE_DESKTOP_TARGET ?? hostTargets[`${process.platform}-${process.arch}`];
  const platform = platforms[target];
  if (!platform) fail(`Unsupported desktop target: ${target}`, `不支持的桌面目标：${target}`);

  const release = env.ONE_DESKTOP_RELEASE === "1";
  const version = env.RELEASE_VERSION ?? "0.0.0-dev";
  if (
    release &&
    (!/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.test(version) ||
      git(repositoryRoot, "status", "--porcelain"))
  )
    fail(
      "Release builds require a stable version and clean committed source",
      "正式构建需要稳定版本和干净提交",
    );
  if (args.length > 1) fail("Desktop build options are fixed", "桌面构建不接受额外打包参数");

  const desktop = resolve(repositoryRoot, "apps/desktop-renderer");
  const native = resolve(repositoryRoot, "services/desktop-main");
  return {
    command,
    target,
    platform,
    release,
    version,
    commit: git(repositoryRoot, "rev-parse", "HEAD"),
    channel: release ? "release" : "development",
    root: repositoryRoot,
    desktop,
    native,
    bundleDirectory: resolve(native, "dist", target),
    env: { ...env },
  };
}
