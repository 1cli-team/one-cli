import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

export function isMain(url: string) {
  return Boolean(process.argv[1] && url === pathToFileURL(resolve(process.argv[1])).href);
}

export async function runMain(main: () => void | Promise<void>) {
  try {
    await main();
  } catch (error) {
    console.error(error instanceof Error ? error.message : error);
    process.exitCode = 1;
  }
}
