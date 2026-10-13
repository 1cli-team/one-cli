import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import {
  appendFileSync,
  copyFileSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { assertAssets, fail, parseChecksums, repository, stableTag } from "./release-rules.mts";

export const platforms: Record<
  string,
  { os: string; arch: string; bundle: string; extension: string }
> = {
  "aarch64-apple-darwin": { os: "darwin", arch: "arm64", bundle: "dmg", extension: ".dmg" },
  "x86_64-apple-darwin": { os: "darwin", arch: "amd64", bundle: "dmg", extension: ".dmg" },
  "x86_64-pc-windows-msvc": { os: "windows", arch: "amd64", bundle: "nsis", extension: ".exe" },
  "x86_64-unknown-linux-gnu": {
    os: "linux",
    arch: "amd64",
    bundle: "appimage",
    extension: ".AppImage",
  },
  "aarch64-unknown-linux-gnu": {
    os: "linux",
    arch: "arm64",
    bundle: "appimage",
    extension: ".AppImage",
  },
};
type Manifest = {
  version: string;
  commit: string;
  channel: string;
  target: string;
  sha256: string;
  cliSha256: string;
};
type Installer = { target: string; name: string; sha256: string; notarized: boolean };
type ReleaseRecord = { version: string; commit: string; cliTag: string; artifacts: Installer[] };
type Release = {
  draft: boolean;
  prerelease: boolean;
  body: string;
  html_url: string;
  assets: { name: string; state: string }[];
};
const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const digest = (path: string) => createHash("sha256").update(readFileSync(path)).digest("hex");

function run(bin: string, args: string[]) {
  const result = spawnSync(bin, args, { cwd: root, encoding: "utf8", maxBuffer: 16 * 1024 * 1024 });
  if (result.error || result.status !== 0)
    fail(result.stderr || `${bin} failed`, `${bin} 执行失败`);
  return result.stdout.trim();
}
function api(path: string, fields: string[] = [], optional = false): any {
  const result = spawnSync("gh", ["api", `repos/${repository}/${path}`, ...fields], {
    encoding: "utf8",
    maxBuffer: 16 * 1024 * 1024,
  });
  if (optional && result.status !== 0 && /\(HTTP 404\)/.test(result.stderr)) return undefined;
  if (result.error || result.status !== 0)
    fail(result.stderr || "GitHub API failed", "GitHub API 请求失败");
  return JSON.parse(result.stdout);
}
function release(tag: string): Release | undefined {
  return api(`releases/tags/${tag}`, [], true);
}
function sourceFor(tag: string) {
  let object = api(`git/ref/tags/${tag}`, [], true)?.object;
  if (!object) return undefined;
  for (let depth = 0; object.type === "tag" && depth < 5; depth++)
    object = api(`git/tags/${object.sha}`).object;
  if (object.type !== "commit") fail("Tag must resolve to a commit", "tag 必须指向提交");
  return object.sha as string;
}
function filename(version: string, target: string) {
  const p = platforms[target];
  return `one-desktop_${version}_${p.os}_${p.arch}${p.extension}`;
}
function expectedNames(version: string) {
  return [
    ...Object.keys(platforms).map((target) => filename(version, target)),
    "desktop-release.json",
    "desktop-checksums.txt",
  ].sort();
}
function equalNames(actual: string[], expected: string[]) {
  if (JSON.stringify([...actual].sort()) !== JSON.stringify([...expected].sort()))
    fail("Incomplete or unexpected desktop artifacts", "桌面附件不完整或包含意外文件");
}
function validateRecord(record: ReleaseRecord, version: string, commit: string) {
  if (record.version !== version || record.commit !== commit || record.cliTag !== `v${version}`)
    fail("Desktop source/version mismatch", "桌面源码或版本不一致");
  equalNames(
    record.artifacts.map((a) => a.target),
    Object.keys(platforms),
  );
  for (const a of record.artifacts) {
    if (
      a.name !== filename(version, a.target) ||
      !/^[a-f0-9]{64}$/.test(a.sha256) ||
      a.notarized !== (platforms[a.target].os === "darwin")
    )
      fail(
        "Invalid installer metadata or missing macOS notarization",
        "安装包记录无效或缺少 macOS 公证",
      );
  }
}
function verifyFiles(directory: string, version: string, commit: string) {
  equalNames(readdirSync(directory), expectedNames(version));
  const record: ReleaseRecord = JSON.parse(
    readFileSync(join(directory, "desktop-release.json"), "utf8"),
  );
  validateRecord(record, version, commit);
  const sums = parseChecksums(readFileSync(join(directory, "desktop-checksums.txt"), "utf8"));
  equalNames(
    [...sums.keys()],
    expectedNames(version).filter((name) => name !== "desktop-checksums.txt"),
  );
  for (const [name, hash] of sums) {
    if (digest(join(directory, name)) !== hash)
      fail(`Checksum mismatch: ${name}`, `附件校验失败：${name}`);
  }
  for (const a of record.artifacts) {
    if (sums.get(a.name) !== a.sha256) fail("Manifest checksum mismatch", "发布记录摘要不一致");
  }
  return record;
}
function verifyRemote(tag: string, version: string, commit: string, local?: string) {
  const current = release(tag);
  if (!current || current.prerelease || current.assets.some((a) => a.state !== "uploaded"))
    fail("Release missing or assets not uploaded", "Release 不存在或附件上传未完成");
  equalNames(
    current.assets.map((a) => a.name),
    expectedNames(version),
  );
  const directory = mkdtempSync(join(tmpdir(), "one-desktop-download-"));
  try {
    run("gh", ["release", "download", tag, "--repo", repository, "--dir", directory]);
    verifyFiles(directory, version, commit);
    if (local) {
      for (const name of expectedNames(version)) {
        if (digest(join(directory, name)) !== digest(join(local, name)))
          fail(`Downloaded artifact differs: ${name}`, `下载附件与本地产物不同：${name}`);
      }
    }
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
}
function cliSource(tag: string) {
  if (!stableTag.test(tag)) fail("Specify a stable CLI tag vX.Y.Z", "请指定稳定的 CLI tag vX.Y.Z");
  const cli = release(tag);
  if (!cli || cli.draft || cli.prerelease)
    fail("Publish the CLI release first", "请先完成 CLI 正式发布");
  assertAssets(cli.assets.map((a) => a.name));
  if (cli.assets.some((a) => a.state !== "uploaded"))
    fail("CLI assets are incomplete", "CLI 附件未上传完成");
  const commit = sourceFor(tag);
  if (!commit || !/^[a-f0-9]{40}$/.test(commit))
    fail("Missing CLI source commit", "缺少 CLI 源码提交");
  return { cli, commit, version: tag.slice(1), tag: `desktop-${tag}` };
}

export function collectInstaller(workspace: string, manifest: Manifest) {
  const p = platforms[manifest.target];
  if (!p || manifest.channel !== "release" || !stableTag.test(`v${manifest.version}`))
    fail("Installer collection requires a release build", "安装包收集需要正式构建");
  const directory = join(
    workspace,
    "apps/desktop/src-tauri/target",
    manifest.target,
    "release/bundle",
    p.bundle,
  );
  const files = readdirSync(directory).filter((name) => name.endsWith(p.extension));
  if (files.length !== 1) fail("Expected exactly one installer", "必须生成唯一的安装包");
  const out = join(workspace, "desktop-dist", manifest.target);
  rmSync(out, { recursive: true, force: true });
  mkdirSync(out, { recursive: true });
  const name = filename(manifest.version, manifest.target);
  copyFileSync(join(directory, files[0]), join(out, name));
  const artifact: Installer = {
    target: manifest.target,
    name,
    sha256: digest(join(out, name)),
    notarized: p.os === "darwin",
  };
  // Called only by desktop.mts after signing, notarization and bundle validation.
  writeFileSync(
    join(out, `${manifest.target}.json`),
    JSON.stringify({ version: manifest.version, commit: manifest.commit, artifact }, null, 2) +
      "\n",
  );
  console.log(`Installer ready / 安装包就绪: ${join(out, name)}`);
}

function plan(cliTag: string) {
  const p = cliSource(cliTag);
  run("git", ["merge-base", "--is-ancestor", p.commit, "origin/master"]);
  const required = [
    "apps/desktop/src-tauri/tauri.conf.json",
    "scripts/desktop-release.mts",
    ".github/workflows/desktop.yml",
  ];
  const files = run("git", ["ls-tree", "-r", "--name-only", p.commit, "--", ...required]).split(
    "\n",
  );
  if (required.some((path) => !files.includes(path)))
    fail(
      "This CLI tag predates desktop packaging; publish a new CLI version containing the workflow first",
      "此 CLI tag 尚未包含桌面打包流程，请先发布包含工作流的新 CLI 版本",
    );
  const existingSource = sourceFor(p.tag);
  if (existingSource && existingSource !== p.commit)
    fail("Desktop tag points to another commit; never move it", "桌面 tag 指向其他提交，禁止移动");
  const current = release(p.tag);
  let skip = false;
  if (current && !current.draft) {
    if (existingSource !== p.commit)
      fail("Published desktop tag is missing", "已发布桌面 tag 缺失");
    verifyRemote(p.tag, p.version, p.commit);
    skip = true;
  }
  const values = {
    tag: p.tag,
    cli_tag: cliTag,
    version: p.version,
    source: p.commit,
    skip: String(skip),
  };
  if (process.env.GITHUB_OUTPUT)
    appendFileSync(
      process.env.GITHUB_OUTPUT,
      Object.entries(values)
        .map(([k, v]) => `${k}=${v}\n`)
        .join(""),
    );
  console.log(JSON.stringify(values, null, 2));
}

function publish(cliTag: string, input: string) {
  if (!input) fail("Provide the downloaded artifact directory", "请提供下载的构建产物目录");
  const p = cliSource(cliTag);
  if (run("git", ["rev-parse", "HEAD"]) !== p.commit)
    fail("Publisher must use the pinned source", "发布任务必须使用固定源码提交");
  const directory = resolve(input);
  const record: ReleaseRecord = { version: p.version, commit: p.commit, cliTag, artifacts: [] };
  equalNames(readdirSync(directory), Object.keys(platforms));
  for (const target of Object.keys(platforms)) {
    const path = join(directory, target);
    equalNames(readdirSync(path), [filename(p.version, target), `${target}.json`]);
    const item = JSON.parse(readFileSync(join(path, `${target}.json`), "utf8"));
    if (item.version !== p.version || item.commit !== p.commit || item.artifact.target !== target)
      fail("Build metadata disagrees with source", "构建记录与源码不一致");
    record.artifacts.push(item.artifact);
  }
  validateRecord(record, p.version, p.commit);
  const out = mkdtempSync(join(tmpdir(), "one-desktop-publish-"));
  try {
    for (const a of record.artifacts)
      copyFileSync(join(directory, a.target, a.name), join(out, a.name));
    writeFileSync(join(out, "desktop-release.json"), JSON.stringify(record, null, 2) + "\n");
    writeFileSync(
      join(out, "desktop-checksums.txt"),
      readdirSync(out)
        .sort()
        .map((name) => `${digest(join(out, name))}  ${name}\n`)
        .join(""),
    );
    verifyFiles(out, p.version, p.commit);
    let current = release(p.tag);
    const existingSource = sourceFor(p.tag);
    if (existingSource && existingSource !== p.commit)
      fail("Desktop tag has conflicting source", "桌面 tag 源码冲突");
    if (current && !current.draft) {
      if (!existingSource) fail("Published desktop tag is missing", "已发布桌面 tag 缺失");
      verifyRemote(p.tag, p.version, p.commit);
      console.log(`Already published / 已发布: ${current.html_url}`);
      return;
    }
    if (!existingSource) {
      const object = api("git/tags", [
        "-f",
        `tag=${p.tag}`,
        "-f",
        `message=One Desktop ${p.version}\nSource: ${p.commit}`,
        "-f",
        `object=${p.commit}`,
        "-f",
        "type=commit",
      ]);
      api("git/refs", ["-f", `ref=refs/tags/${p.tag}`, "-f", `sha=${object.sha}`]);
    }
    // Upload only the explicit attachment allowlist; notes are cleaned on failure.
    const notes = join(out, "release-notes.md");
    writeFileSync(
      notes,
      `One Desktop ${p.version}\n\n## 中文\n\n基于 [CLI ${cliTag}](${p.cli.html_url}) 的同一源码提交构建，内置同版本 one-api 与 one。macOS Intel / Apple Silicon 的应用和 DMG 已签名、公证并附加票据；Windows x64 与 Linux x64 / arm64 安装包未签名。下载后可用 desktop-checksums.txt 校验。应用内自动更新尚未启用，请下载新版安装包更新。\n\n## English\n\nBuilt from the same source as [CLI ${cliTag}](${p.cli.html_url}), bundling matching one-api and one binaries. macOS Intel / Apple Silicon apps and DMGs are signed, notarized and stapled. Windows x64 and Linux x64 / arm64 installers are unsigned. Verify downloads with desktop-checksums.txt. In-app updates are not enabled; install the newer package to update.\n\nSource: ${p.commit}\n\n---\n\n${p.cli.body ?? ""}\n`,
    );
    if (!current) {
      run("gh", [
        "release",
        "create",
        p.tag,
        "--repo",
        repository,
        "--verify-tag",
        "--draft",
        "--title",
        `One Desktop ${p.version}`,
        "--notes-file",
        notes,
        "--latest=false",
      ]);
    } else {
      if (
        current.prerelease ||
        current.assets.some((a) => !expectedNames(p.version).includes(a.name))
      )
        fail("Unexpected desktop draft state", "桌面草稿状态或附件异常");
      run("gh", [
        "release",
        "edit",
        p.tag,
        "--repo",
        repository,
        "--title",
        `One Desktop ${p.version}`,
        "--notes-file",
        notes,
      ]);
    }
    rmSync(notes);
    current = release(p.tag);
    if (!current?.draft)
      fail("Release must remain a draft during upload", "上传时 Release 必须保持草稿");
    run("gh", [
      "release",
      "upload",
      p.tag,
      "--repo",
      repository,
      "--clobber",
      ...expectedNames(p.version).map((name) => join(out, name)),
    ]);
    verifyRemote(p.tag, p.version, p.commit, out);
    if (sourceFor(cliTag) !== p.commit || sourceFor(p.tag) !== p.commit || !release(p.tag)?.draft)
      fail("Release state changed during verification", "验证期间发布状态发生变化");
    run("gh", ["release", "edit", p.tag, "--repo", repository, "--draft=false", "--latest=false"]);
    current = release(p.tag);
    if (!current || current.draft)
      fail("Desktop Release is still a draft", "桌面 Release 仍为草稿");
    console.log(`Published / 发布成功: ${current.html_url}`);
  } finally {
    rmSync(out, { recursive: true, force: true });
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const [
    action,
    tag = process.env.ONE_DESKTOP_CLI_TAG ?? "",
    input = process.env.ONE_DESKTOP_ARTIFACTS ?? "",
  ] = process.argv.slice(2);
  if (action === "plan") plan(tag);
  else if (action === "publish") publish(tag, input);
  else
    fail(
      "Usage: node scripts/desktop-release.mts <plan|publish> vX.Y.Z [artifact-directory]",
      "用法：node scripts/desktop-release.mts <plan|publish> vX.Y.Z [构建产物目录]",
    );
}
