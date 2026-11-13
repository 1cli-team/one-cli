import { createHash } from "node:crypto";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

export function sha256File(path: string) {
  return createHash("sha256").update(readFileSync(path)).digest("hex");
}

// The type describes the caller's expected schema; validation stays with the
// workflow that knows the file's protocol and required fields.
export function readJson<T>(path: string): T {
  return JSON.parse(readFileSync(path, "utf8")) as T;
}

export function writeJson(path: string, value: unknown, options: { mode?: number } = {}) {
  writeFileSync(path, JSON.stringify(value, null, 2) + "\n", options);
}

// Only for synchronous, disposable work. Release recovery directories are
// intentionally retained by their callers and must not use this helper.
export function withTempDirectory<T>(prefix: string, action: (directory: string) => T): T {
  const directory = mkdtempSync(join(tmpdir(), prefix));
  try {
    return action(directory);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
}
