// This boundary is fixed: never infer it from a moving Latest release.
export const unifiedReleaseSince = "v0.1.22";
export const debianReleaseSince = "v0.1.23";

export const platforms: Record<
  string,
  { os: string; arch: string; bundle: string; extension: string }
> = {
  "aarch64-apple-darwin": { os: "darwin", arch: "arm64", bundle: "dmg", extension: ".dmg" },
  "x86_64-apple-darwin": { os: "darwin", arch: "amd64", bundle: "dmg", extension: ".dmg" },
  "x86_64-pc-windows-msvc": { os: "windows", arch: "amd64", bundle: "nsis", extension: ".exe" },
  "x86_64-unknown-linux-gnu": {
    os: "linux",
    arch: "amd64",
    bundle: "appimage",
    extension: ".AppImage",
  },
  "aarch64-unknown-linux-gnu": {
    os: "linux",
    arch: "arm64",
    bundle: "appimage",
    extension: ".AppImage",
  },
};

export function desktopFilename(
  version: string,
  target: string,
  bundle = platforms[target].bundle,
) {
  const p = platforms[target];
  const extension = bundle === "deb" && p.os === "linux" ? ".deb" : p.extension;
  return `one-desktop_${version}_${p.os}_${p.arch}${extension}`;
}

export function desktopBundles(version: string, target: string) {
  const p = platforms[target];
  const current = version.split(".").map(BigInt);
  const since = debianReleaseSince.slice(1).split(".").map(BigInt);
  const difference = current.findIndex((part, index) => part !== since[index]);
  const includeDeb = difference === -1 || current[difference] > since[difference];
  return p.os === "linux" && includeDeb ? [p.bundle, "deb"] : [p.bundle];
}

export function desktopInstallerNames(version: string, target: string) {
  return desktopBundles(version, target).map((bundle) => desktopFilename(version, target, bundle));
}

export function desktopAssets(version: string) {
  return [
    ...Object.keys(platforms).flatMap((target) => desktopInstallerNames(version, target)),
    "desktop-release.json",
    "desktop-checksums.txt",
  ].sort();
}
