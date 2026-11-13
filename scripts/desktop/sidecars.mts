import { mkdirSync } from "node:fs";
import { resolve } from "node:path";
import { sha256File } from "../lib/files.mts";
import { shell } from "../lib/shell.mts";
import { signMacSidecar } from "./macos/signing.mts";
import type { DesktopConfig } from "./config.mts";

const binaries = [
  ["one-api", "./services/desktop-api/cmd/one-api"],
  ["one", "./packages/cli/cmd/one"],
] as const;

export function buildSidecars(config: DesktopConfig, identity?: string) {
  const directory = resolve(config.native, "binaries");
  mkdirSync(directory, { recursive: true });
  const run = shell({ cwd: config.root, env: config.env, stdio: "inherit" });
  run`go run ./packages/cli/tools/sync-resources bundled`;

  const packagePath = "github.com/torchstellar-team/one-cli/packages/one-core";
  const flags = [
    "-s",
    "-w",
    `-X main.version=${config.version}`,
    `-X main.commit=${config.commit}`,
    `-X ${packagePath}/pkg/platform/updatecheck.buildChannel=${config.channel}`,
  ];
  if (config.release)
    flags.push(`-X ${packagePath}/pkg/modules/skills.bundledSourceRef=${config.commit}`);

  const compile = shell({
    cwd: config.root,
    stdio: "inherit",
    env: {
      ...config.env,
      CGO_ENABLED: "0",
      GOOS: config.platform.os,
      GOARCH: config.platform.arch,
    },
  });
  const hashes: Record<string, string> = {};
  const signedFiles: Record<string, string> = {};
  for (const [name, entry] of binaries) {
    const extension = config.platform.os === "windows" ? ".exe" : "";
    const path = resolve(directory, `${name}-${config.target}${extension}`);
    compile`go build -trimpath -ldflags ${flags.join(" ")} -o ${path} ${entry}`;
    if (identity) {
      signMacSidecar(path, identity);
      signedFiles[`MacOS/${name}`] = path;
    }
    hashes[name] = sha256File(path);
  }
  return { hashes, signedFiles };
}

export type Sidecars = ReturnType<typeof buildSidecars>;
