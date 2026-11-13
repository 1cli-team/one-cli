import { copyFileSync, mkdirSync, readdirSync, rmSync } from "node:fs";
import { join } from "node:path";
import { desktopBundles, desktopFilename as filename, platforms } from "../release-assets.mts";
import { stableTag } from "../release-rules.mts";
import { sha256File, writeJson } from "../lib/files.mts";
import { fail } from "../lib/output.mts";
import type { DesktopManifest as Manifest, Installer } from "./types.mts";

export function collectInstallers(workspace: string, manifest: Manifest) {
  const p = platforms[manifest.target];
  if (!p || manifest.channel !== "release" || !stableTag.test(`v${manifest.version}`))
    fail("Installer collection requires a release build", "安装包收集需要正式构建");
  const sources = desktopBundles(manifest.version, manifest.target).map((bundle) => {
    const directory = join(
      workspace,
      "apps/desktop/src-tauri/target",
      manifest.target,
      "release/bundle",
      bundle,
    );
    const extension = bundle === "deb" ? ".deb" : p.extension;
    const files = readdirSync(directory).filter((name) => name.endsWith(extension));
    if (files.length !== 1)
      fail("Expected exactly one installer per format", "每种格式必须生成唯一的安装包");
    return {
      path: join(directory, files[0]),
      name: filename(manifest.version, manifest.target, bundle),
    };
  });
  const out = join(workspace, "desktop-dist", manifest.target);
  rmSync(out, { recursive: true, force: true });
  mkdirSync(out, { recursive: true });
  const artifacts: Installer[] = sources.map(({ path, name }) => {
    copyFileSync(path, join(out, name));
    console.log(`Installer ready / 安装包就绪: ${join(out, name)}`);
    return {
      target: manifest.target,
      name,
      sha256: sha256File(join(out, name)),
      notarized: p.os === "darwin",
    };
  });
  // Called only by desktop.mts after signing, notarization and bundle validation.
  writeJson(join(out, `${manifest.target}.json`), {
    version: manifest.version,
    commit: manifest.commit,
    artifacts,
  });
}
