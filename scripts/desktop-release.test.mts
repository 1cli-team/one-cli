import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import {
  chmodSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import nodeTest from "node:test";
import { assets, releaseAssets } from "./release-rules.mts";
import { desktopInstallerNames, platforms } from "./release-assets.mts";
import { collectInstallers } from "./desktop-release.mts";

const publisher = fileURLToPath(new URL("./desktop-release.mts", import.meta.url));
// The local publisher and these executable git/gh stubs target Linux/macOS.
const test = process.platform === "win32" ? nodeTest.skip : nodeTest;
const sha = "a".repeat(40);
const hash = (file: string) => createHash("sha256").update(readFileSync(file)).digest("hex");

// Run the real publisher offline. Only its external git/gh boundary is replaced;
// local artifact collection, checksums, identity checks and publish ordering run normally.
function fixture(version = "0.1.23") {
  const root = mkdtempSync(join(tmpdir(), "one-unified-release-test-"));
  const bin = join(root, "bin");
  const remote = join(root, "remote");
  const installers = join(root, "installers");
  for (const path of [bin, remote, installers]) mkdirSync(path);
  const statePath = join(root, "state.json");
  for (const name of assets.slice(1)) writeFileSync(join(remote, name), `CLI ${name}`);
  writeFileSync(
    join(remote, "checksums.txt"),
    assets
      .slice(1)
      .map((name) => `${hash(join(remote, name))}  ${name}\n`)
      .join(""),
  );
  const state = {
    id: 123,
    tag_name: `v${version}`,
    draft: true,
    prerelease: false,
    html_url: `https://example.invalid/releases/v${version}`,
    commit: sha,
    tagObject: "b".repeat(40),
    assets: assets.map((name, i) => ({
      id: i + 1,
      name,
      size: readFileSync(join(remote, name)).length,
      state: "uploaded",
      digest: `sha256:${hash(join(remote, name))}`,
    })),
  };
  writeFileSync(statePath, JSON.stringify(state));
  for (const target of Object.keys(platforms)) {
    const dir = join(installers, target);
    mkdirSync(dir);
    const artifacts = desktopInstallerNames(version, target).map((name) => {
      writeFileSync(join(dir, name), `Installer ${target} ${name}`);
      return {
        name,
        target,
        sha256: hash(join(dir, name)),
        notarized: platforms[target].os === "darwin",
      };
    });
    writeFileSync(
      join(dir, `${target}.json`),
      JSON.stringify({
        version,
        commit: sha,
        ...(version === "0.1.22" ? { artifact: artifacts[0] } : { artifacts }),
      }),
    );
  }
  const mock = `#!${process.execPath}
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const root = process.env.ONE_RELEASE_TEST_ROOT;
const file = path.join(root, 'state.json');
const state = JSON.parse(fs.readFileSync(file, 'utf8'));
const args = process.argv.slice(2);
const tool = path.basename(process.argv[1]);
fs.appendFileSync(path.join(root, 'calls.jsonl'), JSON.stringify({tool, args})+'\\n');
const done = (value) => { if(value !== undefined) process.stdout.write(typeof value === 'string' ? value : JSON.stringify(value)); process.exit(0); };
const option = (key) => args[args.indexOf(key)+1];
if(tool === 'git') {
  if(args[0] === 'rev-parse') done(state.commit);
  if(args[0] === 'merge-base' || args[0] === 'cat-file') done('');
  if(args[0] === 'ls-remote') done(state.tagObject+'\\trefs/tags/'+state.tag_name);
}
if(tool === 'gh' && args[0] === 'api') {
  const route = args[1].replace('repos/1cli-team/one-cli/', '');
  if(route === 'releases/tags/'+state.tag_name) {
    if(state.draft) { process.stderr.write('(HTTP 404)'); process.exit(1); }
    done(state);
  }
  if(route.startsWith('releases?')) done([state]);
  if(route === 'git/ref/tags/'+state.tag_name) done({object:{type:'tag', sha:state.tagObject}});
  if(route === 'git/tags/'+state.tagObject) done({object:{type:'commit', sha:state.commit}});
  if(route === 'releases/latest') done({tag_name: state.draft ? 'v0.1.21' : state.tag_name});
}
if(tool === 'gh' && args[0] === 'release') {
  if(args[1] === 'download') {
    const dest = option('--dir'); fs.mkdirSync(dest, {recursive:true});
    const names = args.flatMap((arg, i) => arg === '--pattern' ? [args[i+1]] : []);
    for(const name of names) fs.copyFileSync(path.join(root, 'remote', name), path.join(dest, name));
    done('');
  }
  if(args[1] === 'upload') {
    if(!state.draft) throw Error('attempt to mutate a public release');
    for(const source of args.slice(args.indexOf('--clobber')+1)) {
      const name = path.basename(source); const data = fs.readFileSync(source);
      fs.copyFileSync(source, path.join(root,'remote',name));
      state.assets = state.assets.filter(a=>a.name!==name);
      state.assets.push({id:100+state.assets.length,name,size:data.length,state:'uploaded',digest:'sha256:'+crypto.createHash('sha256').update(data).digest('hex')});
    }
    fs.writeFileSync(file, JSON.stringify(state)); done('');
  }
  if(args[1] === 'edit' && args.includes('--draft=false') && args.includes('--latest')) {
    state.draft = false; fs.writeFileSync(file, JSON.stringify(state)); done('');
  }
}
throw Error('unhandled external command: '+tool+' '+args.join(' '));
`;
  for (const tool of ["git", "gh"]) {
    writeFileSync(join(bin, tool), mock);
    chmodSync(join(bin, tool), 0o755);
  }
  const env = {
    ...process.env,
    PATH: `${bin}:${process.env.PATH}`,
    ONE_RELEASE_TEST_ROOT: root,
    GITHUB_OUTPUT: "",
  };
  const run = (action: string, extra: NodeJS.ProcessEnv = {}, tag = `v${version}`) =>
    spawnSync(process.execPath, [publisher, action, tag, installers], {
      env: { ...env, ...extra },
      encoding: "utf8",
    });
  const readState = () => JSON.parse(readFileSync(statePath, "utf8")) as typeof state;
  const calls = () =>
    existsSync(join(root, "calls.jsonl"))
      ? readFileSync(join(root, "calls.jsonl"), "utf8")
          .trim()
          .split("\n")
          .map((s) => JSON.parse(s) as { tool: string; args: string[] })
      : [];
  const mutations = () =>
    calls().filter((c) => c.tool === "gh" && c.args[0] === "release" && c.args[1] !== "download");
  const plan = () => {
    const result = run("plan");
    assert.equal(result.status, 0, result.stderr);
    const p = JSON.parse(result.stdout);
    return {
      ONE_RELEASE_SOURCE: p.source,
      ONE_RELEASE_TAG_OBJECT: p.tag_object,
      ONE_RELEASE_ID: p.release_id,
      ONE_RELEASE_CLI_CHECKSUMS_SHA256: p.cli_checksums_sha256,
    };
  };
  return {
    root,
    remote,
    installers,
    statePath,
    run,
    plan,
    readState,
    mutations,
    close: () => rmSync(root, { recursive: true, force: true }),
  };
}

test("offline unified publisher stages all 15 assets once and public retries never write", () => {
  const f = fixture();
  try {
    const cli = Object.fromEntries(assets.map((name) => [name, hash(join(f.remote, name))]));
    const pinned = f.plan();
    assert.equal(f.mutations().length, 0);
    const result = f.run("publish", pinned);
    assert.equal(result.status, 0, result.stderr);
    assert.equal(f.readState().draft, false);
    assert.deepEqual(readdirSync(f.remote).sort(), releaseAssets("v0.1.23").sort());
    assert.deepEqual(
      Object.fromEntries(assets.map((name) => [name, hash(join(f.remote, name))])),
      cli,
    );
    const writes = f.mutations();
    assert.deepEqual(
      writes.map((c) => c.args[1]),
      ["upload", "edit"],
    );
    assert.ok(
      writes[0].args
        .filter((arg) => arg.startsWith("/"))
        .every((arg) => !assets.includes(arg.split("/").at(-1)!)),
    );
    const retry = f.run("publish", pinned);
    assert.equal(retry.status, 0, retry.stderr);
    assert.match(retry.stdout, /Already published/);
    assert.deepEqual(f.mutations(), writes);
  } finally {
    f.close();
  }
});

test("v0.1.22 legacy build records still publish and public retries never add DEBs", () => {
  const f = fixture("0.1.22");
  try {
    const pinned = f.plan();
    const result = f.run("publish", pinned);
    assert.equal(result.status, 0, result.stderr);
    assert.deepEqual(readdirSync(f.remote).sort(), releaseAssets("v0.1.22").sort());
    const writes = f.mutations();
    const retry = f.run("publish", pinned);
    assert.equal(retry.status, 0, retry.stderr);
    assert.deepEqual(f.mutations(), writes);
  } finally {
    f.close();
  }
});

test("offline publisher refuses missing or corrupted DEBs before uploading", () => {
  for (const change of ["missing", "corrupt"]) {
    const f = fixture();
    try {
      const pinned = f.plan();
      const deb = join(
        f.installers,
        "x86_64-unknown-linux-gnu",
        "one-desktop_0.1.23_linux_amd64.deb",
      );
      if (change === "missing") rmSync(deb);
      else writeFileSync(deb, "corrupted DEB");
      const result = f.run("publish", pinned);
      assert.notEqual(result.status, 0);
      assert.equal(f.readState().draft, true);
      assert.equal(f.mutations().length, 0);
    } finally {
      f.close();
    }
  }
});

test("Linux installer collection retains both formats and validates each format before replacing output", () => {
  for (const target of ["x86_64-unknown-linux-gnu", "aarch64-unknown-linux-gnu"]) {
    const root = mkdtempSync(join(tmpdir(), "one-installer-collection-test-"));
    try {
      const bundle = join(root, "apps/desktop/src-tauri/target", target, "release/bundle");
      for (const [format, name] of [
        ["appimage", "One Desktop.AppImage"],
        ["deb", "one-desktop.deb"],
      ]) {
        mkdirSync(join(bundle, format), { recursive: true });
        writeFileSync(join(bundle, format, name), `${target} ${format}`);
      }
      const manifest = {
        version: "0.1.23",
        commit: sha,
        channel: "release",
        target,
        sha256: "",
        cliSha256: "",
      };
      collectInstallers(root, manifest);
      const out = join(root, "desktop-dist", target);
      assert.deepEqual(
        readdirSync(out).sort(),
        [...desktopInstallerNames("0.1.23", target), `${target}.json`].sort(),
      );
      const record = JSON.parse(readFileSync(join(out, `${target}.json`), "utf8"));
      assert.equal(record.artifacts.length, 2);
      for (const item of record.artifacts) assert.equal(item.sha256, hash(join(out, item.name)));
      writeFileSync(join(out, "preserve-on-failure"), "previous output");
      rmSync(join(bundle, "deb", "one-desktop.deb"));
      assert.throws(() => collectInstallers(root, manifest), /one installer per format/);
      assert.equal(readFileSync(join(out, "preserve-on-failure"), "utf8"), "previous output");
      for (const name of ["one.deb", "two.deb"])
        writeFileSync(join(bundle, "deb", name), "ambiguous");
      assert.throws(() => collectInstallers(root, manifest), /one installer per format/);
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  }
});

test("offline publisher refuses historical tags without contacting GitHub", () => {
  const f = fixture();
  try {
    const result = f.run("publish", {}, "v0.1.21");
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /historical releases/);
    assert.equal(existsSync(join(f.root, "calls.jsonl")), false);
  } finally {
    f.close();
  }
});

test("offline publisher refuses incomplete installers before uploading", () => {
  const f = fixture();
  try {
    const pinned = f.plan();
    rmSync(join(f.installers, "aarch64-apple-darwin"), { recursive: true });
    const result = f.run("publish", pinned);
    assert.notEqual(result.status, 0);
    assert.equal(f.readState().draft, true);
    assert.equal(f.mutations().length, 0);
  } finally {
    f.close();
  }
});

test("offline publisher refuses a recreated draft and changed CLI checksums", () => {
  for (const change of ["draft", "checksum"]) {
    const f = fixture();
    try {
      const pinned = f.plan();
      if (change === "draft")
        writeFileSync(f.statePath, JSON.stringify({ ...f.readState(), id: 456 }));
      else
        writeFileSync(
          join(f.remote, "checksums.txt"),
          readFileSync(join(f.remote, "checksums.txt"), "utf8") + "\n",
        );
      const result = f.run("publish", pinned);
      assert.notEqual(result.status, 0);
      assert.equal(f.readState().draft, true);
      assert.equal(f.mutations().length, 0);
    } finally {
      f.close();
    }
  }
});

test("offline publisher leaves the Release in draft when downloaded CLI bytes fail verification", () => {
  const f = fixture();
  try {
    const pinned = f.plan();
    writeFileSync(join(f.remote, assets[1]), "corrupt CLI archive");
    const result = f.run("publish", pinned);
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /CLI checksum mismatch/);
    assert.equal(f.readState().draft, true);
    assert.ok(f.mutations().every((c) => c.args[1] === "upload"));
  } finally {
    f.close();
  }
});
