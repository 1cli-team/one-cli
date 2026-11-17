import { app, BrowserWindow, session } from "electron";
import { cp, mkdtemp, readdir, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  legacyRoots,
  preferenceKeys,
  readLegacyWebKit,
  validatePreferences,
  type DisplayPreferences,
} from "./preferences.ts";

export async function readLegacyPreferences(): Promise<DisplayPreferences> {
  // Test/development profiles must never import the developer's real settings.
  if (!app.isPackaged || process.env.ONE_DESKTOP_USER_DATA) return {};
  if (process.platform !== "win32") return readLegacyWebKit();
  const root = legacyRoots()[0];
  const temporary = await mkdtemp(join(tmpdir(), "one-webview-preferences-"));
  let window: BrowserWindow | undefined;
  try {
    const entries = await readdir(root, { withFileTypes: true });
    const profiles = [
      join(root, "EBWebView"),
      ...entries
        .filter((e) => e.isDirectory() && e.name.endsWith(".WebView2"))
        .map((e) => join(root, e.name)),
    ];
    for (const profile of profiles) {
      try {
        await cp(join(profile, "Default/Local Storage"), join(temporary, "Local Storage"), {
          recursive: true,
        });
        break;
      } catch {
        /* Try the next known WebView2 profile location. */
      }
    }
    const storage = session.fromPath(temporary, { cache: false });
    storage.protocol.handle("http", (request) =>
      new URL(request.url).origin === "http://tauri.localhost"
        ? new Response("<!doctype html><title>Preferences</title>", {
            headers: { "Content-Security-Policy": "default-src 'none'" },
          })
        : new Response(null, { status: 403 }),
    );
    window = new BrowserWindow({
      show: false,
      webPreferences: {
        session: storage,
        sandbox: true,
        contextIsolation: true,
        nodeIntegration: false,
      },
    });
    window.webContents.setWindowOpenHandler(() => ({ action: "deny" }));
    window.webContents.on("will-navigate", (event) => event.preventDefault());
    storage.setPermissionRequestHandler((_contents, _permission, callback) => callback(false));
    storage.setPermissionCheckHandler(() => false);
    await window.loadURL("http://tauri.localhost");
    const values = await window.webContents.executeJavaScript(
      `Object.fromEntries(${JSON.stringify(preferenceKeys)}.map(key => [key, localStorage.getItem(key)]))`,
    );
    window.destroy();
    window = undefined;
    await storage.closeAllConnections();
    return validatePreferences(values);
  } catch {
    return {};
  } finally {
    window?.destroy();
    await rm(temporary, { recursive: true, force: true }).catch(() => {});
  }
}
