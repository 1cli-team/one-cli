import { resolve } from "node:path";
import { platforms } from "../release-assets.mts";
import { git, repositoryRoot } from "../lib/repository.mts";
import { shell } from "../lib/shell.mts";
import { fail } from "../lib/output.mts";

export type DesktopConfig = ReturnType<typeof readDesktopConfig>;

export function readDesktopConfig(args = process.argv.slice(2), env = process.env) {
  const command = args[0] ?? "prepare";
  if (command !== "prepare" && command !== "build" && command !== "dev")
    fail(`Unknown desktop operation: ${command}`, `未知桌面操作：${command}`);

  const read = shell({ cwd: repositoryRoot, env });
  const target = env.ONE_DESKTOP_TARGET ?? read`rustc --print host-tuple`.stdout.trim();
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
  if (release && args.length > 1)
    fail("Release build options are fixed", "正式构建不接受额外打包参数");

  const updater = Boolean(env.ONE_DESKTOP_UPDATER_PUBLIC_KEY && env.TAURI_SIGNING_PRIVATE_KEY);
  if (
    release &&
    Boolean(env.ONE_DESKTOP_UPDATER_PUBLIC_KEY) !== Boolean(env.TAURI_SIGNING_PRIVATE_KEY)
  )
    fail("Configure both updater keys or neither", "更新公钥和私钥必须成对配置");

  const desktop = resolve(repositoryRoot, "apps/desktop");
  const native = resolve(desktop, "src-tauri");
  return {
    command,
    target,
    platform,
    release,
    version,
    updater,
    commit: git(repositoryRoot, "rev-parse", "HEAD"),
    channel: release ? "release" : "development",
    root: repositoryRoot,
    desktop,
    native,
    bundleDirectory: resolve(native, "target", target, "release/bundle"),
    extraArgs: args.slice(1),
    env: { ...env },
  };
}
