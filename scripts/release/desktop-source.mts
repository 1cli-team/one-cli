import {
  assets,
  assertDraftAssets,
  assertReleaseAssets,
  isUnifiedRelease,
  stableTag,
  compareTags,
} from "../release-rules.mts";
import { githubApi as api } from "../lib/github.mts";
import { fail } from "../lib/output.mts";
import { git, repositoryRoot as root } from "../lib/repository.mts";
import type { GitHubRelease } from "./types.mts";

type GitObject = { type: string; sha: string };

export function readRelease(tag: string): GitHubRelease | undefined {
  const published = api<GitHubRelease>(`releases/tags/${tag}`, { optional: true });
  if (published) return published;
  // The tag endpoint omits drafts, even when the Git tag already exists.
  // List authenticated releases so upload verification and retries find them.
  const matches: GitHubRelease[] = [];
  for (let page = 1; ; page++) {
    const releases = api<GitHubRelease[]>(`releases?per_page=100&page=${page}`);
    matches.push(...releases.filter((item) => item.tag_name === tag));
    if (releases.length < 100) break;
  }
  if (matches.length > 1) fail("Multiple releases share the same tag", "多个 Release 使用相同 tag");
  return matches[0];
}
function sourceFor(tag: string) {
  let object = api<{ object: GitObject }>(`git/ref/tags/${tag}`, { optional: true })?.object;
  if (!object) fail("Missing stable release tag", "缺少稳定发布 tag");
  const tagObject = object.sha;
  for (let depth = 0; object.type === "tag" && depth < 5; depth++)
    object = api<{ object: GitObject }>(`git/tags/${object.sha}`).object;
  if (object.type !== "commit" || !/^[a-f0-9]{40}$/.test(object.sha))
    fail("Tag must resolve to a commit", "tag 必须指向提交");
  return { commit: object.sha as string, tagObject: tagObject as string };
}
export function readDesktopSource(tag: string) {
  // Reject old tags before any API calls or writes. Historical releases stay unchanged.
  if (!isUnifiedRelease(tag))
    fail(
      "Unified publishing requires v0.1.22 or later; historical releases are read-only",
      "统一发布仅支持 v0.1.22 及以后版本，历史 Release 保持只读",
    );
  const cli = readRelease(tag);
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
export function assertLatestTag(tag: string) {
  const tags = git(root, "ls-remote", "origin", "refs/tags/v*")
    .split("\n")
    .map((line) => line.split(/\s+/)[1]?.replace("refs/tags/", ""))
    .filter((name): name is string => Boolean(name && stableTag.test(name)));
  if (tags.some((name) => compareTags(name, tag) > 0))
    fail(
      "A newer stable tag exists; do not publish an older draft as Latest",
      "已存在更新的稳定 tag，禁止把旧草稿发布为 Latest",
    );
}

export type DesktopSource = ReturnType<typeof readDesktopSource>;
