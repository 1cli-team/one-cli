import { EventEmitter } from "node:events";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { IpcMainInvokeEvent } from "electron";
import { createMainWindow } from "../src/windows/main.window";
import { createAppIPC } from "../src/ipc";
import { createMainContainer } from "../src/container";
import { IPC } from "../src/types/events";

const mocks = vi.hoisted(() => ({
  handles: new Map<string, (...args: unknown[]) => unknown>(),
  removeHandler: vi.fn(),
  expose: vi.fn(),
  invoke: vi.fn(),
  log: { info: vi.fn(), error: vi.fn() },
}));
vi.mock("../src/utils", () => ({
  preloadPath: "/app/preload/index.cjs",
  rendererPath: "/app with spaces/renderer/index.html",
}));
vi.mock("electron-log/main", () => ({ default: mocks.log }));
vi.mock("electron", async () => {
  const { EventEmitter } = await import("node:events");
  return {
    app: Object.assign(new EventEmitter(), {
      getName: () => "desktop",
      getVersion: () => "0.1.0",
      quit: vi.fn(),
    }),
    ipcMain: {
      handle: (channel: string, handler: (...args: unknown[]) => unknown) =>
        mocks.handles.set(channel, handler),
      removeHandler: mocks.removeHandler,
    },
    contextBridge: { exposeInMainWorld: mocks.expose },
    ipcRenderer: { invoke: mocks.invoke },
    BrowserWindow: class extends EventEmitter {
      webContents = Object.assign(new EventEmitter(), {
        mainFrame: { url: "file:///app%20with%20spaces/renderer/index.html" },
        setWindowOpenHandler: vi.fn(),
      });
      loadURL = vi.fn(async () => {});
      loadFile = vi.fn(async () => {});
      show = vi.fn();
      focus = vi.fn();
      restore = vi.fn();
      isMinimized = vi.fn(() => true);
      isDestroyed = vi.fn(() => false);
      destroy = vi.fn();
    },
  };
});
beforeEach(() => {
  vi.clearAllMocks();
  mocks.handles.clear();
  vi.stubEnv("NODE_ENV", "production");
});

describe("main window", () => {
  it("loads the production HTML file, protects navigation, and reuses its window", async () => {
    const service = createMainWindow();
    await service.init();
    const window = service.window!;
    expect(window.loadFile).toHaveBeenCalledWith(
      "/app with spaces/renderer/index.html",
    );
    expect(window.loadURL).not.toHaveBeenCalled();
    const preventDefault = vi.fn();
    (window.webContents as unknown as EventEmitter).emit("will-navigate", {
      preventDefault,
    });
    expect(preventDefault).toHaveBeenCalledOnce();
    const handler = vi.mocked(window.webContents.setWindowOpenHandler).mock
      .calls[0][0];
    expect(handler({} as never)).toEqual({ action: "deny" });
    await service.init();
    expect(window.restore).toHaveBeenCalledOnce();
    expect(window.focus).toHaveBeenCalledOnce();
    expect(service.window).toBe(window);
    (window as unknown as EventEmitter).emit("closed");
    expect(service.window).toBeNull();
  });
  it("loads Vite in development", async () => {
    vi.stubEnv("NODE_ENV", "development");
    const service = createMainWindow();
    await service.init();
    expect(service.window!.loadURL).toHaveBeenCalledWith(
      "http://localhost:5173/",
    );
    expect(service.window!.loadFile).not.toHaveBeenCalled();
  });
});
describe("isolated app information bridge", () => {
  it("accepts only its window's trusted main frame and no arguments", async () => {
    const mainWindow = createMainWindow();
    await mainWindow.init();
    const ipc = createAppIPC({ mainWindow });
    ipc.init();
    const contents = mainWindow.window!.webContents;
    const event = {
      sender: contents,
      senderFrame: contents.mainFrame,
    } as IpcMainInvokeEvent;
    const handler = mocks.handles.get(IPC.appInfo)!;
    expect(handler(event)).toEqual({
      name: "desktop",
      version: "0.1.0",
      platform: process.platform,
    });
    for (const bad of [
      { ...event, sender: {} },
      { ...event, senderFrame: { url: contents.mainFrame.url } },
      { ...event, senderFrame: null },
    ])
      expect(() => handler(bad)).toThrow("Unauthorized");
    expect(() => handler(event, "extra")).toThrow("Unauthorized");
    const frame = contents.mainFrame as unknown as { url: string };
    frame.url = "https://untrusted.example/";
    expect(() => handler(event)).toThrow("Unauthorized");
    ipc.dispose();
    expect(mocks.removeHandler).toHaveBeenCalledWith(IPC.appInfo);
  });
  it("exposes a fixed API without raw channel access", async () => {
    const { createRequire } = await import("node:module");
    const preloadPackage = "@one-template-electron/preload";
    const entry = createRequire(import.meta.url).resolve(preloadPackage);
    await import(entry.replace(/dist[/\\]index\.cjs$/, "src/index.ts"));
    const [name, api] = mocks.expose.mock.calls[0];
    expect(name).toBe("electron");
    expect(Object.keys(api)).toEqual(["getAppInfo"]);
    await api.getAppInfo();
    expect(mocks.invoke).toHaveBeenCalledWith(IPC.appInfo);
  });
  it("disposes IPC and lifecycle hooks through Awilix", async () => {
    const { app } = await import("electron");
    const container = createMainContainer();
    const application = container.resolve("application");
    await application.init();
    expect(app.listenerCount("activate")).toBe(1);
    await container.dispose();
    expect(app.listenerCount("activate")).toBe(0);
    expect(app.listenerCount("second-instance")).toBe(0);
    expect(mocks.removeHandler).toHaveBeenCalledWith(IPC.appInfo);
  });
});
