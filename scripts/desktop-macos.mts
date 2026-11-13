import { cleanupMacSigning } from "./desktop/macos/keychain.mts";
import { isMain, runMain } from "./lib/entrypoint.mts";
import { fail } from "./lib/output.mts";

export { cleanupMacSigning, withMacSigning } from "./desktop/macos/keychain.mts";
export {
  MissingMacAppTicketError,
  notarizeMacDMG,
  signMacSidecar,
  verifyMacApp,
} from "./desktop/macos/signing.mts";

if (isMain(import.meta.url))
  runMain(() => {
    if (process.argv[2] !== "cleanup" || process.platform !== "darwin")
      fail(
        "Usage: node scripts/desktop-macos.mts cleanup (macOS)",
        "用法：node scripts/desktop-macos.mts cleanup（macOS）",
      );
    cleanupMacSigning();
  });
