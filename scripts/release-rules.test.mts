import assert from "node:assert/strict";
import test from "node:test";
import {
  assets,
  assertAssets,
  assertDraftAssets,
  assertReleaseAssets,
  assertReleaseIdentity,
  assertUnchangedAssets,
  assertBuildSettings,
  bumpTag,
  compareTags,
  decidePlan,
  parseChecksums,
  releaseAssets,
  isUnifiedRelease,
} from "./release-rules.mts";
import { desktopAssets } from "./release-assets.mts";

const tag = { name: "v0.1.16", object: "tag-object", sha: "previous" };
const release = {
  tag_name: tag.name,
  draft: false,
  prerelease: false,
  html_url: "https://github.com/1cli-team/one-cli/releases/tag/v0.1.16",
  assets: assets.map((name) => ({ name, state: "uploaded" })),
};
const input = {
  head: "current",
  master: "current",
  tags: [tag],
  releases: [release],
  bump: "patch" as const,
  highestOnMaster: true,
};

test("semantic versions sort numerically and reset lower components", () => {
  assert.ok(compareTags("v0.1.16", "v0.1.9") > 0);
  assert.equal(bumpTag("v0.1.16", "patch"), "v0.1.17");
  assert.equal(bumpTag("v0.1.16", "minor"), "v0.2.0");
  assert.equal(bumpTag("v0.1.16", "major"), "v1.0.0");
});
test("first release starts at v0.0.1 and nonstable tags are ignored", () => {
  assert.equal(decidePlan({ ...input, tags: [], releases: [] }).tag, "v0.0.1");
  assert.equal(
    decidePlan({ ...input, tags: [tag, { ...tag, name: "v9.0.0-rc.1" }] }).tag,
    "v0.1.17",
  );
});
test("new release uses master and completed release on the same commit is idempotent", () => {
  assert.equal(decidePlan(input).sourceSha, "current");
  assert.equal(decidePlan({ ...input, head: "previous", master: "previous" }).mode, "idempotent");
});
test("interrupted releases resume the tagged commit even after master advances", () => {
  for (const releases of [[], [{ ...release, draft: true }]]) {
    const plan = decidePlan({ ...input, releases, bump: "major" });
    assert.equal(plan.mode, "resume");
    assert.equal(plan.tag, "v0.1.16");
    assert.equal(plan.sourceSha, "previous");
  }
});
test("off-master orphan tags stop while off-master drafts reserve their version", () => {
  assert.throws(() => decidePlan({ ...input, releases: [], highestOnMaster: false }), /off master/);
  assert.equal(
    decidePlan({ ...input, releases: [{ ...release, draft: true }], highestOnMaster: false }).tag,
    "v0.1.17",
  );
});
test("changed master, malformed published assets, and stable prereleases stop", () => {
  assert.throws(() => decidePlan({ ...input, master: "advanced" }), /HEAD differs/);
  assert.throws(
    () => decidePlan({ ...input, releases: [{ ...release, assets: [] }] }),
    /Unexpected release assets/,
  );
  assert.throws(
    () => decidePlan({ ...input, releases: [{ ...release, prerelease: true }] }),
    /prerelease/,
  );
});
test("duplicate stable tags and releases without matching tags stop", () => {
  assert.throws(
    () =>
      decidePlan({
        ...input,
        tags: [{ ...tag, sha: "current" }],
        releases: [{ ...release, draft: true }],
        highestOnMaster: false,
      }),
    /already has a stable tag/,
  );
  assert.throws(
    () => decidePlan({ ...input, releases: [release, { ...release, tag_name: "v0.1.17" }] }),
    /without its Git tag/,
  );
});
test("asset set and checksum manifest reject duplicates, extras, traversal and omissions", () => {
  assertAssets([...assets].reverse());
  assert.throws(() => assertAssets([...assets, assets[0]]), /Unexpected/);
  const checksums = assets
    .slice(1)
    .map((name) => `${"a".repeat(64)}  ${name}`)
    .join("\n");
  assert.equal(parseChecksums(checksums).size, 5);
  assert.throws(() => parseChecksums(`${checksums}\n${"a".repeat(64)}  ../one`), /Invalid/);
  assert.throws(() => parseChecksums(`${checksums}\n${checksums.split("\n")[0]}`), /duplicate/);
  assert.throws(() => parseChecksums(checksums.split("\n").slice(1).join("\n")), /Unexpected/);
});

