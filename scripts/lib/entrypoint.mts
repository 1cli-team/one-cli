import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

export function isMain(url: string) {
  return Boolean(process.argv[1] && url === pathToFileURL(resolve(process.argv[1])).href);
}

export function runMain(main: () => void) {
  try {
    main();
  } catch (error) {
    console.error(error instanceof Error ? error.message : error);
    process.exitCode = 1;
  }
}
