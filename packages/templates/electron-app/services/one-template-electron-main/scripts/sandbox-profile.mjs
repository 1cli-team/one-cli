import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

// Generate an application-specific user-namespace permission for Ubuntu.
// This command only prints a profile; installing it requires an administrator.
export function sandboxProfileName(binary) {
  return `one-electron-${createHash("sha256").update(binary).digest("hex").slice(0, 12)}`;
}

export function sandboxProfile(binary) {
  const name = sandboxProfileName(binary);
  const quoted = `"${binary.replace(/[\\"*?[\]{}\n\r]/g, (char) => `\\${char}`)}"`;
  return `abi <abi/4.0>,
include <tunables/global>

profile ${name} ${quoted} flags=(unconfined) {
  userns,
  ${quoted} mr,
}
`;
}

if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  const require = createRequire(import.meta.url);
  process.stdout.write(sandboxProfile(require("electron")));
}
