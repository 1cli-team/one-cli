import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { shell } from "./shell.mts";

export const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");

export function git(cwd: string, ...args: string[]) {
  return shell({ cwd })`git ${args}`.stdout.trim();
}

export function gitSucceeds(cwd: string, ...args: string[]) {
  return shell({ cwd, nothrow: true })`git ${args}`.ok;
}
