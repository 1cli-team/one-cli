import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { assets, goreleaserVersion, parseChecksums } from "../release-rules.mts";
import type { Plan } from "../release-rules.mts";
import { sha256File, writeJson } from "../lib/files.mts";
import { git, gitSucceeds } from "../lib/repository.mts";
import { fail, say } from "../lib/output.mts";
import { shell } from "../lib/shell.mts";
import { smoke } from "./smoke.mts";
import { revalidate } from "./state.mts";
import type { Build } from "./types.mts";

export function verifyArchives(build: Build) {
  const dist = join(build.source, "dist");
  const checksums = parseChecksums(readFileSync(join(dist, "checksums.txt"), "utf8"));
  const hashes = Object.fromEntries(assets.map((name) => [name, sha256File(join(dist, name))]));
  for (const [name, expected] of checksums) {
    if (hashes[name] !== expected) fail(`Checksum mismatch: ${name}.`, `文件校验失败：${name}。`);
  }
  if (JSON.stringify(hashes) !== JSON.stringify(build.hashes))
    fail("Build artifacts changed after validation.", "验证完成后构建产物发生变化。");
  if (
    git(build.source, "rev-parse", "HEAD") !== build.plan.sourceSha ||
    git(build.source, "status", "--porcelain", "--untracked-files=no")
  ) {
    fail("The release source checkout changed after validation.", "验证后发布源码副本发生变化。");
  }
}

function prepareWorkspace(root: string, plan: Plan, notesFile: string) {
  if (!notesFile || !existsSync(notesFile) || !readFileSync(notesFile, "utf8").trim()) {
    fail(
      "Provide reviewed release notes with --notes <file> before building.",
      "打包前请通过 --notes <file> 提供已审阅的发布说明。",
    );
  }
  const cache = join(root, ".cache", "releases");
  mkdirSync(cache, { recursive: true });
  const directory = mkdtempSync(join(cache, `${plan.tag}-`));
  const source = join(directory, "source");
  shell({ stdio: "inherit" })`git clone --no-hardlinks --no-checkout ${root} ${source}`;
  const run = shell({ cwd: source, stdio: "inherit" });
  run`git remote set-url origin ${git(root, "remote", "get-url", "origin")}`;
  run`git checkout --detach ${plan.sourceSha}`;
  const config = join(directory, "goreleaser.yaml");
  copyFileSync(join(root, ".goreleaser.yaml"), config);
  const notes = join(directory, "notes.md");
  copyFileSync(notesFile, notes);
  return { directory, source, config, notes };
}

function checkSource(source: string, env: NodeJS.ProcessEnv) {
  const run = shell({ cwd: source, env, stdio: "inherit" });
  run`mise trust`;
  for (const module of ["packages/one-core", "packages/cli", "services/desktop-api"]) {
    shell({
      cwd: source,
      stdio: "inherit",
      env: { ...env, GOWORK: "off" },
    })`mise exec -- go -C ${module} mod download`;
  }
  run`mise run --jobs 1 --force check`;
  if (git(source, "status", "--porcelain", "--untracked-files=no"))
    fail(
      "Checks changed tracked source files; stop.",
      "检查修改了受版本控制的源码文件，请停止发布。",
    );
}

function prepareTag(source: string, plan: Plan) {
  // show-ref also handles packed refs; never replace an existing tag.
  if (!gitSucceeds(source, "show-ref", "--verify", "--quiet", `refs/tags/${plan.tag}`)) {
    shell({
      cwd: source,
      stdio: "inherit",
    })`git -c ${"user.name=One CLI Release"} -c user.email=release@1cli.dev tag -a ${plan.tag} ${plan.sourceSha} -m ${plan.tag}`;
  }
  if (git(source, "rev-list", "-n", "1", plan.tag) !== plan.sourceSha)
    fail("The local release tag has the wrong commit.", "本地发布 tag 指向错误提交。");
}

function createArchives(source: string, config: string, plan: Plan, env: NodeJS.ProcessEnv) {
  const run = shell({ cwd: source, env, stdio: "inherit" });
  const tool = `goreleaser@${goreleaserVersion}`;
  run`mise exec ${tool} -- goreleaser check --config ${config}`;
  shell({
    cwd: source,
    stdio: "inherit",
    env: { ...env, GORELEASER_CURRENT_TAG: plan.tag },
  })`mise exec ${tool} -- goreleaser release --config ${config} --skip=publish --clean`;
}

export function buildRelease(
  root: string,
  plan: Plan,
  notesFile: string,
): { manifest: string; build: Build } {
  const { directory, source, config, notes } = prepareWorkspace(root, plan, notesFile);
  const env = {
    ...process.env,
    GOTOOLCHAIN: "local",
    GOPROXY: "https://proxy.golang.org,direct",
    MISE_TASK_CACHE: "off",
    RELEASE_VERSION: plan.version,
  };
  say(
    `Building ${plan.tag} from ${plan.sourceSha} in ${source}.`,
    `在 ${source} 中从 ${plan.sourceSha} 构建 ${plan.tag}。`,
  );
  checkSource(source, env);
  prepareTag(source, plan);
  createArchives(source, config, plan, env);
  const build: Build = {
    schema: "one-cli/release-build/v1",
    plan,
    source,
    config,
    notes,
    hashes: Object.fromEntries(
      assets.map((name) => [name, sha256File(join(source, "dist", name))]),
    ),
  };
  verifyArchives(build);
  smoke(build);
  revalidate(root, plan);
  const manifest = join(directory, "release.json");
  writeJson(manifest, build);
  say(`Verified build: ${manifest}`, `已验证的构建记录：${manifest}`);
  return { manifest, build };
}
