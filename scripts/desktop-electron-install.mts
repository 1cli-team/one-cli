import { createRequire } from "node:module";
import { resolve } from "node:path";
import { repositoryRoot } from "./lib/repository.mts";
import { shell } from "./lib/shell.mts";
import { isMain, runMain } from "./lib/entrypoint.mts";

// Electron 44 exposes an explicit install-electron command rather than an npm
// postinstall. Keep runtime preparation part of install-deps on every platform.
function installElectron() {
  const require = createRequire(resolve(repositoryRoot, "services/desktop-main/package.json"));
  const install = require.resolve("electron/install.js");
  shell({ cwd: repositoryRoot, stdio: "inherit" })`${process.execPath} ${install}`;
}
if (isMain(import.meta.url)) await runMain(installElectron);
