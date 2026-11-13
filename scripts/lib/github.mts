import { mkdirSync } from "node:fs";
import { repository } from "../release-rules.mts";
import { fail } from "./output.mts";
import { repositoryRoot } from "./repository.mts";
import { $, shell } from "./shell.mts";

type ApiOptions = { fields?: string[]; optional?: boolean };

export function githubApi<T>(path: string, options: ApiOptions & { optional: true }): T | undefined;
export function githubApi<T>(path: string, options?: ApiOptions & { optional?: false }): T;
export function githubApi<T>(path: string, { fields = [], optional = false }: ApiOptions = {}) {
  const result = shell({
    cwd: repositoryRoot,
    nothrow: true,
  })`gh api ${`repos/${repository}/${path}`} ${fields}`;
  if (optional && !result.ok && /\(HTTP 404\)/.test(result.stderr)) return undefined;
  if (!result.ok) fail(result.stderr || "GitHub API failed", "GitHub API 请求失败");
  return JSON.parse(result.stdout) as T;
}

export function listGitHubReleases<T>(): T[] {
  // JSON Lines also works with gh versions that do not support --slurp.
  const lines =
    $`gh api --paginate ${`repos/${repository}/releases?per_page=100`} --jq ${".[] | @json"}`.stdout.trim();
  return lines ? lines.split("\n").map((line) => JSON.parse(line) as T) : [];
}

export function downloadReleaseAssets(
  tag: string,
  names: string[],
  directory: string,
  { live = false } = {},
) {
  mkdirSync(directory, { recursive: true });
  const patterns = names.flatMap((name) => ["--pattern", name]);
  shell({
    cwd: repositoryRoot,
    stdio: live ? "inherit" : "pipe",
  })`gh release download ${tag} --repo ${repository} --dir ${directory} ${patterns}`;
}
