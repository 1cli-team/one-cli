import assert from "node:assert/strict";
import { test } from "node:test";
import { DatabaseSync } from "node:sqlite";
import { mkdtemp, rm, mkdir, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { isLegacyOrigin, readLegacyWebKit, validatePreferences } from "./preferences.ts";

test("legacy migration reads display preferences without changing the original database", async () => {
  const root = await mkdtemp(join(tmpdir(), "one-preferences-test-"));
  const file = join(root, "tauri_localhost_0.localstorage");
  const db = new DatabaseSync(file);
  try {
    db.exec("CREATE TABLE ItemTable (key TEXT UNIQUE, value BLOB NOT NULL)");
    const insert = db.prepare("INSERT INTO ItemTable VALUES (?, ?)");
    insert.run("app_locale_mode", Buffer.from("zh-CN", "utf16le"));
    insert.run("app_theme_mode", Buffer.from("dark", "utf16le"));
    insert.run("one_desktop_setup_seen", Buffer.from("1", "utf16le"));
    insert.run("unrelated", Buffer.from("do not import", "utf16le"));
    assert.deepEqual(await readLegacyWebKit([root]), {
      app_locale_mode: "zh-CN",
      app_theme_mode: "dark",
      one_desktop_setup_seen: "1",
    });
    assert.equal(db.prepare("SELECT count(*) AS n FROM ItemTable").get()?.n, 4);
    assert.deepEqual(
      validatePreferences({ app_locale_mode: "bad", app_theme_mode: "script", token: "secret" }),
      {},
    );
  } finally {
    db.close();
    await rm(root, { recursive: true, force: true });
  }
});

test("modern WebKit imports only the application's two matching origins", async () => {
  const encode = (value: string) => {
    const length = Buffer.alloc(4);
    length.writeUInt32LE(value.length);
    return Buffer.concat([length, Buffer.from([1]), Buffer.from(value)]);
  };
  const origin = Buffer.concat([encode("tauri"), encode("localhost"), Buffer.from([0])]);
  const root = await mkdtemp(join(tmpdir(), "one-modern-webkit-"));
  const directory = join(root, "hashed-top", "hashed-frame", "LocalStorage");
  await mkdir(directory, { recursive: true });
  const db = new DatabaseSync(join(directory, "localstorage.sqlite3"));
  try {
    db.exec("CREATE TABLE ItemTable (key TEXT UNIQUE, value BLOB NOT NULL)");
    db.prepare("INSERT INTO ItemTable VALUES (?, ?)").run(
      "app_theme_mode",
      Buffer.from("dark", "utf16le"),
    );
    const path = join(directory, "..", "origin");
    await writeFile(path, Buffer.concat([origin, origin]));
    assert.equal(isLegacyOrigin(Buffer.concat([origin, origin])), true);
    assert.deepEqual(await readLegacyWebKit([root]), { app_theme_mode: "dark" });
    await writeFile(
      path,
      Buffer.concat([origin, encode("https"), encode("localhost"), Buffer.from([0])]),
    );
    assert.deepEqual(await readLegacyWebKit([root]), {});
    assert.equal(isLegacyOrigin(origin.subarray(0, 7)), false);
  } finally {
    db.close();
    await rm(root, { recursive: true, force: true });
  }
});
