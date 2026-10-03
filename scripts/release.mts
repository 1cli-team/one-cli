import { execFileSync, spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import {
  copyFileSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  unlinkSync,
  writeFileSync,
} from "node:fs";
import { arch, platform } from "node:os";
import { dirname, join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import {
  assets,
  assertAssets,
  compareTags,
  decidePlan,
  fail,
  goreleaserVersion,
  parseChecksums,
  repository,
  stableTag,
} from "./release-rules.mts";
import type { Bump, Plan, Release, Tag } from "./release-rules.mts";

type Options = { cwd?: string; env?: NodeJS.ProcessEnv };
type State = { master: string; tags: Tag[]; releases: Release[] };
type Build = {
  schema: "one-cli/release-build/v1";
  plan: Plan;
  source: string;
  config: string;
  notes: string;
  hashes: Record<string, string>;
};

function read(command: string, args: string[], options: Options = {}) {
  return execFileSync(command, args, {
    encoding: "utf8",
    maxBuffer: 32 * 1024 * 1024,
    ...options,
  }).trim();
}
function run(command: string, args: string[], options: Options = {}) {
  const result = spawnSync(command, args, { stdio: "inherit", ...options });
  if (result.error) throw result.error;
  if (result.status !== 0)
    fail(
      `${command} failed (${result.status ?? result.signal}); see the original output above.`,
      `${command} 执行失败（${result.status ?? result.signal}），具体原因见上方原始输出。`,
    );
}
function say(en: string, zh: string) {
  console.log(`${en}\n${zh}`);
}
function sha256(path: string) {
  return createHash("sha256").update(readFileSync(path)).digest("hex");
}
function git(root: string, ...args: string[]) {
  return read("git", args, { cwd: root });
}

function state(root: string): State {
  const refs = git(root, "ls-remote", "origin", "refs/heads/master", "refs/tags/v*");
  const objects = new Map(
    refs.split("\n").map((line) => {
      const [sha, ref] = line.split(/\s+/);
      return [ref, sha];
    }),
  );
  const tags: Tag[] = [];
  for (const [ref, object] of objects) {
    if (!ref?.startsWith("refs/tags/") || !stableTag.test(ref.slice(10))) continue;
    tags.push({ name: ref.slice(10), object, sha: objects.get(`${ref}^{}`) ?? object });
  }
  // JSON Lines works with older gh versions that do not support --slurp.
  const releaseLines = read("gh", [
    "api",
    "--paginate",
    `repos/${repository}/releases?per_page=100`,
    "--jq",
    ".[] | @json",
  ]);
  const master = objects.get("refs/heads/master");
  if (!master) fail("origin/master was not found.", "找不到远端 master。");
  return {
    master,
    tags: tags.sort((a, b) => compareTags(b.name, a.name)),
    releases: releaseLines ? releaseLines.split("\n").map((line) => JSON.parse(line)) : [],
  };
}

function plan(root: string, bump: Bump) {
  const remote = git(root, "remote", "get-url", "origin");
  if (!/^(https:\/\/github\.com\/|git@github\.com:)1cli-team\/one-cli(?:\.git)?$/.test(remote)) {
    fail(
      `Unexpected origin: ${remote}. Expected ${repository}.`,
      `origin 不正确：${remote}。应指向 ${repository}。`,
    );
  }
  if (git(root, "branch", "--show-current") !== "master")
    fail("Run from the master branch.", "请从 master 分支运行。");
  run("git", ["fetch", "origin", "master", "--tags"], { cwd: root });
  const remoteState = state(root);
  const highest = remoteState.tags[0];
  const highestOnMaster =
    !highest ||
    spawnSync("git", ["merge-base", "--is-ancestor", highest.sha, remoteState.master], {
      cwd: root,
    }).status === 0;
  return decidePlan({
    head: git(root, "rev-parse", "HEAD"),
    master: remoteState.master,
    tags: remoteState.tags,
    releases: remoteState.releases,
    bump,
    highestOnMaster,
  });
}

function revalidate(root: string, p: Plan): Release | undefined {
  const current = state(root);
  const targetTag = current.tags.find((t) => t.name === p.tag);
  const release = current.releases.find((r) => r.tag_name === p.tag);
  if (targetTag && targetTag.sha !== p.sourceSha)
    fail(`Tag ${p.tag} moved; stop and inspect it.`, `tag ${p.tag} 已移动，请停止并检查。`);
  if (release?.prerelease)
    fail(`Stable release ${p.tag} is marked as a prerelease.`, `稳定版本 ${p.tag} 被标为预发布。`);
  if (release && !targetTag)
    fail(`Release ${p.tag} has no matching remote tag.`, `Release ${p.tag} 缺少对应的远端 tag。`);
  const highest = current.tags[0];
  // A retry may see the tag pushed by this same build, but no newer release.
  if (targetTag) {
    if (highest?.name !== p.tag || (p.mode === "resume" && targetTag.object !== p.baseTagObject)) {
      fail(
        "The highest stable tag changed; restart the release.",
        "最高稳定 tag 已变化，请重新规划发布。",
      );
    }
  } else if ((highest?.name ?? "") !== p.baseTag || (highest?.object ?? "") !== p.baseTagObject) {
    fail("The base tag changed; restart the release.", "基础 tag 已变化，请重新规划发布。");
  }
  const base = current.tags.find((t) => t.name === p.baseTag);
  if (p.baseTag && base?.object !== p.baseTagObject)
    fail("The base tag moved; restart the release.", "基础 tag 已移动，请重新规划发布。");
  if (p.mode === "bump" && !targetTag && current.master !== p.sourceSha) {
    fail(
      "master advanced during the build; restart the release.",
      "构建期间 master 已前进，请重新规划发布。",
    );
  }
  if (
    spawnSync("git", ["merge-base", "--is-ancestor", p.sourceSha, current.master], { cwd: root })
      .status !== 0
  ) {
    fail(
      "The release commit is no longer on master; stop.",
      "发布提交已不在 master 历史中，请停止发布。",
    );
  }
  if (release && !release.draft) assertAssets(release.assets.map((a) => a.name));
  return release;
}

function verifyArchives(build: Build) {
  const dist = join(build.source, "dist");
  const checksums = parseChecksums(readFileSync(join(dist, "checksums.txt"), "utf8"));
  const hashes = Object.fromEntries(assets.map((name) => [name, sha256(join(dist, name))]));
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

function smoke(build: Build) {
  const p = build.plan;
  const verify = mkdtempSync(join(dirname(build.source), "verify-"));
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
    if (name.endsWith(".zip")) run("unzip", ["-q", file, "-d", dir]);
    else run("tar", ["-xzf", file, "-C", dir]);
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
    const info = read("mise", ["exec", "--", "go", "version", "-m", exe], { cwd: build.source });
    for (const value of [
      `main.version=${p.version}`,
      "updatecheck.buildChannel=release",
      `skills.bundledSourceRef=${p.sourceSha}`,
      "CGO_ENABLED=0",
    ]) {
      if (!info.includes(value))
        fail(
          `Archive ${name} lacks build setting ${value}.`,
          `归档 ${name} 缺少构建设置 ${value}。`,
        );
    }
    if (name === `one-cli_${hostOS}_${hostArch}.tar.gz`) binary = exe;
  }
  const config = join(verify, "config");
  const env = {
    ...process.env,
    CI: "1",
    XDG_CONFIG_HOME: config,
    XDG_DATA_HOME: join(verify, "data"),
    XDG_CACHE_HOME: join(verify, "cache"),
    XDG_STATE_HOME: join(verify, "state"),
  };
  if (read(binary, ["--version"], { cwd: verify, env }) !== p.version)
    fail("Packaged CLI version is incorrect.", "发布包中的 CLI 版本不正确。");
  for (const [locale, helpText] of [
    ["zh-CN", "创建工作区"],
    ["en-US", "Create a workspace"],
  ]) {
    read(binary, ["locale", locale, "-o", "json"], { cwd: verify, env });
    if (!read(binary, ["--help"], { cwd: verify, env }).includes(helpText))
      fail(`CLI help did not switch to ${locale}.`, `CLI 帮助未切换到 ${locale}。`);
    JSON.parse(read(binary, ["templates", "-o", "json"], { cwd: verify, env }));
  }
  say(
    "All five archives, build metadata, versions, and bilingual CLI smoke checks passed.",
    "五个平台的归档、构建信息、版本及中英文 CLI 冒烟检查均通过。",
  );
}

function buildRelease(
  root: string,
  p: Plan,
  notesFile: string,
): { manifest: string; build: Build } {
  if (!notesFile || !existsSync(notesFile) || !readFileSync(notesFile, "utf8").trim()) {
    fail(
      "Provide reviewed release notes with --notes <file> before building.",
      "打包前请通过 --notes <file> 提供已审阅的发布说明。",
    );
  }
  const cache = join(root, ".cache", "releases");
  mkdirSync(cache, { recursive: true });
  const dir = mkdtempSync(join(cache, `${p.tag}-`));
  const source = join(dir, "source");
  run("git", ["clone", "--no-hardlinks", "--no-checkout", root, source]);
  run("git", ["remote", "set-url", "origin", git(root, "remote", "get-url", "origin")], {
    cwd: source,
  });
  run("git", ["checkout", "--detach", p.sourceSha], { cwd: source });
  const config = join(dir, "goreleaser.yaml");
  copyFileSync(join(root, ".goreleaser.yaml"), config);
  const notes = join(dir, "notes.md");
  copyFileSync(notesFile, notes);
  const env = {
    ...process.env,
    GOTOOLCHAIN: "local",
    GOPROXY: "https://proxy.golang.org,direct",
    MISE_TASK_CACHE: "off",
    RELEASE_VERSION: p.version,
  };
  say(
    `Building ${p.tag} from ${p.sourceSha} in ${source}.`,
    `在 ${source} 中从 ${p.sourceSha} 构建 ${p.tag}。`,
  );
  run("mise", ["trust"], { cwd: source, env });
  for (const module of ["packages/kernel", "packages/cli"]) {
    run("mise", ["exec", "--", "go", "-C", module, "mod", "download"], {
      cwd: source,
      env: { ...env, GOWORK: "off" },
    });
  }
  run("mise", ["run", "--jobs", "1", "--force", "check"], { cwd: source, env });
  if (git(source, "status", "--porcelain", "--untracked-files=no"))
    fail(
      "Checks changed tracked source files; stop.",
      "检查修改了受版本控制的源码文件，请停止发布。",
    );
  if (!existsSync(join(source, ".git", "refs", "tags", p.tag))) {
    // show-ref also handles packed refs; never replace an existing tag.
    if (
      spawnSync("git", ["show-ref", "--verify", "--quiet", `refs/tags/${p.tag}`], { cwd: source })
        .status !== 0
    ) {
      run(
        "git",
        [
          "-c",
          "user.name=One CLI Release",
          "-c",
          "user.email=release@1cli.dev",
          "tag",
          "-a",
          p.tag,
          p.sourceSha,
          "-m",
          p.tag,
        ],
        { cwd: source },
      );
    }
  }
  if (git(source, "rev-list", "-n", "1", p.tag) !== p.sourceSha)
    fail("The local release tag has the wrong commit.", "本地发布 tag 指向错误提交。");
  run(
    "mise",
    ["exec", `goreleaser@${goreleaserVersion}`, "--", "goreleaser", "check", "--config", config],
    { cwd: source, env },
  );
  run(
    "mise",
    [
      "exec",
      `goreleaser@${goreleaserVersion}`,
      "--",
      "goreleaser",
      "release",
      "--config",
      config,
      "--skip=publish",
      "--clean",
    ],
    { cwd: source, env: { ...env, GORELEASER_CURRENT_TAG: p.tag } },
  );
  const build: Build = {
    schema: "one-cli/release-build/v1",
    plan: p,
    source,
    config,
    notes,
    hashes: Object.fromEntries(assets.map((name) => [name, sha256(join(source, "dist", name))])),
  };
  verifyArchives(build);
  smoke(build);
  revalidate(root, p);
  const manifest = join(dir, "release.json");
  writeFileSync(manifest, JSON.stringify(build, null, 2) + "\n");
  say(`Verified build: ${manifest}`, `已验证的构建记录：${manifest}`);
  return { manifest, build };
}

function publish(root: string, build: Build) {
  verifyArchives(build);
  const p = build.plan;
  let release = revalidate(root, p);
  if (release && !release.draft) {
    say(`Already published: ${release.html_url}`, `已完成发布：${release.html_url}`);
    return;
  }
  const target = state(root).tags.find((t) => t.name === p.tag);
  if (!target) {
    // Resolve state again immediately before pushing the tag. No force push.
    revalidate(root, p);
    run(
      "git",
      [
        "-c",
        "credential.helper=",
        "-c",
        "credential.helper=!gh auth git-credential",
        "push",
        "origin",
        `refs/tags/${p.tag}:refs/tags/${p.tag}`,
      ],
      { cwd: build.source },
    );
  }
  const currentTag = state(root).tags.find((t) => t.name === p.tag);
  if (
    currentTag?.sha !== p.sourceSha ||
    currentTag.object !== git(build.source, "rev-parse", p.tag)
  ) {
    fail(
      "The pushed tag differs from the verified build; stop.",
      "远端 tag 与已验证构建不一致，请停止发布。",
    );
  }
  release = revalidate(root, p);
  if (release && !release.draft) {
    say(`Already published: ${release.html_url}`, `已完成发布：${release.html_url}`);
    return;
  }
  if (!release)
    run("gh", [
      "release",
      "create",
      p.tag,
      "--repo",
      repository,
      "--verify-tag",
      "--draft",
      "--title",
      p.tag,
      "--notes-file",
      build.notes,
    ]);
  else run("gh", ["release", "edit", p.tag, "--repo", repository, "--notes-file", build.notes]);
  run("gh", [
    "release",
    "upload",
    p.tag,
    "--repo",
    repository,
    "--clobber",
    ...assets.map((name) => join(build.source, "dist", name)),
  ]);
  release = revalidate(root, p);
  if (!release?.draft)
    fail(
      "Release must remain a draft while artifacts are verified.",
      "验证附件时 Release 必须保持草稿状态。",
    );
  assertAssets(release.assets.map((a) => a.name));
  const download = mkdtempSync(join(dirname(build.source), "download-"));
  run("gh", ["release", "download", p.tag, "--repo", repository, "--dir", download]);
  for (const name of assets) {
    if (sha256(join(download, name)) !== build.hashes[name])
      fail(
        `Uploaded asset differs from the verified build: ${name}.`,
        `上传附件与已验证构建不一致：${name}。`,
      );
  }
  release = revalidate(root, p);
  if (!release?.draft)
    fail(
      "Draft release changed during verification; restart.",
      "验证期间草稿状态已变化，请重新执行。",
    );
  say(
    `Draft ${p.tag}: all six downloaded assets match the verified build. Publishing.`,
    `草稿 ${p.tag}：下载验证的六个附件均与构建一致，开始公开发布。`,
  );
  run("gh", ["release", "edit", p.tag, "--repo", repository, "--draft=false", "--latest"]);
  release = revalidate(root, p);
  if (!release || release.draft)
    fail("The release is still a draft; retry publishing.", "Release 仍为草稿，请重试发布。");
  const latest = JSON.parse(read("gh", ["api", `repos/${repository}/releases/latest`]));
  if (latest.tag_name !== p.tag)
    fail(
      "Published release is not Latest; inspect the release state.",
      "已公开版本未成为 Latest，请检查发布状态。",
    );
  say(
    `Published ${p.tag} from ${p.sourceSha}: ${release.html_url}`,
    `已从 ${p.sourceSha} 发布 ${p.tag}：${release.html_url}`,
  );
}

function main() {
  const args = process.argv.slice(2);
  const action = args.shift() ?? "plan";
  if (["--help", "-h", "help"].includes(action)) {
    say(
      "Usage: mise run release -- <plan|build|publish> [patch|minor|major] [--notes <file>] [--from <release.json>]",
      "用法：mise run release -- <plan|build|publish> [patch|minor|major] [--notes <file>] [--from <release.json>]。详见 RELEASING.md。",
    );
    return;
  }
  if (!["plan", "build", "publish"].includes(action))
    fail(`Unknown action: ${action}. Use --help.`, `未知操作：${action}。请查看 --help。`);
  const bump = args[0] && !args[0].startsWith("--") ? args.shift()! : "patch";
  if (!["patch", "minor", "major"].includes(bump))
    fail(`Unsupported bump: ${bump}.`, `不支持的版本递增方式：${bump}。`);
  let notes = "",
    from = "";
  while (args.length) {
    const flag = args.shift();
    const value = args.shift();
    if (!value || value.startsWith("--") || !["--notes", "--from"].includes(flag!))
      fail("Invalid arguments. Use --help.", "参数无效，请查看 --help。");
    if (flag === "--notes") notes = resolve(value);
    else from = resolve(value);
  }
  const root = git(process.cwd(), "rev-parse", "--show-toplevel");
  const lock = join(root, ".cache", "releases", "release.lock");
  if (action !== "plan") {
    mkdirSync(dirname(lock), { recursive: true });
    try {
      writeFileSync(lock, `${process.pid}\n`, { flag: "wx" });
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== "EEXIST") throw error;
      fail(
        `Release lock exists: ${lock}. Check its PID before removing a stale lock.`,
        `发布锁已存在：${lock}。移除残留锁前，请确认记录的进程已退出。`,
      );
    }
    process.once("exit", () => {
      if (existsSync(lock)) unlinkSync(lock);
    });
  }
  if (from) {
    if (action !== "publish" || notes)
      fail(
        "Use --from only with publish and without --notes.",
        "--from 仅用于 publish，且不能同时使用 --notes。",
      );
    const build: Build = JSON.parse(readFileSync(from, "utf8"));
    if (
      build.schema !== "one-cli/release-build/v1" ||
      !stableTag.test(build.plan.tag) ||
      build.plan.version !== build.plan.tag.slice(1)
    ) {
      fail("Invalid build manifest.", "构建记录无效。");
    }
    publish(root, build);
    return;
  }
  const p = plan(root, bump as Bump);
  console.log(JSON.stringify(p, null, 2));
  if (action === "plan" || p.mode === "idempotent") return;
  const { build } = buildRelease(root, p, notes);
  if (action === "publish") publish(root, build);
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try {
    main();
  } catch (error) {
    console.error(error instanceof Error ? error.message : error);
    process.exitCode = 1;
  }
}
