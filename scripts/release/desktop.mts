import { appendFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { assertReleaseIdentity, assertUnchangedAssets, repository } from "../release-rules.mts";
import { desktopAssets } from "../release-assets.mts";
import { withTempDirectory } from "../lib/files.mts";
import { githubApi } from "../lib/github.mts";
import { fail } from "../lib/output.mts";
import { git, repositoryRoot } from "../lib/repository.mts";
import { shell } from "../lib/shell.mts";
import {
  collectDesktopRecord,
  readCliChecksum,
  verifyRemoteArtifacts,
  writeDesktopArtifacts,
} from "./desktop-artifacts.mts";
import { assertLatestTag, readDesktopSource } from "./desktop-source.mts";
import type { RemoteAsset } from "../release-rules.mts";

const run = shell({ cwd: repositoryRoot });

function readPinnedSource(tag: string, checksum: string) {
  const current = readDesktopSource(tag);
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
}

export function planDesktop(tag: string) {
  const source = readDesktopSource(tag);
  run`git merge-base --is-ancestor ${source.commit} origin/master`;
  run`git cat-file -e ${`${source.commit}:scripts/release-assets.mts`}`;
  const checksum = readCliChecksum(tag);
  if (source.cli.draft) assertLatestTag(tag);
  else verifyRemoteArtifacts(tag, source.version, source.commit, checksum);
  const current = readDesktopSource(tag);
  if (
    current.cli.id !== source.cli.id ||
    current.cli.draft !== source.cli.draft ||
    current.commit !== source.commit ||
    current.tagObject !== source.tagObject
  )
    fail("Release state changed during planning", "规划期间发布状态发生变化");

  const values = {
    tag,
    version: source.version,
    source: source.commit,
    tag_object: source.tagObject,
    release_id: String(source.cli.id),
    cli_checksums_sha256: checksum,
    skip: String(!source.cli.draft),
  };
  if (process.env.GITHUB_OUTPUT)
    appendFileSync(
      process.env.GITHUB_OUTPUT,
      Object.entries(values)
        .map(([key, value]) => `${key}=${value}\n`)
        .join(""),
    );
  console.log(JSON.stringify(values, null, 2));
}

function publishVerifiedRelease(tag: string, checksum: string, verified: RemoteAsset[]) {
  const before = readPinnedSource(tag, checksum).cli;
  if (!before.draft) fail("Release state changed during verification", "验证期间发布状态发生变化");
  assertUnchangedAssets(before.assets, verified);
  assertLatestTag(tag);
  run`gh release edit ${tag} --repo ${repository} --draft=false --latest`;
  const current = readPinnedSource(tag, checksum).cli;
  if (current.draft) fail("Release is still a draft", "Release 仍为草稿");
  assertUnchangedAssets(current.assets, verified);
  if (githubApi<{ tag_name: string }>("releases/latest").tag_name !== tag)
    fail("Published release is not Latest", "已公开版本未成为 Latest");
  console.log(`Published / 发布成功: ${current.html_url}`);
}

export function publishDesktop(tag: string, input: string) {
  if (!input) fail("Provide the downloaded artifact directory", "请提供下载的构建产物目录");
  const source = readDesktopSource(tag);
  if (git(repositoryRoot, "rev-parse", "HEAD") !== source.commit)
    fail("Publisher must use the pinned source", "发布任务必须使用固定源码提交");
  const checksum = process.env.ONE_RELEASE_CLI_CHECKSUMS_SHA256 ?? "";
  readPinnedSource(tag, checksum);
  if (!source.cli.draft) {
    verifyRemoteArtifacts(tag, source.version, source.commit, checksum);
    console.log(`Already published / 已发布: ${source.cli.html_url}`);
    return;
  }
  assertLatestTag(tag);
  if (readCliChecksum(tag) !== checksum)
    fail("CLI checksum manifest changed after planning", "规划后 CLI 校验文件发生变化");

  const directory = resolve(input);
  const record = collectDesktopRecord(directory, source);
  withTempDirectory("one-desktop-publish-", (out) => {
    writeDesktopArtifacts(directory, out, record);
    if (!readPinnedSource(tag, checksum).cli.draft)
      fail("Release must remain a draft during upload", "上传时 Release 必须保持草稿");
    const files = desktopAssets(source.version).map((name) => join(out, name));
    run`gh release upload ${tag} --repo ${repository} --clobber ${files}`;
    const verified = verifyRemoteArtifacts(tag, source.version, source.commit, checksum, out);
    publishVerifiedRelease(tag, checksum, verified);
  });
}
