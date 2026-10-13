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
import {
  assets,
  assertDraftAssets,
  assertReleaseAssets,
  assertReleaseIdentity,
  assertUnchangedAssets,
  fail,
  isUnifiedRelease,
  parseChecksums,
  repository,
  stableTag,
  compareTags,
} from "./release-rules.mts";
import type { RemoteAsset } from "./release-rules.mts";
import {
  desktopAssets as expectedNames,
  desktopFilename as filename,
  desktopBundles,
  desktopInstallerNames,
  platforms,
} from "./release-assets.mts";
export { platforms } from "./release-assets.mts";

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
  id: number;
  tag_name: string;
  draft: boolean;
  prerelease: boolean;
  html_url: string;
  assets: RemoteAsset[];
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
  const published = api(`releases/tags/${tag}`, [], true);
  if (published) return published;
  // The tag endpoint omits drafts, even when the Git tag already exists.
  // List authenticated releases so upload verification and retries find them.
  const matches: Release[] = [];
  for (let page = 1; ; page++) {
    const releases: Release[] = api(`releases?per_page=100&page=${page}`);
    matches.push(...releases.filter((item) => item.tag_name === tag));
    if (releases.length < 100) break;
  }
  if (matches.length > 1) fail("Multiple releases share the same tag", "多个 Release 使用相同 tag");
  return matches[0];
}
function sourceFor(tag: string) {
  let object = api(`git/ref/tags/${tag}`, [], true)?.object;
  if (!object) fail("Missing stable release tag", "缺少稳定发布 tag");
  const tagObject = object.sha;
  for (let depth = 0; object.type === "tag" && depth < 5; depth++)
    object = api(`git/tags/${object.sha}`).object;
  if (object.type !== "commit" || !/^[a-f0-9]{40}$/.test(object.sha))
    fail("Tag must resolve to a commit", "tag 必须指向提交");
  return { commit: object.sha as string, tagObject: tagObject as string };
}
function downloadFiles(tag: string, names: string[], directory: string) {
  mkdirSync(directory, { recursive: true });
  run("gh", [
    "release",
    "download",
    tag,
    "--repo",
    repository,
    "--dir",
    directory,
    ...names.flatMap((name) => ["--pattern", name]),
  ]);
}
function cliChecksum(tag: string) {
  const directory = mkdtempSync(join(tmpdir(), "one-cli-checksum-"));
  try {
    downloadFiles(tag, ["checksums.txt"], directory);
    parseChecksums(readFileSync(join(directory, "checksums.txt"), "utf8"));
    return digest(join(directory, "checksums.txt"));
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
}
function verifyCLI(directory: string, checksum: string) {
  equalNames(readdirSync(directory), assets);
  if (digest(join(directory, "checksums.txt")) !== checksum)
    fail("CLI checksum manifest changed after planning", "规划后 CLI 校验文件发生变化");
  const sums = parseChecksums(readFileSync(join(directory, "checksums.txt"), "utf8"));
  for (const [name, expected] of sums) {
    if (digest(join(directory, name)) !== expected)
      fail(`CLI checksum mismatch: ${name}`, `CLI 附件校验失败：${name}`);
  }
}
function equalNames(actual: string[], expected: string[]) {
  if (JSON.stringify([...actual].sort()) !== JSON.stringify([...expected].sort()))
    fail("Incomplete or unexpected desktop artifacts", "桌面附件不完整或包含意外文件");
}
function validateRecord(record: ReleaseRecord, version: string, commit: string) {
  if (record.version !== version || record.commit !== commit || record.cliTag !== `v${version}`)
    fail("Desktop source/version mismatch", "桌面源码或版本不一致");
  equalNames(
    record.artifacts.map((a) => a.name),
    Object.keys(platforms).flatMap((target) => desktopInstallerNames(version, target)),
  );
  for (const a of record.artifacts) {
    if (
      !platforms[a.target] ||
      !desktopInstallerNames(version, a.target).includes(a.name) ||
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
  const sums = parseChecksums(
    readFileSync(join(directory, "desktop-checksums.txt"), "utf8"),
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
function verifyRemote(
  tag: string,
  version: string,
  commit: string,
  checksum: string,
  local?: string,
) {
  const current = release(tag);
  if (!current || current.prerelease || current.assets.some((a) => a.state !== "uploaded"))
    fail("Release missing or assets not uploaded", "Release 不存在或附件上传未完成");
  assertReleaseAssets(
    tag,
    current.assets.map((a) => a.name),
  );
  const directory = mkdtempSync(join(tmpdir(), "one-release-download-"));
  try {
    const cli = join(directory, "cli");
    const desktop = join(directory, "desktop");
    downloadFiles(tag, assets, cli);
    verifyCLI(cli, checksum);
    downloadFiles(tag, expectedNames(version), desktop);
    verifyFiles(desktop, version, commit);
    if (local) {
      for (const name of expectedNames(version)) {
        if (digest(join(desktop, name)) !== digest(join(local, name)))
          fail(`Downloaded artifact differs: ${name}`, `下载附件与本地产物不同：${name}`);
      }
    }
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
  return current.assets;
}
function cliSource(tag: string) {
  // Reject old tags before any API calls or writes. Historical releases stay unchanged.
  if (!isUnifiedRelease(tag))
    fail(
      "Unified publishing requires v0.1.22 or later; historical releases are read-only",
      "统一发布仅支持 v0.1.22 及以后版本，历史 Release 保持只读",
    );
  const cli = release(tag);
  if (!cli || cli.prerelease)
    fail(
      "Prepare the CLI draft with mise run release first",
      "请先通过 mise run release 准备 CLI 草稿",
    );
  assertDraftAssets(
    tag,
    cli.assets.map((a) => a.name),
    true,
  );
  if (cli.assets.some((a) => assets.includes(a.name) && a.state !== "uploaded"))
    fail("CLI assets are incomplete", "CLI 附件未上传完成");
  if (!cli.draft)
    assertReleaseAssets(
      tag,
      cli.assets.map((a) => a.name),
    );
  return { cli, ...sourceFor(tag), version: tag.slice(1), tag };
}
function ensureLatestTag(tag: string) {
  const tags = run("git", ["ls-remote", "origin", "refs/tags/v*"])
    .split("\n")
    .map((line) => line.split(/\s+/)[1]?.replace("refs/tags/", ""))
    .filter((name): name is string => Boolean(name && stableTag.test(name)));
  if (tags.some((name) => compareTags(name, tag) > 0))
    fail(
      "A newer stable tag exists; do not publish an older draft as Latest",
      "已存在更新的稳定 tag，禁止把旧草稿发布为 Latest",
    );
}
export function collectInstallers(workspace: string, manifest: Manifest) {
  const p = platforms[manifest.target];
  if (!p || manifest.channel !== "release" || !stableTag.test(`v${manifest.version}`))
    fail("Installer collection requires a release build", "安装包收集需要正式构建");
  const sources = desktopBundles(manifest.version, manifest.target).map((bundle) => {
    const directory = join(
      workspace,
      "apps/desktop/src-tauri/target",
      manifest.target,
      "release/bundle",
      bundle,
    );
    const extension = bundle === "deb" ? ".deb" : p.extension;
    const files = readdirSync(directory).filter((name) => name.endsWith(extension));
    if (files.length !== 1)
      fail("Expected exactly one installer per format", "每种格式必须生成唯一的安装包");
    return {
      path: join(directory, files[0]),
      name: filename(manifest.version, manifest.target, bundle),
    };
  });
  const out = join(workspace, "desktop-dist", manifest.target);
  rmSync(out, { recursive: true, force: true });
  mkdirSync(out, { recursive: true });
  const artifacts: Installer[] = sources.map(({ path, name }) => {
    copyFileSync(path, join(out, name));
    console.log(`Installer ready / 安装包就绪: ${join(out, name)}`);
    return {
      target: manifest.target,
      name,
      sha256: digest(join(out, name)),
      notarized: p.os === "darwin",
    };
  });
  // Called only by desktop.mts after signing, notarization and bundle validation.
  writeFileSync(
    join(out, `${manifest.target}.json`),
    JSON.stringify({ version: manifest.version, commit: manifest.commit, artifacts }, null, 2) +
      "\n",
  );
}

function plan(tag: string) {
  const p = cliSource(tag);
  run("git", ["merge-base", "--is-ancestor", p.commit, "origin/master"]);
  run("git", ["cat-file", "-e", `${p.commit}:scripts/release-assets.mts`]);
  const checksum = cliChecksum(tag);
  if (p.cli.draft) ensureLatestTag(tag);
  else verifyRemote(tag, p.version, p.commit, checksum);
  const current = cliSource(tag);
  if (
    current.cli.id !== p.cli.id ||
    current.cli.draft !== p.cli.draft ||
    current.commit !== p.commit ||
    current.tagObject !== p.tagObject
  )
    fail("Release state changed during planning", "规划期间发布状态发生变化");
  const values = {
    tag,
    version: p.version,
    source: p.commit,
    tag_object: p.tagObject,
    release_id: String(p.cli.id),
    cli_checksums_sha256: checksum,
    skip: String(!p.cli.draft),
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

function publish(tag: string, input: string) {
  if (!input) fail("Provide the downloaded artifact directory", "请提供下载的构建产物目录");
  const p = cliSource(tag);
  if (run("git", ["rev-parse", "HEAD"]) !== p.commit)
    fail("Publisher must use the pinned source", "发布任务必须使用固定源码提交");
  const directory = resolve(input);
  const checksum = process.env.ONE_RELEASE_CLI_CHECKSUMS_SHA256 ?? "";
  const pinned = () => {
    const current = cliSource(tag);
    assertReleaseIdentity(
      { commit: current.commit, tagObject: current.tagObject, releaseID: String(current.cli.id) },
      {
        commit: process.env.ONE_RELEASE_SOURCE ?? "",
        tagObject: process.env.ONE_RELEASE_TAG_OBJECT ?? "",
        releaseID: process.env.ONE_RELEASE_ID ?? "",
      },
    );
    if (!/^[a-f0-9]{64}$/.test(checksum))
      fail("Missing frozen CLI checksum", "缺少冻结的 CLI 校验文件摘要");
    return current;
  };
  pinned();
  if (!p.cli.draft) {
    verifyRemote(tag, p.version, p.commit, checksum);
    console.log(`Already published / 已发布: ${p.cli.html_url}`);
    return;
  }
  ensureLatestTag(tag);
  if (cliChecksum(tag) !== checksum)
    fail("CLI checksum manifest changed after planning", "规划后 CLI 校验文件发生变化");
  const record: ReleaseRecord = {
    version: p.version,
    commit: p.commit,
    cliTag: tag,
    artifacts: [],
  };
  equalNames(readdirSync(directory), Object.keys(platforms));
  for (const target of Object.keys(platforms)) {
    const path = join(directory, target);
    equalNames(readdirSync(path), [...desktopInstallerNames(p.version, target), `${target}.json`]);
    const item = JSON.parse(readFileSync(join(path, `${target}.json`), "utf8"));
    // v0.1.22 build records contain one artifact; newer builds may contain several formats.
    const artifacts: Installer[] = item.artifacts ?? [item.artifact];
    if (
      item.version !== p.version ||
      item.commit !== p.commit ||
      !Array.isArray(artifacts) ||
      artifacts.some((artifact) => !artifact || artifact.target !== target)
    )
      fail("Build metadata disagrees with source", "构建记录与源码不一致");
    record.artifacts.push(...artifacts);
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
    if (!pinned().cli.draft)
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
    const verified = verifyRemote(tag, p.version, p.commit, checksum, out);
    const beforePublish = pinned().cli;
    if (!beforePublish.draft)
      fail("Release state changed during verification", "验证期间发布状态发生变化");
    assertUnchangedAssets(beforePublish.assets, verified);
    ensureLatestTag(tag);
    run("gh", ["release", "edit", tag, "--repo", repository, "--draft=false", "--latest"]);
    const current = pinned().cli;
    if (current.draft) fail("Release is still a draft", "Release 仍为草稿");
    assertUnchangedAssets(current.assets, verified);
    if (api("releases/latest").tag_name !== tag)
      fail("Published release is not Latest", "已公开版本未成为 Latest");
    console.log(`Published / 发布成功: ${current.html_url}`);
  } finally {
    rmSync(out, { recursive: true, force: true });
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const [
    action,
    tag = process.env.ONE_RELEASE_TAG ?? "",
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
