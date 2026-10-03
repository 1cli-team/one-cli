import assert from "node:assert/strict";
import test from "node:test";
import {
  assets,
  assertAssets,
  bumpTag,
  compareTags,
  decidePlan,
  parseChecksums,
} from "./release-rules.mts";

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
