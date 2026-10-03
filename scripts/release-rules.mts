export const repository = "1cli-team/one-cli";
export const goreleaserVersion = "2.18.0";
export const assets = [
  "checksums.txt",
  "one-cli_darwin_amd64.tar.gz",
  "one-cli_darwin_arm64.tar.gz",
  "one-cli_linux_amd64.tar.gz",
  "one-cli_linux_arm64.tar.gz",
  "one-cli_windows_amd64.zip",
];
export const stableTag = /^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/;
export type Bump = "patch" | "minor" | "major";
export type Tag = { name: string; object: string; sha: string };
export type Release = {
  tag_name: string;
  draft: boolean;
  prerelease: boolean;
  html_url: string;
  assets: { name: string; digest?: string; state: string }[];
};
export type Plan = {
  mode: "bump" | "resume" | "idempotent";
  tag: string;
  version: string;
  sourceSha: string;
  masterSha: string;
  baseTag: string;
  baseTagObject: string;
};

export function fail(en: string, zh: string): never {
  throw new Error(`${en}\n${zh}`);
}

export function assertAssets(names: string[]) {
  if (JSON.stringify([...names].sort()) !== JSON.stringify([...assets].sort())) {
    fail(
      `Unexpected release assets: ${names.join(", ")}. Expected: ${assets.join(", ")}.`,
      `发布附件不完整或包含意外文件：${names.join("，")}。应包含：${assets.join("，")}。`,
    );
  }
}

export function compareTags(a: string, b: string) {
  const left = a.slice(1).split(".").map(BigInt);
  const right = b.slice(1).split(".").map(BigInt);
  for (let i = 0; i < 3; i++) {
    if (left[i] !== right[i]) return left[i] > right[i] ? 1 : -1;
  }
  return 0;
}

export function bumpTag(tag: string, bump: Bump) {
  const version = tag.slice(1).split(".").map(BigInt);
  const index = { major: 0, minor: 1, patch: 2 }[bump];
  version[index]++;
  for (let i = index + 1; i < 3; i++) version[i] = 0n;
  return `v${version.join(".")}`;
}

export function decidePlan(input: {
  head: string;
  master: string;
  tags: Tag[];
  releases: Release[];
  bump: Bump;
  highestOnMaster: boolean;
}): Plan {
  const { head, master, releases, highestOnMaster, bump } = input;
  if (head !== master)
    fail(
      "HEAD differs from origin/master; update the checkout first.",
      "HEAD 与远端 master 不一致，请先更新工作区。",
    );
  const tags = input.tags
    .filter((t) => stableTag.test(t.name))
    .sort((a, b) => compareTags(b.name, a.name));
  const highest = tags[0];
  const latest = releases.find((r) => r.tag_name === highest?.name);
  let mode: Plan["mode"] = "bump";
  if (highest) {
    if (latest?.prerelease)
      fail(
        `Stable tag ${highest.name} is marked as a prerelease; resolve it first.`,
        `稳定 tag ${highest.name} 被标为预发布，请先处理该状态。`,
      );
    if (!latest) {
      if (!highestOnMaster)
        fail(
          `Tag ${highest.name} is off master and has no release; resolve it first.`,
          `tag ${highest.name} 不在 master 上且没有 Release，请先处理。`,
        );
      mode = "resume";
    } else if (latest.draft) {
      if (highestOnMaster) mode = "resume";
    } else {
      assertAssets(latest.assets.map((a) => a.name));
      if (highestOnMaster && highest.sha === head) mode = "idempotent";
    }
  }
  const tag = mode === "bump" ? bumpTag(highest?.name ?? "v0.0.0", bump) : highest!.name;
  if (mode === "bump" && tags.some((t) => t.sha === head)) {
    fail(
      "This commit already has a stable tag; resolve its release before creating another.",
      "当前提交已有稳定 tag，请先处理对应 Release，避免重复打版本。",
    );
  }
  if (mode === "bump" && releases.some((r) => r.tag_name === tag)) {
    fail(
      `Release ${tag} exists without its Git tag; resolve it first.`,
      `Release ${tag} 存在但缺少 Git tag，请先处理。`,
    );
  }
  return {
    mode,
    tag,
    version: tag.slice(1),
    sourceSha: mode === "bump" ? head : highest!.sha,
    masterSha: master,
    baseTag: highest?.name ?? "",
    baseTagObject: highest?.object ?? "",
  };
}

export function parseChecksums(text: string): Map<string, string> {
  const checksums = new Map<string, string>();
  for (const line of text.trim().split(/\r?\n/)) {
    const match = /^([a-f0-9]{64})\s+\*?([^/\\\s]+)$/.exec(line);
    if (!match || checksums.has(match[2]))
      fail("Invalid or duplicate checksum entry.", "校验文件含无效或重复条目。");
    checksums.set(match[2], match[1]);
  }
  assertAssets(["checksums.txt", ...checksums.keys()]);
  return checksums;
}

export function assertBuildSettings(info: string, sha: string, os: string, architecture: string) {
  const settings = new Map(
    info.split("\n").flatMap((line) => {
      const match = /^\s*build\s+([^=]+)=(.*)$/.exec(line);
      return match ? [[match[1], match[2]] as [string, string]] : [];
    }),
  );
  // Go intentionally omits -ldflags from build info when -trimpath is used.
  for (const [key, value] of Object.entries({
    "-trimpath": "true",
    CGO_ENABLED: "0",
    GOOS: os,
    GOARCH: architecture,
    "vcs.revision": sha,
    "vcs.modified": "false",
  })) {
    if (settings.get(key) !== value)
      fail(
        `Unexpected build setting ${key}: expected ${value}.`,
        `构建设置 ${key} 不正确，应为 ${value}。`,
      );
  }
}
