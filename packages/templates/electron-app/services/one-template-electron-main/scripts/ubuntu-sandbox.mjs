import { spawnSync } from "node:child_process";
import * as fs from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { messages } from "./dev-messages.mjs";
import { sandboxProfile, sandboxProfileName } from "./sandbox-profile.mjs";

export function isUbuntu(release) {
  // Use ID, never ID_LIKE or a version guess. Do not evaluate shell content.
  return /^ID=(?:ubuntu|"ubuntu"|'ubuntu')\s*$/m.test(release);
}

export function shellQuote(value) {
  return `'${value.replaceAll("'", "'\\''")}'`;
}

// The probe runs under the actual Electron binary's AppArmor profile. Probing
// unshare directly would test Node's permissions instead of Electron's.
export function probeUserNamespace(binary, env, run = spawnSync) {
  const source = `const { spawnSync } = require('node:child_process');
const result = spawnSync('/usr/bin/unshare', ['--user', '--map-root-user', '--mount', '/usr/bin/true'], { encoding: 'utf8', timeout: 3000, env: { ...process.env, LC_ALL: 'C' } });
console.log(JSON.stringify({ status: result.status, error: result.error?.message, stderr: result.stderr }));`;
  const result = run(binary, ["--eval", source], {
    env: { ...env, ELECTRON_RUN_AS_NODE: "1" },
    encoding: "utf8",
    timeout: 5000,
  });
  if (result.error || result.status !== 0)
    throw new Error(
      result.error?.message ||
        result.stderr ||
        `Electron exited with code ${result.status}`,
    );
  return JSON.parse(result.stdout.trim());
}

export async function checkUbuntuSandbox({
  binary,
  env = process.env,
  platform = process.platform,
  text = messages["en-US"],
  io = fs,
  probe = probeUserNamespace,
  temporaryDirectory = tmpdir(),
}) {
  if (platform !== "linux") return;
  let release;
  try {
    release = await io.readFile("/etc/os-release", "utf8");
  } catch {
    try {
      release = await io.readFile("/usr/lib/os-release", "utf8");
    } catch {
      return;
    }
  }
  if (!isUbuntu(release)) return;

  // A correctly installed SUID helper also supports the Chromium sandbox.
  try {
    const helper = await io.stat(
      path.join(path.dirname(binary), "chrome-sandbox"),
    );
    if (
      helper.isFile() &&
      helper.uid === 0 &&
      (helper.mode & 0o7777) === 0o4755
    )
      return;
  } catch {
    // Prefer the application-specific user-namespace permission below.
  }
  const kernelSetting = async (name) => {
    try {
      return (await io.readFile(`/proc/sys/kernel/${name}`, "utf8")).trim();
    } catch {
      return undefined;
    }
  };
  if ((await kernelSetting("unprivileged_userns_clone")) === "0")
    throw new Error(text.userns);
  if ((await kernelSetting("apparmor_restrict_unprivileged_userns")) !== "1")
    return;

  let result;
  try {
    result = probe(binary, env);
  } catch (error) {
    throw new Error(text.probe(error.message), { cause: error });
  }
  if (result.status === 0) return;
  if (
    result.error ||
    !/Operation not permitted|Permission denied/.test(result.stderr || "")
  )
    throw new Error(
      text.probe(
        result.error ||
          result.stderr ||
          `unshare exited with code ${result.status}`,
      ),
    );

  // Create a private, reviewable file. Never invoke sudo or change host policy.
  const directory = await io.mkdtemp(
    path.join(temporaryDirectory, "one-electron-apparmor-"),
  );
  const name = sandboxProfileName(binary);
  const profile = path.join(directory, name);
  await io.writeFile(profile, sandboxProfile(binary), {
    mode: 0o600,
    flag: "wx",
  });
  const target = `/etc/apparmor.d/${name}`;
  const commands = [
    `sudo install -o root -g root -m 644 -- ${shellQuote(profile)} ${shellQuote(target)}`,
    `sudo apparmor_parser -r ${shellQuote(target)}`,
  ].join("\n");
  throw new Error(text.sandbox(binary, profile, commands));
}
