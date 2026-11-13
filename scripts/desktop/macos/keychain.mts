import { randomBytes } from "node:crypto";
import { existsSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { readJson, writeJson } from "../../lib/files.mts";
import { fail } from "../../lib/output.mts";
import { secret } from "./commands.mts";

const directory = join(process.env.RUNNER_TEMP ?? tmpdir(), "one-desktop-signing");
const keychain = join(directory, "build.keychain-db");
const searchList = join(directory, "search-list.json");

export function notaryCredentials() {
  return [
    "--apple-id",
    process.env.APPLE_ID!,
    "--password",
    process.env.APPLE_PASSWORD!,
    "--team-id",
    process.env.APPLE_TEAM_ID!,
  ];
}

export function cleanupMacSigning() {
  if (!existsSync(directory)) return;
  try {
    if (existsSync(searchList)) {
      const original = readJson<string[]>(searchList);
      secret`security list-keychains -d user -s ${original}`;
    }
  } finally {
    try {
      if (existsSync(keychain)) secret`security delete-keychain ${keychain}`;
    } finally {
      rmSync(directory, { recursive: true, force: true });
    }
  }
}

function signingIdentity() {
  const identities = [
    ...secret`security find-identity -v -p codesigning ${keychain}`.matchAll(
      /[A-Fa-f0-9]{40} "(Developer ID Application: [^"]+)"/g,
    ),
  ]
    .map((match) => match[1])
    .filter((name) => name.endsWith(`(${process.env.APPLE_TEAM_ID})`));
  if (identities.length !== 1)
    fail(
      "Expected one valid Developer ID Application identity matching APPLE_TEAM_ID",
      "需要唯一有效且 Team ID 匹配的 Developer ID Application 证书及私钥",
    );
  return identities[0];
}

function prepareKeychain() {
  const original = [...secret`security list-keychains -d user`.matchAll(/"([^"]+)"/g)].map(
    (match) => match[1],
  );
  if (!original.length) fail("Cannot read keychain search list", "无法读取钥匙串列表");
  writeJson(searchList, original, { mode: 0o600 });
  const password = randomBytes(32).toString("hex");
  const certificate = join(directory, "certificate.p12");
  writeFileSync(certificate, Buffer.from(process.env.APPLE_CERTIFICATE_BASE64!, "base64"), {
    mode: 0o600,
  });
  secret`security create-keychain -p ${password} ${keychain}`;
  secret`security set-keychain-settings -lut 21600 ${keychain}`;
  secret`security unlock-keychain -p ${password} ${keychain}`;
  const access = [
    "-k",
    keychain,
    "-P",
    process.env.APPLE_CERTIFICATE_PASSWORD!,
    "-T",
    "/usr/bin/codesign",
    "-T",
    "/usr/bin/security",
  ];
  secret`security import ${certificate} ${access}`;
  rmSync(certificate);
  secret`security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k ${password} ${keychain}`;
  secret`security list-keychains -d user -s ${keychain} ${original}`;
  const identity = signingIdentity();
  secret`xcrun notarytool store-credentials one-desktop-notary --validate ${notaryCredentials()} --keychain ${keychain}`;
  return identity;
}

export function withMacSigning<T>(action: (identity: string) => T): T {
  for (const name of [
    "APPLE_TEAM_ID",
    "APPLE_ID",
    "APPLE_PASSWORD",
    "APPLE_CERTIFICATE_BASE64",
    "APPLE_CERTIFICATE_PASSWORD",
  ]) {
    if (!process.env[name]?.trim()) fail(`Missing CI Secret: ${name}`, `缺少 CI Secret：${name}`);
  }
  if (existsSync(directory))
    fail("Signing directory already exists; clean it first", "签名目录已存在，请先清理");
  mkdirSync(directory, { mode: 0o700 });
  try {
    return action(prepareKeychain());
  } finally {
    cleanupMacSigning();
  }
}
