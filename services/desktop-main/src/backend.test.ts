import assert from "node:assert/strict";
import { test } from "node:test";
import { mkdtemp, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { setTimeout as delay } from "node:timers/promises";
import { createBackend, type Manifest } from "./backend.ts";

const manifest: Manifest = {
  version: "1.2.3",
  commit: "test",
  channel: "development",
  protocol: 2,
  api: 1,
  target: "test",
  sha256: "",
  cliSha256: "",
};

async function until(predicate: () => boolean) {
  for (let attempt = 0; attempt < 300; attempt++) {
    if (predicate()) return;
    await delay(10);
  }
  assert.fail("backend state did not settle");
}

test("host supervises handshake, quiesce, crash, retry and graceful shutdown", async () => {
  const directory = await mkdtemp(join(tmpdir(), "one-backend-test-"));
  const executable = join(directory, "child.cjs");
  await writeFile(
    executable,
    `const readline = require('node:readline');
let boot;
readline.createInterface({input: process.stdin}).on('line', line => {
 const command = JSON.parse(line);
 if (command.type === 'bootstrap') {
  boot = command;
  process.stdout.write(JSON.stringify({type:'ready',generation:boot.generation,port:12345,storageRoot:${JSON.stringify(directory)},version:'1.2.3',commit:'test',channel:'development',protocol:2,api:1})+'\\n');
 } else if (command.type === 'quiesce') process.stdout.write('{"type":"quiesced","active":2}\\n');
 else if (command.type === 'resume') process.exit(1);
 else if (command.type === 'shutdown') process.exit(0);
}).on('close', () => process.exit(0));
`,
    { mode: 0o755 },
  );
  const backend = createBackend({
    executable: process.execPath,
    executableArgs: [executable],
    manifest,
    origin: "one-desktop://app",
    verifyBinary: false,
  });
  let events = 0;
  const unsubscribe = backend.onState(() => {
    events++;
  });
  try {
    await backend.start();
    await until(() => backend.connection().status === "httpReady");
    const first = backend.connection();
    assert.equal(first.token?.length, 64);
    assert.equal(await backend.quiesce(), 2);
    await assert.rejects(backend.confirmHealth("stale"), /backend_session_changed/);
    await backend.confirmHealth(first.generation);
    backend.resume();
    await until(() => backend.connection().status === "failed");
    assert.equal(backend.connection().token, undefined);
    await backend.retry();
    await until(() => backend.connection().status === "httpReady");
    assert.notEqual(backend.connection().generation, first.generation);
    assert.notEqual(backend.connection().token, first.token);
    await backend.stop();
    assert.equal(backend.connection().status, "stopped");
    assert.ok(events >= 5);
  } finally {
    unsubscribe();
    await backend.stop();
    await rm(directory, { recursive: true, force: true });
  }
});

test("a wrong bundled binary fails closed", async () => {
  const backend = createBackend({
    executable: process.execPath,
    manifest,
    origin: "one-desktop://app",
  });
  await backend.start();
  assert.equal(backend.connection().status, "failed");
  assert.equal(backend.connection().error, "backend_identity");
  assert.equal(backend.connection().token, undefined);
});