test("trimpath builds verify source/platform metadata without expecting linker flags", () => {
  const info =
    "\tbuild\t-trimpath=true\n\tbuild\tCGO_ENABLED=0\n\tbuild\tGOOS=darwin\n\tbuild\tGOARCH=arm64\n\tbuild\tvcs.revision=source-sha\n\tbuild\tvcs.modified=false\n";
  assertBuildSettings(info, "source-sha", "darwin", "arm64");
  assert.throws(() => assertBuildSettings(info, "other-sha", "darwin", "arm64"), /vcs.revision/);
  assert.throws(() => assertBuildSettings(info, "source-sha", "linux", "arm64"), /GOOS/);
  assert.throws(
    () =>
      assertBuildSettings(
        info.replace("CGO_ENABLED=0", "CGO_ENABLED=1"),
        "source-sha",
        "darwin",
        "arm64",
      ),
    /CGO_ENABLED/,
  );
});

test("desktop checksum allowlist stays separate from the CLI release protocol", () => {
  const desktop = [
    "desktop-release.json",
    "one-desktop_0.1.20_darwin_arm64.dmg",
    "one-desktop_0.1.20_darwin_amd64.dmg",
    "one-desktop_0.1.20_windows_amd64.exe",
    "one-desktop_0.1.20_linux_amd64.AppImage",
    "one-desktop_0.1.20_linux_arm64.AppImage",
  ];
  const lines = desktop.map((name) => `${"b".repeat(64)}  ${name}`);
  const text = lines.join("\n");
  assert.equal(parseChecksums(text, desktop).size, 6);
  assert.throws(() => parseChecksums(text), /Unexpected/);
  assert.throws(() => parseChecksums(lines.slice(1).join("\n"), desktop), /Unexpected/);
  assert.throws(
    () => parseChecksums(`${text}\n${"a".repeat(64)}  one-cli_darwin_arm64.tar.gz`, desktop),
    /Unexpected/,
  );
  assert.throws(() => parseChecksums(`${text}\n${lines[0]}`, desktop), /duplicate/);
  assert.throws(() => parseChecksums(`${text}\n${"a".repeat(64)}  ../one`, desktop), /Invalid/);
});

test("v0.1.22 is the fixed boundary; historical release assets remain unchanged", () => {
  assert.equal(isUnifiedRelease("v0.1.21"), false);
  assert.equal(isUnifiedRelease("desktop-v0.1.22"), false);
  assert.equal(isUnifiedRelease("v0.1.22-rc.1"), false);
  assert.equal(isUnifiedRelease("v0.1.22"), true);
  assert.equal(isUnifiedRelease("v0.2.0"), true);
  assert.deepEqual(releaseAssets("v0.1.21"), assets);
  assertReleaseAssets("v0.1.21", assets);
  assert.throws(() => assertReleaseAssets("v0.1.21", releaseAssets("v0.1.22")), /Unexpected/);
});

test("a unified published release requires all 13 version-matched attachments", () => {
  const names = releaseAssets("v0.1.22");
  assert.equal(names.length, 13);
  assertReleaseAssets("v0.1.22", [...names].reverse());
  for (const missing of names)
    assert.throws(
      () =>
        assertReleaseAssets(
          "v0.1.22",
          names.filter((n) => n !== missing),
        ),
      /Unexpected/,
    );
  assert.throws(() => assertReleaseAssets("v0.1.22", [...names, names[0]]), /Unexpected/);
  assert.throws(() => assertReleaseAssets("v0.1.22", [...names, "unexpected.txt"]), /Unexpected/);
  assert.throws(
    () => assertReleaseAssets("v0.1.22", [...assets, ...desktopAssets("0.1.23")]),
    /Unexpected/,
  );
});

