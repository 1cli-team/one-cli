import { planDesktop, publishDesktop } from "./release/desktop.mts";
import { isMain, runMain } from "./lib/entrypoint.mts";
import { fail } from "./lib/output.mts";

export { collectInstallers } from "./desktop/installers.mts";
export { platforms } from "./release-assets.mts";

if (isMain(import.meta.url))
  runMain(() => {
    const [
      action,
      tag = process.env.ONE_RELEASE_TAG ?? "",
      input = process.env.ONE_DESKTOP_ARTIFACTS ?? "",
    ] = process.argv.slice(2);
    if (action === "plan") planDesktop(tag);
    else if (action === "publish") publishDesktop(tag, input);
    else
      fail(
        "Usage: node scripts/desktop-release.mts <plan|publish> vX.Y.Z [artifact-directory]",
        "用法：node scripts/desktop-release.mts <plan|publish> vX.Y.Z [构建产物目录]",
      );
  });
