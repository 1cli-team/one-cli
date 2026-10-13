import { spawnSync } from "node:child_process";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const desktop = resolve(root, "apps/desktop");
const native = resolve(desktop, "src-tauri");
const command = process.argv[2] ?? "prepare";
function run(bin: string, args: string[], cwd = root, env = process.env) {
 const result = spawnSync(bin, args, { cwd, env, stdio: "inherit" });
 if (result.error) throw result.error;
 if (result.status !== 0) throw new Error(`${bin} exited with ${result.status}`);
}
function output(bin: string, args: string[]) {
 const result = spawnSync(bin, args, { cwd: root, encoding: "utf8" });
 if (result.status !== 0) throw new Error(result.stderr || `${bin} failed`);
 return result.stdout.trim();
}
const platforms: Record<string, [string,string]> = {
 "aarch64-apple-darwin": ["darwin","arm64"],
 "x86_64-apple-darwin": ["darwin","amd64"],
 "x86_64-pc-windows-msvc": ["windows","amd64"],
 "x86_64-unknown-linux-gnu": ["linux","amd64"],
 "aarch64-unknown-linux-gnu": ["linux","arm64"],
};
const target = process.env.ONE_DESKTOP_TARGET ?? output("rustc", ["--print", "host-tuple"]);
const platform = platforms[target];
if (!platform) throw new Error(`Unsupported desktop target / 不支持的桌面目标: ${target}`);
const version = process.env.RELEASE_VERSION ?? "0.0.0-dev";
const commit = output("git", ["rev-parse", "HEAD"]);
const release = process.env.ONE_DESKTOP_RELEASE === "1";
if (release && (!/^\d+\.\d+\.\d+$/.test(version) || output("git", ["status", "--porcelain"]))) {
 throw new Error("Release builds require a stable version and clean committed source / 正式构建需要稳定版本和干净提交");
}
const channel = release ? "release" : "development";
const bins = resolve(native, "binaries");
mkdirSync(bins, {recursive:true});
run("go", ["run", "./packages/cli/tools/sync-resources", "bundled"]);
const flags = ["-s", "-w", `-X main.version=${version}`, `-X main.commit=${commit}`, `-X github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/updatecheck.buildChannel=${channel}`];
if (release) flags.push(`-X github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/skills.bundledSourceRef=${commit}`);
const hashes: Record<string,string> = {};
for (const [name, entry] of [["one-api", "./services/desktop-api/cmd/one-api"], ["one", "./packages/cli/cmd/one"]]) {
 const path = resolve(bins, `${name}-${target}${platform[0]==="windows"?".exe":""}`);
 run("go", ["build", "-trimpath", "-ldflags", flags.join(" "), "-o", path, entry], root, {...process.env, CGO_ENABLED:"0", GOOS:platform[0], GOARCH:platform[1]});
 hashes[name] = createHash("sha256").update(readFileSync(path)).digest("hex");
}
writeFileSync(resolve(native,"bundle-manifest.json"), JSON.stringify({version,commit,channel,protocol:1,api:1,target,sha256:hashes["one-api"],cliSha256:hashes.one},null,2)+"\n");
const config: Record<string,unknown> = {version};
if (release) {
 if (!process.env.ONE_DESKTOP_UPDATER_PUBLIC_KEY || !process.env.TAURI_SIGNING_PRIVATE_KEY) throw new Error("Signed desktop release requires updater keys / 正式桌面发布需要更新签名配置");
 config.bundle = {createUpdaterArtifacts:true};
 config.plugins = {updater:{pubkey:process.env.ONE_DESKTOP_UPDATER_PUBLIC_KEY}};
}
writeFileSync(resolve(native,"build-config.json"),JSON.stringify(config,null,2)+"\n");
if(command === "prepare") process.exit(0);
if(command !== "build" && command !== "dev") throw new Error(`Unknown desktop operation: ${command}`);
const args = ["exec", "tauri", command, "--config", "src-tauri/build-config.json"];
if(command==="build") args.push("--target",target,"--bundles",platform[0]==="darwin"?"app,dmg":platform[0]==="windows"?"nsis":"appimage");
args.push(...process.argv.slice(3));
run("pnpm",args,desktop);
