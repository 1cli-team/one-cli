import { existsSync, mkdirSync, unlinkSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { stableTag } from "../release-rules.mts";
import type { Bump } from "../release-rules.mts";
import { readJson } from "../lib/files.mts";
import { fail, say } from "../lib/output.mts";
import { git } from "../lib/repository.mts";
import { buildRelease } from "./build.mts";
import { publish } from "./publish.mts";
import { plan } from "./state.mts";
import type { Build } from "./types.mts";

function readOptions(args: string[]) {
  const action = args.shift() ?? "plan";
  if (["--help", "-h", "help"].includes(action)) {
    say(
      "Usage: mise run release -- <plan|build|publish> [patch|minor|major] [--notes <file>] [--from <release.json>]",
      "用法：mise run release -- <plan|build|publish> [patch|minor|major] [--notes <file>] [--from <release.json>]。详见 RELEASING.md。",
    );
    return;
  }
  if (action !== "plan" && action !== "build" && action !== "publish")
    fail(`Unknown action: ${action}. Use --help.`, `未知操作：${action}。请查看 --help。`);
  const bump = args[0] && !args[0].startsWith("--") ? args.shift()! : "patch";
  if (!["patch", "minor", "major"].includes(bump))
    fail(`Unsupported bump: ${bump}.`, `不支持的版本递增方式：${bump}。`);
  let notes = "",
    from = "";
  while (args.length) {
    const flag = args.shift();
    const value = args.shift();
    if (!value || value.startsWith("--") || (flag !== "--notes" && flag !== "--from"))
      fail("Invalid arguments. Use --help.", "参数无效，请查看 --help。");
    if (flag === "--notes") notes = resolve(value);
    else from = resolve(value);
  }
  return { action, bump: bump as Bump, notes, from };
}

function acquireLock(root: string) {
  const path = join(root, ".cache/releases/release.lock");
  mkdirSync(dirname(path), { recursive: true });
  try {
    writeFileSync(path, `${process.pid}\n`, { flag: "wx" });
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code !== "EEXIST") throw error;
    fail(
      `Release lock exists: ${path}. Check its PID before removing a stale lock.`,
      `发布锁已存在：${path}。移除残留锁前，请确认记录的进程已退出。`,
    );
  }
  const release = () => {
    if (existsSync(path)) unlinkSync(path);
  };
  process.once("exit", release);
  return () => {
    process.removeListener("exit", release);
    release();
  };
}

export function runRelease(args = process.argv.slice(2)) {
  const options = readOptions([...args]);
  if (!options) return;
  const root = git(process.cwd(), "rev-parse", "--show-toplevel");
  const unlock = options.action === "plan" ? () => {} : acquireLock(root);
  try {
    if (options.from) {
      if (options.action !== "publish" || options.notes)
        fail(
          "Use --from only with publish and without --notes.",
          "--from 仅用于 publish，且不能同时使用 --notes。",
        );
      const build = readJson<Build>(options.from);
      if (
        build.schema !== "one-cli/release-build/v1" ||
        !stableTag.test(build.plan.tag) ||
        build.plan.version !== build.plan.tag.slice(1)
      )
        fail("Invalid build manifest.", "构建记录无效。");
      publish(root, build);
      return;
    }
    const planned = plan(root, options.bump);
    console.log(JSON.stringify(planned, null, 2));
    if (options.action === "plan" || planned.mode === "idempotent") return;
    const { build } = buildRelease(root, planned, options.notes);
    if (options.action === "publish") publish(root, build);
  } finally {
    unlock();
  }
}
