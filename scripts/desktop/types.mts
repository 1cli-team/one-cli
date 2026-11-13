export type DesktopManifest = {
  version: string;
  commit: string;
  channel: string;
  target: string;
  sha256: string;
  cliSha256: string;
};

export type Installer = { target: string; name: string; sha256: string; notarized: boolean };
export type DesktopReleaseRecord = {
  version: string;
  commit: string;
  cliTag: string;
  artifacts: Installer[];
};
export type DesktopBuildRecord = {
  version: string;
  commit: string;
} & ({ artifacts: Installer[]; artifact?: never } | { artifact: Installer; artifacts?: never });
