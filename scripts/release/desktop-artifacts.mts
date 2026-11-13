import { copyFileSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { assets, assertReleaseAssets, parseChecksums } from "../release-rules.mts";
import {
  desktopAssets as expectedNames,
  desktopInstallerNames,
  platforms,
} from "../release-assets.mts";
import { readJson, sha256File, withTempDirectory, writeJson } from "../lib/files.mts";
import { downloadReleaseAssets } from "../lib/github.mts";
import { fail } from "../lib/output.mts";
import type {
  DesktopBuildRecord as BuildRecord,
  DesktopReleaseRecord,
  Installer,
} from "../desktop/types.mts";
import { readRelease } from "./desktop-source.mts";
import type { DesktopSource } from "./desktop-source.mts";

export function readCliChecksum(tag: string) {
  return withTempDirectory("one-cli-checksum-", (directory) => {
    downloadReleaseAssets(tag, ["checksums.txt"], directory);
    parseChecksums(readFileSync(join(directory, "checksums.txt"), "utf8"));
    return sha256File(join(directory, "checksums.txt"));
  });
}
function verifyCliFiles(directory: string, checksum: string) {
  assertFileNames(readdirSync(directory), assets);
  if (sha256File(join(directory, "checksums.txt")) !== checksum)
    fail("CLI checksum manifest changed after planning", "规划后 CLI 校验文件发生变化");
  const sums = parseChecksums(readFileSync(join(directory, "checksums.txt"), "utf8"));
  for (const [name, expected] of sums) {
    if (sha256File(join(directory, name)) !== expected)
      fail(`CLI checksum mismatch: ${name}`, `CLI 附件校验失败：${name}`);
  }
}
export function assertFileNames(actual: string[], expected: string[]) {
  if (JSON.stringify([...actual].sort()) !== JSON.stringify([...expected].sort()))
    fail("Incomplete or unexpected desktop artifacts", "桌面附件不完整或包含意外文件");
}
export function validateDesktopRecord(
  record: DesktopReleaseRecord,
  version: string,
  commit: string,
) {
  if (record.version !== version || record.commit !== commit || record.cliTag !== `v${version}`)
    fail("Desktop source/version mismatch", "桌面源码或版本不一致");
  assertFileNames(
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
export function verifyDesktopFiles(directory: string, version: string, commit: string) {
  assertFileNames(readdirSync(directory), expectedNames(version));
  const record = readJson<DesktopReleaseRecord>(join(directory, "desktop-release.json"));
  validateDesktopRecord(record, version, commit);
  const sums = parseChecksums(
    readFileSync(join(directory, "desktop-checksums.txt"), "utf8"),
    expectedNames(version).filter((name) => name !== "desktop-checksums.txt"),
  );
  for (const [name, hash] of sums) {
    if (sha256File(join(directory, name)) !== hash)
      fail(`Checksum mismatch: ${name}`, `附件校验失败：${name}`);
  }
  for (const a of record.artifacts) {
    if (sums.get(a.name) !== a.sha256) fail("Manifest checksum mismatch", "发布记录摘要不一致");
  }
  return record;
}
export function verifyRemoteArtifacts(
  tag: string,
  version: string,
  commit: string,
  checksum: string,
  local?: string,
) {
  const current = readRelease(tag);
  if (!current || current.prerelease || current.assets.some((a) => a.state !== "uploaded"))
    fail("Release missing or assets not uploaded", "Release 不存在或附件上传未完成");
  assertReleaseAssets(
    tag,
    current.assets.map((a) => a.name),
  );
  withTempDirectory("one-release-download-", (directory) => {
    const cli = join(directory, "cli");
    const desktop = join(directory, "desktop");
    downloadReleaseAssets(tag, assets, cli);
    verifyCliFiles(cli, checksum);
    downloadReleaseAssets(tag, expectedNames(version), desktop);
    verifyDesktopFiles(desktop, version, commit);
    if (local) {
      for (const name of expectedNames(version)) {
        if (sha256File(join(desktop, name)) !== sha256File(join(local, name)))
          fail(`Downloaded artifact differs: ${name}`, `下载附件与本地产物不同：${name}`);
      }
    }
  });
  return current.assets;
}

export function collectDesktopRecord(directory: string, p: DesktopSource) {
  const record: DesktopReleaseRecord = {
    version: p.version,
    commit: p.commit,
    cliTag: p.tag,
    artifacts: [],
  };
  assertFileNames(readdirSync(directory), Object.keys(platforms));
  for (const target of Object.keys(platforms)) {
    const path = join(directory, target);
    assertFileNames(readdirSync(path), [
      ...desktopInstallerNames(p.version, target),
      `${target}.json`,
    ]);
    const item = readJson<BuildRecord>(join(path, `${target}.json`));
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
  validateDesktopRecord(record, p.version, p.commit);
  return record;
}

export function writeDesktopArtifacts(
  directory: string,
  out: string,
  record: DesktopReleaseRecord,
) {
  for (const a of record.artifacts)
    copyFileSync(join(directory, a.target, a.name), join(out, a.name));
  writeJson(join(out, "desktop-release.json"), record);
  writeFileSync(
    join(out, "desktop-checksums.txt"),
    readdirSync(out)
      .sort()
      .map((name) => `${sha256File(join(out, name))}  ${name}\n`)
      .join(""),
  );
  verifyDesktopFiles(out, record.version, record.commit);
}
