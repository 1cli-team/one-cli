// This boundary is fixed: never infer it from a moving Latest release.
export const unifiedReleaseSince = "v0.1.22";

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

export function desktopFilename(version: string, target: string) {
  const p = platforms[target];
  return `one-desktop_${version}_${p.os}_${p.arch}${p.extension}`;
}

export function desktopAssets(version: string) {
  return [
    ...Object.keys(platforms).map((target) => desktopFilename(version, target)),
    "desktop-release.json",
    "desktop-checksums.txt",
  ].sort();
}
