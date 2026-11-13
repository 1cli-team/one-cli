import { fail } from "../lib/output.mts";
import { buildBundle, writeBundleConfig } from "./bundle.mts";
import { readDesktopConfig } from "./config.mts";
import { collectInstallers } from "./installers.mts";
import { withMacSigning } from "./macos/keychain.mts";
import { buildSidecars } from "./sidecars.mts";
import type { DesktopConfig } from "./config.mts";

function build(config: DesktopConfig, identity?: string) {
  const sidecars = buildSidecars(config, identity);
  const manifest = writeBundleConfig(config, sidecars, identity);
  if (config.command === "prepare") return;
  buildBundle(config, sidecars, identity);
  if (config.release && config.command === "build") collectInstallers(config.root, manifest);
}

export function runDesktop(args = process.argv.slice(2)) {
  const config = readDesktopConfig(args);
  const macBuild = config.command === "build" && config.platform.os === "darwin";
  if (macBuild && config.release) {
    if (process.platform !== "darwin")
      fail("macOS releases must be built on macOS", "macOS 正式版必须在 macOS 构建");
    withMacSigning((identity) => build(config, identity));
    return;
  }
  // Ad-hoc signatures identify local apps to macOS privacy controls as well.
  build(config, macBuild ? (config.env.APPLE_SIGNING_IDENTITY ?? "-") : undefined);
}
