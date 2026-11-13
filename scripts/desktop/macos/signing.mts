import { join } from "node:path";
import { fail } from "../../lib/output.mts";
import { CommandError } from "../../lib/shell.mts";
import { appleCommand, capture, run, secret } from "./commands.mts";
import { notaryCredentials } from "./keychain.mts";

export class MissingMacAppTicketError extends Error {
  constructor() {
    super("macOS app has no stapled ticket / macOS 应用缺少公证票据");
  }
}

export function signMacSidecar(path: string, identity: string) {
  const timestamp = identity === "-" ? "--timestamp=none" : "--timestamp";
  const sign = identity === "-" ? run : appleCommand;
  sign`codesign --force --sign ${identity} --options runtime ${timestamp} ${path}`;
  run`codesign --verify --strict ${path}`;
}

function verifySignature(path: string) {
  run`codesign --verify --deep --strict ${path}`;
  const result = capture`codesign -dv --verbose=4 ${path}`;
  const details = result.stdout + "\n" + result.stderr;
  if (
    !result.ok ||
    !details.includes(`TeamIdentifier=${process.env.APPLE_TEAM_ID}\n`) ||
    !details.includes("Authority=Developer ID Application:") ||
    !details.includes("runtime")
  )
    fail(
      "Missing expected Developer ID signature or hardened runtime",
      "缺少预期 Developer ID 签名或强化运行时",
    );
}

export function verifyMacApp(app: string) {
  for (const path of [join(app, "Contents/MacOS/one"), join(app, "Contents/MacOS/one-api"), app])
    verifySignature(path);
  try {
    appleCommand`xcrun stapler validate ${app}`;
  } catch (error) {
    if (
      error instanceof CommandError &&
      /does not have a ticket stapled to it/.test(error.result.stdout + error.result.stderr)
    )
      throw new MissingMacAppTicketError();
    throw error;
  }
  run`spctl --assess --type execute --verbose ${app}`;
}

export function notarizeMacDMG(dmg: string) {
  run`hdiutil verify ${dmg}`;
  run`codesign --verify --strict ${dmg}`;
  const credentials = notaryCredentials();
  const report = JSON.parse(
    secret`xcrun notarytool submit ${dmg} ${credentials} --wait --timeout 45m --output-format json`,
  );
  if (report.status !== "Accepted")
    fail(
      `DMG notarization not accepted: ${report.id ?? "unknown"} (${report.status ?? "unknown"})`,
      `DMG 公证未通过：${report.id ?? "unknown"} (${report.status ?? "unknown"})`,
    );
  console.log(`DMG notarization accepted / DMG 公证通过: ${report.id}`);
  appleCommand`xcrun stapler staple ${dmg}`;
  appleCommand`xcrun stapler validate ${dmg}`;
  run`spctl --assess --type open --context context:primary-signature --verbose ${dmg}`;
  run`hdiutil verify ${dmg}`;
}
