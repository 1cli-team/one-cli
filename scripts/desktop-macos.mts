import { spawnSync } from "node:child_process";
import { randomBytes } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { pathToFileURL } from "node:url";

const directory = join(process.env.RUNNER_TEMP ?? tmpdir(), "one-desktop-signing");
const keychain = join(directory, "build.keychain-db");
const searchList = join(directory, "search-list.json");

function run(bin: string, args: string[], privateOutput = false) {
  // Never include arguments in errors: security/notarytool receive credentials.
  const result = spawnSync(bin, args, {
    encoding: "utf8",
    stdio: privateOutput ? "pipe" : "inherit",
  });
  if (result.error || result.status !== 0) {
    throw new Error(
      `${bin} failed / 执行失败${privateOutput ? " (credential output withheld / 隐藏凭据输出)" : ""}`,
    );
  }
  return result.stdout?.trim() ?? "";
}

export function cleanupMacSigning() {
  if (!existsSync(directory)) return;
  // The saved list is local to this runner, never part of uploaded artifacts.
  try {
    if (existsSync(searchList)) {
      const original: string[] = JSON.parse(readFileSync(searchList, "utf8"));
      run("security", ["list-keychains", "-d", "user", "-s", ...original], true);
    }
  } finally {
    try {
      if (existsSync(keychain)) run("security", ["delete-keychain", keychain], true);
    } finally {
      rmSync(directory, { recursive: true, force: true });
    }
  }
}

export function withMacSigning<T>(action: (identity: string) => T): T {
  for (const name of [
    "APPLE_TEAM_ID",
    "APPLE_ID",
    "APPLE_PASSWORD",
    "APPLE_CERTIFICATE_BASE64",
    "APPLE_CERTIFICATE_PASSWORD",
  ]) {
    if (!process.env[name]?.trim()) throw new Error(`Missing CI Secret / 缺少 CI Secret: ${name}`);
  }
  if (existsSync(directory))
    throw new Error("Signing directory already exists; clean it first / 签名目录已存在，请先清理");
  mkdirSync(directory, { mode: 0o700 });
  try {
    const original = [
      ...run("security", ["list-keychains", "-d", "user"], true).matchAll(/"([^"]+)"/g),
    ].map((m) => m[1]);
    if (!original.length) throw new Error("Cannot read keychain search list / 无法读取钥匙串列表");
    writeFileSync(searchList, JSON.stringify(original), { mode: 0o600 });
    const password = randomBytes(32).toString("hex");
    const certificate = join(directory, "certificate.p12");
    writeFileSync(certificate, Buffer.from(process.env.APPLE_CERTIFICATE_BASE64!, "base64"), {
      mode: 0o600,
    });
    run("security", ["create-keychain", "-p", password, keychain], true);
    run("security", ["set-keychain-settings", "-lut", "21600", keychain], true);
    run("security", ["unlock-keychain", "-p", password, keychain], true);
    run(
      "security",
      [
        "import",
        certificate,
        "-k",
        keychain,
        "-P",
        process.env.APPLE_CERTIFICATE_PASSWORD!,
        "-T",
        "/usr/bin/codesign",
        "-T",
        "/usr/bin/security",
      ],
      true,
    );
    rmSync(certificate);
    run(
      "security",
      [
        "set-key-partition-list",
        "-S",
        "apple-tool:,apple:,codesign:",
        "-s",
        "-k",
        password,
        keychain,
      ],
      true,
    );
    run("security", ["list-keychains", "-d", "user", "-s", keychain, ...original], true);
    const identities = [
      ...run("security", ["find-identity", "-v", "-p", "codesigning", keychain], true).matchAll(
        /[A-Fa-f0-9]{40} "(Developer ID Application: [^"]+)"/g,
      ),
    ]
      .map((m) => m[1])
      .filter((name) => name.endsWith(`(${process.env.APPLE_TEAM_ID})`));
    if (identities.length !== 1)
      throw new Error(
        "Expected one valid Developer ID Application identity matching APPLE_TEAM_ID / 需要唯一有效且 Team ID 匹配的 Developer ID Application 证书及私钥",
      );
    run(
      "xcrun",
      [
        "notarytool",
        "store-credentials",
        "one-desktop-notary",
        "--validate",
        "--apple-id",
        process.env.APPLE_ID!,
        "--password",
        process.env.APPLE_PASSWORD!,
        "--team-id",
        process.env.APPLE_TEAM_ID!,
        "--keychain",
        keychain,
      ],
      true,
    );
    return action(identities[0]);
  } finally {
    cleanupMacSigning();
  }
}

export function signMacSidecar(path: string, identity: string) {
  run("codesign", [
    "--force",
    "--sign",
    identity,
    "--options",
    "runtime",
    identity === "-" ? "--timestamp=none" : "--timestamp",
    path,
  ]);
  run("codesign", ["--verify", "--strict", path]);
}

export function verifyMacApp(app: string) {
  for (const path of [join(app, "Contents/MacOS/one"), join(app, "Contents/MacOS/one-api"), app]) {
    run("codesign", ["--verify", "--deep", "--strict", path]);
    const result = spawnSync("codesign", ["-dv", "--verbose=4", path], { encoding: "utf8" });
    const details = `${result.stdout}\n${result.stderr}`;
    if (
      result.status !== 0 ||
      !details.includes(`TeamIdentifier=${process.env.APPLE_TEAM_ID}\n`) ||
      !details.includes("Authority=Developer ID Application:") ||
      !details.includes("runtime")
    ) {
      throw new Error(
        "Missing expected Developer ID signature or hardened runtime / 缺少预期 Developer ID 签名或强化运行时",
      );
    }
  }
  run("xcrun", ["stapler", "validate", app]);
  run("spctl", ["--assess", "--type", "execute", "--verbose", app]);
}

export function notarizeMacDMG(dmg: string) {
  run("hdiutil", ["verify", dmg]);
  run("codesign", ["--verify", "--strict", dmg]);
  const report = JSON.parse(
    run(
      "xcrun",
      [
        "notarytool",
        "submit",
        dmg,
        "--apple-id",
        process.env.APPLE_ID!,
        "--password",
        process.env.APPLE_PASSWORD!,
        "--team-id",
        process.env.APPLE_TEAM_ID!,
        "--wait",
        "--timeout",
        "45m",
        "--output-format",
        "json",
      ],
      true,
    ),
  );
  if (report.status !== "Accepted")
    throw new Error(
      `DMG notarization not accepted / DMG 公证未通过: ${report.id ?? "unknown"} (${report.status ?? "unknown"})`,
    );
  run("xcrun", ["stapler", "staple", dmg]);
  run("xcrun", ["stapler", "validate", dmg]);
  run("spctl", [
    "--assess",
    "--type",
    "open",
    "--context",
    "context:primary-signature",
    "--verbose",
    dmg,
  ]);
  run("hdiutil", ["verify", dmg]);
  console.log(`DMG notarization accepted / DMG 公证通过: ${report.id}`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  if (process.argv[2] !== "cleanup" || process.platform !== "darwin")
    throw new Error("Usage / 用法: node scripts/desktop-macos.mts cleanup (macOS)");
  cleanupMacSigning();
}
