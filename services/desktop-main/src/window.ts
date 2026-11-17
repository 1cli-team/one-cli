import { BrowserWindow, protocol, net, nativeTheme, type IpcMainInvokeEvent } from "electron";
import { resolve, relative, isAbsolute, sep, join } from "node:path";
import { pathToFileURL } from "node:url";
import { readLegacyPreferences } from "./legacy.ts";

export const APP_ORIGIN = "one-desktop://app";
export const DEV_ORIGIN = "http://127.0.0.1:5173";

export function trustedURL(value: string, development: boolean) {
  try {
    const url = new URL(value);
    if (url.username || url.password) return false;
    return development
      ? url.origin === DEV_ORIGIN
      : url.protocol === "one-desktop:" && url.hostname === "app" && url.port === "";
  } catch {
    return false;
  }
}

export function registerAppScheme() {
  protocol.registerSchemesAsPrivileged([
    {
      scheme: "one-desktop",
      privileges: { standard: true, secure: true, supportFetchAPI: true, corsEnabled: true },
    },
  ]);
}

export function createMainWindow({
  development,
  buildDirectory,
  assetsDirectory,
}: {
  development: boolean;
  buildDirectory: string;
  assetsDirectory: string;
}) {
  let window: BrowserWindow | undefined;
  let exiting = false;
  let legacyPreferences: ReturnType<typeof readLegacyPreferences> | undefined;
  const csp = [
    "default-src 'self'",
    // Vite injects an inline React refresh preamble in development only.
    `script-src 'self'${development ? " 'unsafe-inline'" : ""}`,
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data:",
    "font-src 'self' data:",
    "object-src 'none'",
    "base-uri 'self'",
    "frame-src 'none'",
    "form-action 'none'",
    `connect-src 'self' http://127.0.0.1:*${development ? " ws://127.0.0.1:5173" : ""}`,
  ].join("; ");

  async function init() {
    const rendererDirectory = resolve(buildDirectory, "renderer");
    if (!development) {
      protocol.handle("one-desktop", async (request) => {
        if (!trustedURL(request.url, false) || !["GET", "HEAD"].includes(request.method)) {
          return new Response(null, { status: 403 });
        }
        let pathname: string;
        try {
          pathname = decodeURIComponent(new URL(request.url).pathname);
        } catch {
          return new Response(null, { status: 400 });
        }
        const file = resolve(rendererDirectory, `.${pathname === "/" ? "/index.html" : pathname}`);
        const rel = relative(rendererDirectory, file);
        if (
          rel === ".." ||
          rel.startsWith(`..${sep}`) ||
          isAbsolute(rel) ||
          pathname.includes("\\")
        ) {
          return new Response(null, { status: 403 });
        }
        try {
          const result = await net.fetch(pathToFileURL(file).href);
          const headers = new Headers(result.headers);
          headers.set("Content-Security-Policy", csp);
          headers.set("X-Content-Type-Options", "nosniff");
          return new Response(request.method === "HEAD" ? null : result.body, {
            status: result.status,
            headers,
          });
        } catch {
          return new Response(null, { status: 404 });
        }
      });
    }
    window = new BrowserWindow({
      title: "One Desktop",
      width: 1280,
      height: 820,
      minWidth: 760,
      minHeight: 560,
      show: false,
      titleBarStyle: "hidden",
      titleBarOverlay: {
        height: 56,
        color: "#00000000",
        symbolColor: nativeTheme.shouldUseDarkColors ? "#ededf0" : "#242427",
      },
      trafficLightPosition: { x: 16, y: 20 },
      icon: join(assetsDirectory, "icon.png"),
      webPreferences: {
        preload: join(buildDirectory, "preload/index.cjs"),
        contextIsolation: true,
        nodeIntegration: false,
        sandbox: true,
        webSecurity: true,
      },
    });
    window.webContents.session.setPermissionRequestHandler((_contents, _permission, callback) =>
      callback(false),
    );
    window.webContents.session.setPermissionCheckHandler(() => false);
    window.webContents.setWindowOpenHandler(() => ({ action: "deny" }));
    window.webContents.on("will-attach-webview", (event) => event.preventDefault());
    window.webContents.on("will-navigate", (event, url) => {
      if (!trustedURL(url, development)) event.preventDefault();
    });
    window.webContents.on("will-redirect", (event, url) => {
      if (!trustedURL(url, development)) event.preventDefault();
    });
    if (development) {
      window.webContents.session.webRequest.onHeadersReceived((details, callback) => {
        if (!trustedURL(details.url, true)) return callback({});
        callback({
          responseHeaders: { ...details.responseHeaders, "Content-Security-Policy": [csp] },
        });
      });
    }
    window.on("close", (event) => {
      if (!exiting) {
        event.preventDefault();
        window?.hide();
      }
    });
    window.once("ready-to-show", () => window?.show());
    await window.loadURL(development ? DEV_ORIGIN : `${APP_ORIGIN}/index.html`);
  }

  return {
    init,
    getLegacyPreferences: () => (legacyPreferences ??= readLegacyPreferences()),
    get window() {
      return window;
    },
    show() {
      if (window?.isMinimized()) window.restore();
      window?.show();
      window?.focus();
    },
    setTheme(theme: "light" | "dark") {
      nativeTheme.themeSource = theme;
      if (process.platform !== "darwin") {
        window?.setTitleBarOverlay({
          symbolColor: theme === "dark" ? "#ededf0" : "#242427",
        });
      }
    },
    permitExit() {
      exiting = true;
    },
    assertTrusted(event: IpcMainInvokeEvent) {
      const contents = window?.webContents;
      if (
        !contents ||
        event.sender !== contents ||
        !event.senderFrame ||
        event.senderFrame !== contents.mainFrame ||
        !trustedURL(event.senderFrame.url, development) ||
        event.senderFrame.origin !== (development ? DEV_ORIGIN : APP_ORIGIN)
      ) {
        throw new Error("untrusted_window");
      }
    },
  };
}

export type MainWindow = ReturnType<typeof createMainWindow>;
