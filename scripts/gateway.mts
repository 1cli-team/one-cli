import { existsSync, statSync } from "node:fs";
import { createServer } from "node:net";
import { homedir } from "node:os";
import { isAbsolute, join } from "node:path";
import { isMain, runMain } from "./lib/entrypoint.mts";
import { readJson } from "./lib/files.mts";
import { fail } from "./lib/output.mts";
import { repositoryRoot } from "./lib/repository.mts";
import { shell } from "./lib/shell.mts";

function readConnection(path: string) {
  let saved: { url?: unknown; token?: unknown } | null;
  try {
    const info = statSync(path);
    if (!info.isFile() || (info.mode & 0o077) !== 0 || info.size > 16 * 1024) {
      throw new Error();
    }
    saved = readJson(path);
  } catch {
    fail(
      `Save a local Gateway connection in One Desktop settings and check that ${path} is private (0600).`,
      `请在 One Desktop 设置中保存本机 Gateway 连接，并确认 ${path} 的权限为 0600。`,
    );
  }

  const url = typeof saved?.url === "string" ? URL.parse(saved.url) : null;
  if (
    !url || url.protocol !== "http:" || url.pathname !== "/" ||
    !["localhost", "127.0.0.1", "[::1]"].includes(url.hostname) ||
    url.port === "0" || url.username || url.password || url.search || url.hash
  ) {
    fail(
      "Set the saved Gateway address to a local HTTP endpoint such as http://127.0.0.1:39128.",
      "请将已保存的 Gateway 地址设为本机 HTTP 地址，例如 http://127.0.0.1:39128。",
    );
  }
  const token = saved?.token;
  if (typeof token !== "string" || token.length < 32 || /\s/.test(token)) {
    fail(
      "Save a Gateway token of at least 32 characters without whitespace in One Desktop.",
      "请在 One Desktop 中保存至少 32 个字符且不含空白的 Gateway 令牌。",
    );
  }
  const host = url.hostname === "localhost" ? "127.0.0.1" : url.hostname;
  return { host, port: Number(url.port || 80), token };
}

async function requireAvailablePort(host: string, port: number, listen: string) {
  const server = createServer();
  try {
    await new Promise<void>((ready, reject) =>
      server.once("error", reject).listen(port, host, ready),
    );
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "EADDRINUSE") {
      fail(
        `Address ${listen} is already in use. Stop its current listener to start a new Gateway.`,
        `地址 ${listen} 已被占用。停止占用该地址的进程后可启动新的 Gateway。`,
      );
    }
    throw error;
  }
  await new Promise<void>((closed, reject) =>
    server.close((error) => error ? reject(error) : closed()),
  );
}

async function startGateway() {
  if (process.platform !== "darwin" && process.platform !== "linux") {
    fail("Local Gateway development requires macOS or Linux.", "本机 Gateway 开发需要 macOS 或 Linux。");
  }
  const oneHome = process.env.ONE_HOME?.trim() || join(process.env.HOME?.trim() || homedir(), ".one");
  if (!isAbsolute(oneHome)) {
    fail("ONE_HOME must be an absolute path.", "ONE_HOME 必须为绝对路径。");
  }
  const config = join(oneHome, "config", "gateway-server.json");
  if (!existsSync(config)) {
    fail(
      `Create the local workflow configuration at ${config}. See services/gateway/README.md.`,
      `请在 ${config} 创建本机流程配置，格式见 services/gateway/README.md。`,
    );
  }
  const { host, port, token } = readConnection(join(oneHome, "config", "gateway.json"));
  const listen = `${host}:${port}`;
  await requireAvailablePort(host === "[::1]" ? "::1" : host, port, listen);

  const build = shell({ cwd: repositoryRoot, stdio: "inherit" });
  build`mise run cli:build`;
  build`mise run gateway:build`;

  const binary = join(repositoryRoot, "services", "gateway", "bin", "one-gateway");
  const one = join(repositoryRoot, "packages", "cli", "bin", "one");
  // Replace the launcher so the Gateway receives the task runner's stop signals.
  const args = [binary, "-one", one, "-config", config, "-listen", listen];
  if (process.stdin.isTTY) args.push("-show-token");
  process.execve(binary, args, {
    ...process.env,
    ONE_HOME: oneHome,
    ONE_GATEWAY_TOKEN: token,
  });
}

if (isMain(import.meta.url)) await runMain(startGateway);
