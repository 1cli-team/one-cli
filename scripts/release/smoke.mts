import { copyFileSync, chmodSync, existsSync, mkdirSync, mkdtempSync, readFileSync } from "node:fs";
import { arch, platform, tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { assets, assertBuildSettings } from "../release-rules.mts";
import { readJson } from "../lib/files.mts";
import { fail, say } from "../lib/output.mts";
import { shell } from "../lib/shell.mts";
import type { Build } from "./types.mts";

const run = shell({ stdio: "inherit" });

function verifyArchives(build: Build, verify: string) {
  const p = build.plan;
  const hostOS = platform() === "darwin" ? "darwin" : platform() === "linux" ? "linux" : "";
  const hostArch = arch() === "x64" ? "amd64" : arch() === "arm64" ? "arm64" : "";
  if (!hostOS || !hostArch)
    fail(
      "Run release validation on Linux or macOS, amd64 or arm64.",
      "请在 Linux 或 macOS 的 amd64/arm64 主机上验证发布包。",
    );
  let binary = "";
  for (const name of assets.slice(1)) {
    const dir = join(verify, name);
    mkdirSync(dir);
    const file = join(build.source, "dist", name);
    if (name.endsWith(".zip")) run`unzip -q ${file} -d ${dir}`;
    else run`tar -xzf ${file} -C ${dir}`;
    const exe = join(dir, name.includes("windows") ? "one.exe" : "one");
    if (
      !existsSync(exe) ||
      !existsSync(join(dir, "README.md")) ||
      !existsSync(join(dir, "third_party/mise/LICENSE"))
    ) {
      fail(
        `Archive ${name} is missing the binary, README, or mise license.`,
        `归档 ${name} 缺少程序、README 或 mise 许可证。`,
      );
    }
    const info = shell({ cwd: build.source })`mise exec -- go version -m ${exe}`.stdout;
    const target = /^one-cli_(darwin|linux|windows)_(amd64|arm64)\./.exec(name)!;
    assertBuildSettings(info, p.sourceSha, target[1], target[2]);
    if (name === `one-cli_${hostOS}_${hostArch}.tar.gz`) binary = exe;
  }
  return binary;
}

function verifyCLI(build: Build, binary: string, verify: string, env: NodeJS.ProcessEnv) {
  const p = build.plan;
  const cli = shell({ cwd: verify, env });
  if (cli`${binary} --version`.stdout.trim() !== p.version)
    fail("Packaged CLI version is incorrect.", "发布包中的 CLI 版本不正确。");
  for (const locale of ["zh-CN", "en-US"]) {
    const switched = JSON.parse(cli`${binary} locale ${locale} -o json`.stdout);
    const dictionary = readJson<Record<string, string>>(
      join(build.source, "packages/one-core/pkg/platform/i18n/locales", `${locale}.json`),
    );
    if (
      switched.resolved !== locale ||
      cli`${binary} --help`.stdout.trim() !== dictionary["root.help"].trim()
    )
      fail(`CLI help did not switch to ${locale}.`, `CLI 帮助未切换到 ${locale}。`);
    JSON.parse(cli`${binary} templates -o json`.stdout);
  }
  // Validate the release channel through the real manual-upgrade worker.
  // The worker and any replacement stay inside this temporary extraction.
  const upgrade = JSON.parse(cli`${binary} upgrade -o json`.stdout);
  if (
    upgrade.schema !== "one-cli/upgrade/v1" ||
    upgrade.current_version !== p.version ||
    upgrade.status !== "current"
  ) {
    fail(
      "The packaged CLI did not report a current release-channel installation.",
      "发布包中的 CLI 未确认当前正式发布通道版本。",
    );
  }
}

function verifySkills(build: Build, binary: string, verify: string, env: NodeJS.ProcessEnv) {
  const p = build.plan;
  // Record the external skills command instead of installing third-party skills.
  // Official files and their GitHub lock entries come from this binary, without
  // a remote install. Only third-party contents use an offline npx fixture.
  const bin = join(verify, "bin");
  mkdirSync(bin);
  const calls = join(verify, "skills-calls.jsonl");
  const fixture = fileURLToPath(new URL("./fixtures/skills-installer.mjs", import.meta.url));
  copyFileSync(fixture, join(bin, "npx"));
  chmodSync(join(bin, "npx"), 0o755);
  const offlineEnv = {
    ...env,
    ONE_SKILLS_SMOKE_CALLS: calls,
    PATH: `${bin}:${process.env.PATH ?? ""}`,
  };
  const offline = shell({ cwd: verify, env: offlineEnv });
  offline`${binary} create ${join(verify, "workspace")} --yes -o json`;
  const commands = readFileSync(calls, "utf8")
    .trim()
    .split("\n")
    .map((line) => JSON.parse(line));
  if (commands.some((args) => args.some((arg: string) => arg.startsWith("1cli-team/one-cli"))))
    fail(
      "The packaged CLI fetched its official skills instead of using embedded files.",
      "发布包中的 CLI 下载了官方技能，未使用内置文件。",
    );
  const installed = readFileSync(join(verify, "workspace/.agents/skills/one-cli/SKILL.md"), "utf8");
  const expected = readFileSync(
    join(build.source, "packages/agent-skills/skills/one-cli/SKILL.md"),
    "utf8",
  );
  if (installed !== expected)
    fail("The packaged skill differs from the release source.", "发布包中的技能与发布源码不一致。");
  const lock = readJson<{ skills: Record<string, Record<string, string>> }>(
    join(verify, "workspace/skills-lock.json"),
  );
  const entry = lock.skills["one-cli"];
  if (
    entry?.source !== "1cli-team/one-cli" ||
    entry.sourceType !== "github" ||
    entry.ref !== "master" ||
    entry.skillPath !== "packages/agent-skills/skills/one-cli/SKILL.md" ||
    !/^[a-f0-9]{64}$/.test(entry.computedHash)
  )
    fail(
      "The bundled skill is missing its upstream update source.",
      "内置技能缺少上游更新来源登记。",
    );
  const skillPreview = JSON.parse(
    shell({
      cwd: join(verify, "workspace"),
      env: offlineEnv,
    })`${binary} init skills --dry-run -o json`.stdout,
  );
  if (
    skillPreview.sourceRef !== p.sourceSha ||
    skillPreview.skills.find((skill: { name: string; status: string }) => skill.name === "one-cli")
      ?.status !== "managed" ||
    skillPreview.changes.length !== 0
  )
    fail(
      "Bundled skill provenance or initialization preview is incorrect.",
      "内置技能来源或初始化预览不正确。",
    );
}

export function smoke(build: Build) {
  const verify = mkdtempSync(join(tmpdir(), "one-cli-release-smoke-"));
  const binary = verifyArchives(build, verify);
  const env = { ...process.env, CI: "1", ONE_HOME: join(verify, "one-home") };
  verifyCLI(build, binary, verify, env);
  verifySkills(build, binary, verify, env);
  say(
    "All five archives, build metadata, versions, and bilingual CLI smoke checks passed.",
    "五个平台的归档、构建信息、版本及中英文 CLI 冒烟检查均通过。",
  );
}
