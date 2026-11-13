import { runDesktop } from "./desktop/build.mts";
import { isMain, runMain } from "./lib/entrypoint.mts";

if (isMain(import.meta.url)) runMain(() => runDesktop());
