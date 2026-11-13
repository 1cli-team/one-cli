import { mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import {
  assets,
  assertDraftAssets,
  isUnifiedRelease,
  releaseAssets,
  repository,
} from "../release-rules.mts";
import type { Release } from "../release-rules.mts";
import { sha256File } from "../lib/files.mts";
import { downloadReleaseAssets } from "../lib/github.mts";
import { fail, say } from "../lib/output.mts";
import { git, gitSucceeds } from "../lib/repository.mts";
import { shell } from "../lib/shell.mts";
import { verifyArchives } from "./build.mts";
import { revalidate, state } from "./state.mts";
import type { Build } from "./types.mts";

const run = shell({ stdio: "inherit" });

function alreadyPublished(release: Release | undefined) {
  if (!release || release.draft) return false;
  say(`Already published: ${release.html_url}`, `已完成发布：${release.html_url}`);
  return true;
}

function ensureRemoteTag(root: string, build: Build) {
  const p = build.plan;
  if (!isUnifiedRelease(p.tag))
    fail(
      "Historical releases are read-only in this publisher.",
      "新发布流程不会修改历史 Release。",
    );
  if (!gitSucceeds(root, "cat-file", "-e", `${p.sourceSha}:scripts/release-assets.mts`))
    fail(
      "The source predates unified publishing; merge the release changes first.",
      "该源码尚不支持统一发布，请先合入发布改造。",
    );

  if (!state(root).tags.some((tag) => tag.name === p.tag)) {
    revalidate(root, p);
    const credentials = [
      "-c",
      "credential.helper=",
      "-c",
      "credential.helper=!gh auth git-credential",
    ];
    const ref = `refs/tags/${p.tag}:refs/tags/${p.tag}`;
    shell({ cwd: build.source, stdio: "inherit" })`git ${credentials} push origin ${ref}`;
  }
  const current = state(root).tags.find((tag) => tag.name === p.tag);
  if (current?.sha !== p.sourceSha || current.object !== git(build.source, "rev-parse", p.tag))
    fail(
      "The pushed tag differs from the verified build; stop.",
      "远端 tag 与已验证构建不一致，请停止发布。",
    );
}

function prepareDraft(build: Build, release: Release | undefined) {
  const notes = join(dirname(build.source), "unified-notes.md");
  const downloads = readFileSync(new URL("./downloads.md", import.meta.url), "utf8").trim();
  writeFileSync(
    notes,
    `${readFileSync(build.notes, "utf8").trim()}\n\n${downloads}\n\nSource: ${build.plan.sourceSha}\n`,
  );
  if (!release) {
    const { tag, version } = build.plan;
    const title = `One ${version}`;
    run`gh release create ${tag} --repo ${repository} --verify-tag --draft --title ${title} --notes-file ${notes}`;
  }
}

function uploadCliAssets(root: string, build: Build) {
  const p = build.plan;
  let release = revalidate(root, p);
  if (!release?.draft) fail("Expected a draft release", "需要处于草稿状态的 Release");
  assertDraftAssets(
    p.tag,
    release.assets.map((asset) => asset.name),
  );
  for (const name of assets) {
    release = revalidate(root, p);
    if (!release?.draft) fail("Draft state changed during upload", "上传期间草稿状态已变化");
    const existing = release.assets.find((asset) => asset.name === name);
    if (!existing) {
      run`gh release upload ${p.tag} --repo ${repository} ${join(build.source, "dist", name)}`;
      continue;
    }
    // Existing CLI bytes are immutable while desktop builds use this draft.
    if (existing.state !== "uploaded")
      fail(
        `Incomplete draft asset ${name}; inspect it before retrying.`,
        `草稿附件 ${name} 未上传完成，请检查后重试。`,
      );
    const directory = mkdtempSync(join(dirname(build.source), "existing-"));
    downloadReleaseAssets(p.tag, [name], directory, { live: true });
    if (sha256File(join(directory, name)) !== build.hashes[name])
      fail(
        `Draft asset changed: ${name}; use the original build record.`,
        `草稿附件 ${name} 与本次构建不同，请使用原始构建记录。`,
      );
  }
}

function verifyUploadedAssets(root: string, build: Build) {
  const p = build.plan;
  const release = revalidate(root, p);
  if (!release?.draft)
    fail(
      "Release must remain a draft while artifacts are verified.",
      "验证附件时 Release 必须保持草稿状态。",
    );
  assertDraftAssets(
    p.tag,
    release.assets.map((asset) => asset.name),
    true,
  );
  const directory = mkdtempSync(join(dirname(build.source), "download-"));
  downloadReleaseAssets(p.tag, assets, directory, { live: true });
  for (const name of assets) {
    if (sha256File(join(directory, name)) !== build.hashes[name])
      fail(
        `Uploaded asset differs from the verified build: ${name}.`,
        `上传附件与已验证构建不一致：${name}。`,
      );
  }
}

export function publish(root: string, build: Build) {
  verifyArchives(build);
  if (alreadyPublished(revalidate(root, build.plan))) return;
  ensureRemoteTag(root, build);
  const release = revalidate(root, build.plan);
  if (alreadyPublished(release)) return;
  prepareDraft(build, release);
  uploadCliAssets(root, build);
  verifyUploadedAssets(root, build);

  const { tag } = build.plan;
  if (!revalidate(root, build.plan)?.draft)
    fail(
      "Draft release changed during verification; restart.",
      "验证期间草稿状态已变化，请重新执行。",
    );
  run`gh workflow run desktop.yml --repo ${repository} --ref master -f ${`release_tag=${tag}`}`;
  say(
    `Draft ${tag}: all six CLI assets verified; desktop workflow dispatched. Publication is pending until all ${releaseAssets(tag).length} release assets pass verification: https://github.com/${repository}/actions/workflows/desktop.yml`,
    `草稿 ${tag}：六个 CLI 附件已验证，已触发桌面流水线；尚未公开，全部 ${releaseAssets(tag).length} 个附件验证通过后才会发布：https://github.com/${repository}/actions/workflows/desktop.yml`,
  );
}