test("v0.1.23 adds both Linux DEBs without changing the historical 13-asset contract", () => {
  assert.equal(releaseAssets("v0.1.22").length, 13);
  for (const version of ["0.1.23", "0.1.100", "0.2.0", "1.0.0"]) {
    const names = releaseAssets(`v${version}`);
    assert.equal(names.length, 15);
    for (const arch of ["amd64", "arm64"]) {
      const deb = `one-desktop_${version}_linux_${arch}.deb`;
      assert.ok(names.includes(deb));
      assert.throws(
        () =>
          assertReleaseAssets(
            `v${version}`,
            names.filter((name) => name !== deb),
          ),
        /Unexpected/,
      );
    }
    assertReleaseAssets(`v${version}`, names);
    assertDraftAssets(`v${version}`, names, true);
  }
  assert.throws(
    () =>
      assertReleaseAssets("v0.1.22", [
        ...releaseAssets("v0.1.22"),
        "one-desktop_0.1.22_linux_amd64.deb",
      ]),
    /Unexpected/,
  );
});

test("drafts permit partial desktop uploads but require the complete CLI before dispatch", () => {
  assertDraftAssets("v0.1.22", []);
  assertDraftAssets("v0.1.22", assets, true);
  assertDraftAssets("v0.1.22", [...assets, desktopAssets("0.1.22")[0]], true);
  assert.throws(() => assertDraftAssets("v0.1.22", assets.slice(1), true), /Incomplete/);
  assert.throws(() => assertDraftAssets("v0.1.22", [...assets, assets[0]]), /unexpected/);
  assert.throws(() => assertDraftAssets("v0.1.22", [...assets, "../file"]), /unexpected/);
  assert.throws(
    () => assertDraftAssets("v0.1.22", [...assets, ...desktopAssets("0.1.21")]),
    /unexpected/,
  );
});

test("unified drafts resume the same commit and incomplete public releases cannot bump", () => {
  const unifiedTag = { ...tag, name: "v0.1.22" };
  const unified = {
    ...release,
    tag_name: unifiedTag.name,
    assets: releaseAssets(unifiedTag.name).map((name) => ({ name, state: "uploaded" })),
  };
  const pending = {
    ...input,
    tags: [unifiedTag],
    releases: [{ ...unified, draft: true, assets: release.assets }],
  };
  const p = decidePlan(pending);
  assert.equal(p.mode, "resume");
  assert.equal(p.tag, "v0.1.22");
  assert.equal(p.sourceSha, "previous");
  assert.throws(
    () => decidePlan({ ...pending, releases: [{ ...pending.releases[0], draft: false }] }),
    /Unexpected/,
  );
  assert.equal(
    decidePlan({ ...pending, releases: [unified], head: "previous", master: "previous" }).mode,
    "idempotent",
  );
  assert.equal(decidePlan({ ...pending, releases: [unified] }).tag, "v0.1.23");
});

test("publisher rejects moved tags, changed source, recreated drafts and missing plan fields", () => {
  const identity = { commit: "a".repeat(40), tagObject: "b".repeat(40), releaseID: "123" };
  assertReleaseIdentity(identity, { ...identity });
  for (const field of ["commit", "tagObject", "releaseID"] as const) {
    assert.throws(
      () => assertReleaseIdentity({ ...identity, [field]: "changed" }, identity),
      /frozen/,
    );
    assert.throws(() => assertReleaseIdentity(identity, { ...identity, [field]: "" }), /frozen/);
  }
});

test("final publication rejects attachments changed or removed after download verification", () => {
  const verified = releaseAssets("v0.1.22").map((name, id) => ({
    id,
    name,
    size: 100,
    digest: `sha256:${"a".repeat(64)}`,
    state: "uploaded",
  }));
  assertUnchangedAssets([...verified].reverse(), verified);
  assert.throws(() => assertUnchangedAssets(verified.slice(1), verified), /changed/);
  for (const update of [{ id: 99 }, { size: 101 }, { digest: "other" }, { state: "starter" }])
    assert.throws(
      () => assertUnchangedAssets([{ ...verified[0], ...update }, ...verified.slice(1)], verified),
      /changed/,
    );
});
