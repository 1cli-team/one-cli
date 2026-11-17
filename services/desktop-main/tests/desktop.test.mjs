import assert from "node:assert/strict";
import { test } from "node:test";
import { mkdtemp, rm, mkdir, writeFile, readFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { createRequire } from "node:module";
import { execFileSync } from "node:child_process";
import { _electron as electron } from "playwright-core";

const require = createRequire(import.meta.url);
const root = path.resolve(import.meta.dirname, "..");

function backendPIDs(parent) {
  if (process.platform === "win32") {
    return execFileSync(
      "powershell.exe",
      [
        "-NoProfile",
        "-Command",
        `Get-CimInstance Win32_Process -Filter 'ParentProcessId = ${Number(parent)}' | Where-Object Name -eq 'one-api.exe' | Select-Object -ExpandProperty ProcessId`,
      ],
      { encoding: "utf8" },
    )
      .trim()
      .split(/\s+/)
      .filter(Boolean)
      .map(Number);
  }
  return execFileSync("ps", ["-eo", "pid=,ppid=,comm="], { encoding: "utf8" })
    .split("\n")
    .map((line) => /^\s*(\d+)\s+(\d+)\s+(.+)$/.exec(line))
    .filter(
      (match) => match && Number(match[2]) === parent && path.basename(match[3]) === "one-api",
    )
    .map((match) => Number(match[1]));
}

test(
  "packaged page connects, preserves UI and protects native boundaries",
  { timeout: 90000 },
  async () => {
    const temporary = await mkdtemp(path.join(tmpdir(), "one-desktop-e2e-"));
    const profile = path.join(temporary, "profile");
    await mkdir(profile);
    const application = await electron.launch({
      executablePath: process.env.ONE_DESKTOP_TEST_EXECUTABLE || require("electron"),
      args: [
        ...(process.env.ONE_DESKTOP_TEST_NO_SANDBOX === "1" ? ["--no-sandbox"] : []),
        ...(process.env.ONE_DESKTOP_TEST_EXECUTABLE ? [] : [root]),
      ],
      env: {
        ...process.env,
        ONE_HOME: path.join(temporary, "one"),
        ONE_DESKTOP_USER_DATA: profile,
        ONE_INSTALL_DIR: path.join(temporary, "bin"),
        ONE_DESKTOP_DEV: process.env.ONE_DESKTOP_TEST_DEV === "1" ? "1" : "0",
        ELECTRON_ENABLE_SECURITY_WARNINGS: "true",
      },
      timeout: 30000,
    });
    const hostProcess = application.process();
    try {
      const page = await application.firstWindow();
      page.setDefaultTimeout(12000);
      page.setDefaultNavigationTimeout(12000);
      const errors = [];
      page.on("pageerror", (error) => errors.push(error.message));
      await page.waitForFunction(
        async () => (await window.desktop.getBackendConnection()).status === "httpReady",
      );
      await page.getByRole("dialog").waitFor();
      assert.equal(await page.evaluate(() => typeof window.require), "undefined");
      const security = await application.evaluate(({ BrowserWindow }) => {
        const window = BrowserWindow.getAllWindows()[0];
        const prefs = window.webContents.getLastWebPreferences();
        return {
          isolation: prefs.contextIsolation,
          sandbox: prefs.sandbox,
          node: prefs.nodeIntegration,
        };
      });
      assert.deepEqual(security, { isolation: true, sandbox: true, node: false });
      const response = await page.evaluate(async () => {
        const c = await window.desktop.getBackendConnection();
        const result = await fetch(`http://127.0.0.1:${c.port}/api/workspaces`, {
          headers: { Authorization: `Bearer ${c.token}`, "X-One-Backend-Generation": c.generation },
        });
        return { status: result.status, data: await result.json() };
      });
      assert.equal(response.status, 200);
      assert.deepEqual(response.data.workspaces, []);
      await page.evaluate(() => window.desktop.setNativeLocale("zh-CN"));
      await assert.rejects(
        page.evaluate(() => window.desktop.setNativeLocale("invalid")),
        /invalid_locale/,
      );
      await assert.rejects(
        page.evaluate(() => window.desktop.openExternal("file:///etc/passwd")),
        /invalid_url/,
      );
      assert.equal((await page.evaluate(() => window.desktop.checkUpdate())).enabled, false);
      await page.evaluate(() => {
        localStorage.setItem("one_desktop_setup_seen", "1");
        localStorage.setItem("app_locale_mode", "zh-CN");
        localStorage.setItem("app_theme_mode", "dark");
      });
      await page.reload();
      await page.getByRole("heading", { name: "工作区", exact: true }).waitFor();
      assert.equal(await page.locator("html").getAttribute("lang"), "zh-CN");
      await page.getByRole("button", { name: "语言", exact: true }).click();
      await page.getByRole("menuitemradio", { name: "English", exact: true }).click();
      await page.getByRole("heading", { name: "Workspaces", exact: true }).waitFor();
      await page.getByRole("button", { name: "Language", exact: true }).click();
      await page.getByRole("menuitemradio", { name: "中文", exact: true }).click();
      await page.getByRole("heading", { name: "工作区", exact: true }).waitFor();
      await application.evaluate(({ BrowserWindow }) => BrowserWindow.getAllWindows()[0].close());
      assert.equal(
        await application.evaluate(({ BrowserWindow }) =>
          BrowserWindow.getAllWindows()[0].isVisible(),
        ),
        false,
      );
      await application.evaluate(({ app }) => app.emit("second-instance", {}, [], ""));
      assert.equal(
        await application.evaluate(({ BrowserWindow }) =>
          BrowserWindow.getAllWindows()[0].isVisible(),
        ),
        true,
      );
      await page.reload();
      await page.getByRole("heading", { name: "工作区", exact: true }).waitFor();
      const mainPID = application.process().pid;
      const backendPID = backendPIDs(mainPID)[0];
      assert.ok(backendPID, "host owns a live Go backend");
      process.kill(Number(backendPID), "SIGKILL");
      await page.waitForFunction(
        async () => (await window.desktop.getBackendConnection()).status === "failed",
      );
      await page.getByRole("button", { name: "重试", exact: true }).click();
      await page.getByRole("heading", { name: "工作区", exact: true }).waitFor();
      const rejection = await application.evaluate(async ({ BrowserWindow, ipcMain }) => {
        const main = BrowserWindow.getAllWindows()[0];
        const attacker = new BrowserWindow({
          show: false,
          webPreferences: { contextIsolation: true, sandbox: true },
        });
        try {
          await attacker.loadURL(main.webContents.getURL());
          try {
            await ipcMain._invokeHandlers.get("desktop:connection")({
              sender: attacker.webContents,
              senderFrame: attacker.webContents.mainFrame,
            });
            return "allowed";
          } catch (error) {
            return error.message;
          }
        } finally {
          attacker.destroy();
        }
      });
      assert.match(rejection, /untrusted_window/);
      assert.deepEqual(errors, []);
      // Native folder selection uses the existing workspace registration flow.
      const workspace = path.join(temporary, "工作区 with spaces");
      await mkdir(workspace);
      await writeFile(
        path.join(workspace, "one.manifest.toml"),
        'version = 2\n[workspace]\nid = "desktop-test"\nname = "桌面测试"\n',
      );
      await application.evaluate(({ dialog }, selected) => {
        dialog.showOpenDialog = async () => ({ canceled: false, filePaths: [selected] });
      }, workspace);
      await page.getByRole("button", { name: "打开工作区", exact: true }).click();
      await page.waitForURL(/#\/workspaces\//);
      await page.waitForFunction(() => document.body.innerText.includes("桌面测试"));
      await page.evaluate(() => {
        window.location.hash = "#/desktop";
      });
      await page.getByRole("heading", { name: "桌面设置", exact: true }).waitFor();
      await page.getByRole("button", { name: "安装 / 更新终端 CLI", exact: true }).click();
      await page.waitForFunction(() =>
        Array.from(document.querySelectorAll("button")).some(
          (button) => button.textContent?.trim() === "刷新" && !button.disabled,
        ),
      );
      const installed = path.join(
        temporary,
        "bin",
        process.platform === "win32" ? "one.exe" : "one",
      );
      const appInfo = await application.evaluate(({ app }) => ({
        packaged: app.isPackaged,
        version: app.getVersion(),
      }));
      const version = appInfo.packaged
        ? appInfo.version
        : JSON.parse(await readFile(path.join(root, "resources/bundle-manifest.json"), "utf8"))
            .version;
      assert.equal(execFileSync(installed, ["--version"], { encoding: "utf8" }).trim(), version);
      await page.getByRole("button", { name: "检查更新", exact: true }).click();
      await page.getByText(/开发构建|签名/).waitFor();
      await page.evaluate(() => {
        window.location.hash = "#/";
      });
      await page.getByRole("heading", { name: "工作区", exact: true }).waitFor();
      await page.screenshot({ path: path.join(tmpdir(), "one-electron-desktop.png") });
      assert.deepEqual(errors, []);
      const liveBackend = backendPIDs(mainPID)[0];
      await application.close();
      assert.throws(
        () => process.kill(liveBackend, 0),
        { code: "ESRCH" },
        "quitting must reap the backend",
      );
    } catch (error) {
      const active = application.windows()[0];
      if (active) {
        console.error(
          await active
            .locator("body")
            .innerText()
            .catch(() => ""),
        );
      }
      hostProcess.kill("SIGTERM");
      throw error;
    } finally {
      if (hostProcess.exitCode === null) await application.close();
      await rm(temporary, { recursive: true, force: true });
    }
  },
);
