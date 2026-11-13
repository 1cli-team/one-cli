import type { Plan, Release, RemoteAsset, Tag } from "../release-rules.mts";

export type GitHubRelease = Omit<Release, "assets"> & { id: number; assets: RemoteAsset[] };

export type State = { master: string; tags: Tag[]; releases: Release[] };
export type Build = {
  schema: "one-cli/release-build/v1";
  plan: Plan;
  source: string;
  config: string;
  notes: string;
  hashes: Record<string, string>;
};
