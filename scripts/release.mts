import { runRelease } from "./release/cli.mts";
import { isMain, runMain } from "./lib/entrypoint.mts";

export { smoke } from "./release/smoke.mts";

if (isMain(import.meta.url)) runMain(() => runRelease());
