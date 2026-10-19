// Prepare the browser application embedded in the standalone Go server.
import { spawnSync } from "node:child_process";
import { cpSync, mkdirSync, readdirSync, rmSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const result = spawnSync("pnpm", ["run", "build:web"], {cwd:resolve(root,"apps/desktop"),stdio:"inherit"});
if (result.error) throw result.error;
if (result.status !== 0) process.exit(result.status ?? 1);
const assets = resolve(root,"services/server/internal/web/assets");
mkdirSync(assets,{recursive:true});
for (const file of readdirSync(assets)) if (file !== "placeholder.txt") rmSync(resolve(assets,file),{recursive:true,force:true});
cpSync(resolve(root,"apps/desktop/dist"),assets,{recursive:true});
