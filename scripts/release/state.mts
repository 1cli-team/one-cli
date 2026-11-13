import {
  assertReleaseAssets,
  compareTags,
  decidePlan,
  repository,
  stableTag,
} from "../release-rules.mts";
import type { Bump, Plan, Release, Tag } from "../release-rules.mts";
import { listGitHubReleases } from "../lib/github.mts";
import { git, gitSucceeds } from "../lib/repository.mts";
import { fail } from "../lib/output.mts";
import { shell } from "../lib/shell.mts";
import type { State } from "./types.mts";

export function state(root: string): State {
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
  const master = objects.get("refs/heads/master");
  if (!master) fail("origin/master was not found.", "找不到远端 master。");
  return {
    master,
    tags: tags.sort((a, b) => compareTags(b.name, a.name)),
    releases: listGitHubReleases<Release>(),
  };
}

export function plan(root: string, bump: Bump) {
  const remote = git(root, "remote", "get-url", "origin");
  if (!/^(https:\/\/github\.com\/|git@github\.com:)1cli-team\/one-cli(?:\.git)?$/.test(remote)) {
    fail(
      `Unexpected origin: ${remote}. Expected ${repository}.`,
      `origin 不正确：${remote}。应指向 ${repository}。`,
    );
  }
  if (git(root, "branch", "--show-current") !== "master")
    fail("Run from the master branch.", "请从 master 分支运行。");
  shell({ cwd: root, stdio: "inherit" })`git fetch origin master --tags`;
  const remoteState = state(root);
  const highest = remoteState.tags[0];
  const highestOnMaster =
    !highest || gitSucceeds(root, "merge-base", "--is-ancestor", highest.sha, remoteState.master);
  return decidePlan({
    head: git(root, "rev-parse", "HEAD"),
    master: remoteState.master,
    tags: remoteState.tags,
    releases: remoteState.releases,
    bump,
    highestOnMaster,
  });
}

export function revalidate(root: string, p: Plan): Release | undefined {
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
  if (!gitSucceeds(root, "merge-base", "--is-ancestor", p.sourceSha, current.master)) {
    fail(
      "The release commit is no longer on master; stop.",
      "发布提交已不在 master 历史中，请停止发布。",
    );
  }
  if (release && !release.draft)
    assertReleaseAssets(
      p.tag,
      release.assets.map((a) => a.name),
    );
  return release;
}
