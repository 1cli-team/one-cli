import { mkdtemp, cp, readdir, rm, stat, readFile } from "node:fs/promises";
import { homedir, tmpdir } from "node:os";
import { join, dirname } from "node:path";
import { DatabaseSync } from "node:sqlite";

export const preferenceKeys = [
  "app_locale_mode",
  "app_theme_mode",
  "one_desktop_setup_seen",
] as const;
export type DisplayPreferences = Partial<Record<(typeof preferenceKeys)[number], string>>;

export function validatePreferences(values: Record<string, unknown>): DisplayPreferences {
  const result: DisplayPreferences = {};
  for (const key of preferenceKeys) {
    const allowed =
      key === "app_locale_mode"
        ? ["auto", "zh-CN", "en-US"]
        : key === "app_theme_mode"
          ? ["light", "dark"]
          : ["1"];
    if (typeof values[key] === "string" && allowed.includes(values[key])) result[key] = values[key];
  }
  return result;
}

export function legacyRoots(platform = process.platform, env = process.env, home = homedir()) {
  if (platform === "darwin")
    return [
      join(home, "Library/WebKit/team.1cli.desktop"),
      join(home, "Library/Application Support/team.1cli.desktop"),
      join(home, "Library/Containers/team.1cli.desktop/Data/Library/WebKit"),
    ];
  if (platform === "win32")
    return [join(env.LOCALAPPDATA || join(home, "AppData/Local"), "team.1cli.desktop")];
  return [join(env.XDG_DATA_HOME || join(home, ".local/share"), "team.1cli.desktop")];
}

// WebKit's current WebsiteData layout uses hashed directories and a binary
// ClientOrigin (two SecurityOriginData records). Parse only the exact local
// application origin, following WebKit's persistent string/optional encoding.
export function isLegacyOrigin(data: Uint8Array): boolean {
  const bytes = Buffer.from(data);
  let offset = 0;
  function byte() {
    if (offset >= bytes.length) throw new Error("truncated origin");
    return bytes[offset++];
  }
  function string() {
    if (offset + 4 > bytes.length) throw new Error("truncated string");
    const length = bytes.readUInt32LE(offset);
    offset += 4;
    const encoding = byte();
    if (length > 128 || encoding > 1) throw new Error("invalid string");
    const size = length * (encoding === 1 ? 1 : 2);
    if (offset + size > bytes.length) throw new Error("truncated string");
    const value = bytes.toString(encoding === 1 ? "latin1" : "utf16le", offset, offset + size);
    offset += size;
    return value;
  }
  try {
    for (let index = 0; index < 2; index++) {
      if (string() !== "tauri" || string() !== "localhost") return false;
      const hasPort = byte();
      if (hasPort === 1) {
        if (offset + 2 > bytes.length || bytes.readUInt16LE(offset) !== 0) return false;
        offset += 2;
      } else if (hasPort !== 0) return false;
    }
    return offset === bytes.length;
  } catch {
    return false;
  }
}

// Read only the three display preferences from a copy of the old WebKit store.
// Never open the user's live database for writes or import unrelated storage.
export async function readLegacyWebKit(roots = legacyRoots()): Promise<DisplayPreferences> {
  const temporary = await mkdtemp(join(tmpdir(), "one-legacy-preferences-"));
  const values: Record<string, unknown> = {};
  let visited = 0;
  async function walk(directory: string, depth: number) {
    if (depth > 8 || visited > 2000) return;
    let entries;
    try {
      entries = await readdir(directory, { withFileTypes: true });
    } catch {
      return;
    }
    for (const entry of entries) {
      if (++visited > 2000) return;
      const file = join(directory, entry.name);
      if (entry.isDirectory()) {
        await walk(file, depth + 1);
        continue;
      }
      if (
        !entry.isFile() ||
        (!entry.name.endsWith(".localstorage") && entry.name !== "localstorage.sqlite3")
      )
        continue;
      // WebKit stores origins in filenames or parent directories. Only import
      // the old application origin, never other origins in a shared WebKit root.
      if (entry.name === "localstorage.sqlite3") {
        try {
          const originPath = join(dirname(directory), "origin");
          if ((await stat(originPath)).size > 1024 || !isLegacyOrigin(await readFile(originPath)))
            continue;
        } catch {
          continue;
        }
      } else if (!/(?:^|[/\\])tauri_localhost_0(?:[/\\.]|$)/.test(file)) continue;
      try {
        if ((await stat(file)).size > 8 * 1024 * 1024) continue;
        const copy = join(temporary, `storage-${visited}.sqlite`);
        await cp(file, copy);
        try {
          if ((await stat(file + "-wal")).size > 8 * 1024 * 1024) continue;
          await cp(file + "-wal", copy + "-wal");
        } catch {
          /* No WAL. */
        }
        const db = new DatabaseSync(copy, { readOnly: true });
        try {
          const rows = db
            .prepare("SELECT key, value FROM ItemTable WHERE key IN (?, ?, ?)")
            .all(...preferenceKeys);
          for (const row of rows) {
            const raw = row.value;
            const value =
              typeof raw === "string"
                ? raw
                : raw instanceof Uint8Array
                  ? Buffer.from(raw).toString("utf16le")
                  : undefined;
            if (value !== undefined)
              Object.assign(values, validatePreferences({ [String(row.key)]: value }));
          }
        } finally {
          db.close();
        }
      } catch {
        /* An unreadable old cache must not prevent startup. */
      }
    }
  }
  try {
    for (const root of roots) await walk(root, 0);
    return validatePreferences(values);
  } finally {
    await rm(temporary, { recursive: true, force: true });
  }
}
