import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type Mock,
} from "vitest";
import type { IpcMainInvokeEvent, Rectangle } from "electron";
import { createMainContainer } from "../src/container";
import { createElectronControllerBinder } from "../src/core/electronBinder";
import { registerControllerHandlers } from "../src/core/registerControllerHandlers";
import { createProtocolService } from "../src/core/protocol";
import { createMainWindow } from "../src/windows/main.window";
import { createElectronLogger } from "../src/vendor/ElectronLogger";
import { createElectronStore } from "../src/vendor/ElectronStore";
import { createElectronUpdater } from "../src/vendor/ElectronUpdater";
import { createElectronDevtools } from "../src/vendor/ElectronDevtools";
import {
  createContextMenuController,
  createDialogController,
  createShellController,
} from "../src/controller";
import { EVENT, IPC } from "../src/types/events";

interface MockWindow {
  show: ReturnType<typeof vi.fn>;
  loadURL: ReturnType<typeof vi.fn>;
  getBounds: Mock<() => Rectangle>;
  webContents: {
    send: ReturnType<typeof vi.fn>;
    openDevTools: ReturnType<typeof vi.fn>;
  };
  once: ReturnType<typeof vi.fn>;
  on: ReturnType<typeof vi.fn>;
  listeners: Map<string, () => void>;
}

const mocks = vi.hoisted(() => ({
  isDev: false,
  windows: [] as MockWindow[],
  windowOptions: [] as unknown[],
  appOn: vi.fn<(event: string, listener: () => void) => void>(),
  getAllWindows: vi.fn(() => []),
  protocolHandle: vi.fn(),
  ipcHandle: vi.fn(),
  ipcOn: vi.fn(),
  storeGet: vi.fn(),
  storeSet: vi.fn(),
  access: vi.fn(),
  readFile: vi.fn(),
  writeFile: vi.fn(),
  mkdirSync: vi.fn(),
  showOpenDialog: vi.fn(),
  showSaveDialog: vi.fn(),
  openExternal: vi.fn(),
  openPath: vi.fn(),
  buildMenu: vi.fn(),
  installDevtools: vi.fn(),
  log: {
    transports: {
      file: { level: "", resolvePathFn: () => "", format: "", maxSize: 0 },
      console: { level: "", format: "" },
    },
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
    debug: vi.fn(),
  },
  updaterListeners: new Map<string, (...args: unknown[]) => void>(),
  updater: {
    disableWebInstaller: false,
    allowPrerelease: false,
    forceDevUpdateConfig: false,
    autoDownload: false,
    logger: {} as unknown,
    on: vi.fn(),
    checkForUpdates: vi.fn(),
    downloadUpdate: vi.fn(),
    quitAndInstall: vi.fn(),
  },
}));

vi.mock("electron", () => ({
  app: { on: mocks.appOn },
  BrowserWindow: Object.assign(
    vi.fn(function (options: unknown) {
      const listeners = new Map<string, () => void>();
      const window: MockWindow = {
        show: vi.fn(),
        loadURL: vi.fn().mockResolvedValue(undefined),
        getBounds: vi.fn(() => ({ width: 900, height: 600, x: 20, y: 30 })),
        webContents: { send: vi.fn(), openDevTools: vi.fn() },
        once: vi.fn((event: string, callback: () => void) =>
          listeners.set(event, callback),
        ),
        on: vi.fn((event: string, callback: () => void) =>
          listeners.set(event, callback),
        ),
        listeners,
      };
      mocks.windows.push(window);
      mocks.windowOptions.push(options);
      return window;
    }),
    { getAllWindows: mocks.getAllWindows },
  ),
  protocol: { handle: mocks.protocolHandle },
  ipcMain: { handle: mocks.ipcHandle, on: mocks.ipcOn },
  dialog: {
    showOpenDialog: mocks.showOpenDialog,
    showSaveDialog: mocks.showSaveDialog,
  },
  shell: { openExternal: mocks.openExternal, openPath: mocks.openPath },
  Menu: { buildFromTemplate: mocks.buildMenu },
}));
vi.mock("electron-is-dev", () => ({
  get default() {
    return mocks.isDev;
  },
}));
vi.mock("electron-log", () => ({ default: mocks.log }));
vi.mock("electron-store", () => ({
  default: vi.fn(function () {
    return { get: mocks.storeGet, set: mocks.storeSet };
  }),
}));
vi.mock("electron-updater", () => ({
  default: { autoUpdater: mocks.updater },
}));
vi.mock("electron-devtools-installer", () => ({
  default: mocks.installDevtools,
  REACT_DEVELOPER_TOOLS: "react",
  REDUX_DEVTOOLS: "redux",
}));
vi.mock("node:fs", () => ({ mkdirSync: mocks.mkdirSync }));
vi.mock("node:fs/promises", () => ({
  default: { readFile: mocks.readFile, writeFile: mocks.writeFile },
  readFile: mocks.readFile,
  access: mocks.access,
}));
vi.mock("../src/utils", () => ({
  __dirname: "/app/build/main",
  workspace: "/app/user-data",
  appName: "test-app",
  defaultScheme: "myapp",
  preloadPath: "/app/preload/index.cjs",
}));

beforeEach(() => {
  vi.clearAllMocks();
  vi.useFakeTimers();
  mocks.isDev = false;
  mocks.windows.length = 0;
  mocks.windowOptions.length = 0;
  mocks.updaterListeners.clear();
  mocks.storeGet.mockReturnValue(undefined);
  mocks.access.mockResolvedValue(undefined);
  mocks.readFile.mockResolvedValue(Buffer.from("<html>test</html>"));
  mocks.getAllWindows.mockReturnValue([]);
  mocks.updater.checkForUpdates.mockResolvedValue(undefined);
  mocks.updater.downloadUpdate.mockResolvedValue(["update.exe"]);
  mocks.updater.on.mockImplementation(
    (event: string, listener: (...args: unknown[]) => void) => {
      mocks.updaterListeners.set(event, listener);
    },
  );
});

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.unstubAllEnvs();
});

const makeWindow = () => createMainWindow({ store: createElectronStore() });
const invokeEvent = {} as IpcMainInvokeEvent;

function captureIpc() {
  const handlers = new Map<string, (...args: unknown[]) => unknown>();
  const events = new Map<string, (...args: unknown[]) => unknown>();
  const binder = createElectronControllerBinder(
    {
      handle: (channel, listener) => {
        handlers.set(channel, listener);
      },
      on: (channel, listener) => {
        events.set(channel, listener);
      },
    },
    { error: mocks.log.error },
  );
  return { handlers, events, binder };
}

describe("Awilix composition", () => {
  it("resolves a single shared instance of each service and controller", () => {
    const container = createMainContainer();
    expect(container.resolve("application")).toBe(
      container.resolve("application"),
    );
    expect(container.resolve("logger")).toBe(container.resolve("logger"));
    expect(container.resolve("mainWindow")).toBe(
      container.resolve("mainWindow"),
    );
    expect(container.resolve("controllers")).toEqual([
      container.resolve("dialogController"),
      container.resolve("shellController"),
      container.resolve("contextMenuController"),
    ]);
    expect(mocks.windows).toHaveLength(0);
  });

  it("keeps startup order and passes the same window to IPC and update services", async () => {
    const container = createMainContainer();
    const order: string[] = [];
    vi.spyOn(container.resolve("protocol"), "create").mockImplementation(() => {
      order.push("protocol");
    });
    vi.spyOn(container.resolve("devTools"), "init").mockImplementation(
      async () => {
        order.push("devTools");
      },
    );
    vi.spyOn(container.resolve("updater"), "init").mockImplementation(
      async () => {
        order.push("updater");
      },
    );
    vi.spyOn(container.resolve("router"), "init").mockImplementation(() => {
      order.push("router");
    });
    await container.resolve("application").init();
    expect(order).toEqual(["protocol", "router", "devTools", "updater"]);
    expect(mocks.windows).toHaveLength(1);
    const firstWindow = mocks.windows[0];
    container.resolve("application").secondInstance();
    expect(mocks.windows).toHaveLength(1);
    expect(firstWindow.show).toHaveBeenCalledOnce();
    expect(mocks.log.info).toHaveBeenCalledWith("app ready");
    const activate = mocks.appOn.mock.calls.find(
      ([event]) => event === "activate",
    )?.[1];
    expect(activate).toBeDefined();
    activate?.();
    expect(firstWindow.show).toHaveBeenCalledTimes(2);
  });

  it("registers every existing controller channel without reflection", () => {
    createMainContainer().resolve("router").init();
    expect(mocks.ipcHandle.mock.calls.map(([channel]) => channel)).toEqual([
      IPC.dialog.open,
      IPC.dialog.save,
      IPC.shell.open,
      IPC.contextMenu.show,
    ]);
  });
});

describe("window lifecycle", () => {
  it("restores bounds and keeps the preload security options", () => {
    mocks.storeGet.mockReturnValue({ width: 900, height: 600, x: 20, y: 30 });
    const window = makeWindow();
    window.init();
    expect(mocks.windowOptions[0]).toMatchObject({
      width: 900,
      height: 600,
      x: 20,
      y: 30,
      webPreferences: {
        preload: "/app/preload/index.cjs",
        contextIsolation: true,
        nodeIntegration: false,
      },
    });
    expect(mocks.windows[0].loadURL).toHaveBeenCalledWith(
      "myapp://index.html/",
    );
    mocks.windows[0].listeners.get("ready-to-show")?.();
    expect(mocks.windows[0].show).toHaveBeenCalledOnce();
  });

  it("persists bounds before clearing the reference and can reopen the window", () => {
    const window = makeWindow();
    window.init();
    const current = mocks.windows[0];
    current.listeners.get("close")?.();
    expect(mocks.storeSet).toHaveBeenCalledWith(
      "mainBounds",
      current.getBounds(),
    );
    expect(window.window).toBe(current);
    current.listeners.get("closed")?.();
    expect(window.window).toBeNull();
    window.send("closed-window", "ignored");
    expect(current.webContents.send).not.toHaveBeenCalled();
    window.init();
    expect(mocks.windows).toHaveLength(2);
    window.send("new-window", 42);
    expect(mocks.windows[1].webContents.send).toHaveBeenCalledWith(
      "new-window",
      42,
    );
  });
});

describe("functional IPC controllers", () => {
  it("preserves successful invoke responses and event arguments", async () => {
    const { handlers, events, binder } = captureIpc();
    const eventHandler = vi.fn();
    registerControllerHandlers(
      [
        {
          handle: { [IPC.shell.open]: async () => undefined },
          on: { "example:event": eventHandler },
        },
      ],
      binder,
    );
    expect(
      await handlers.get(IPC.shell.open)?.(invokeEvent, "https://example.com"),
    ).toEqual({ code: 0, message: "success", data: null });
    await events.get("example:event")?.(invokeEvent, "payload", 42);
    expect(eventHandler).toHaveBeenCalledWith(invokeEvent, "payload", 42);
  });

  it.each([new Error("failed"), "failed"])(
    "wraps thrown failures and logs them: %s",
    async (failure) => {
      const { handlers, binder } = captureIpc();
      binder({
        event: "test",
        method: "handle",
        handler: async () => {
          throw failure;
        },
      });
      expect(await handlers.get("test")?.()).toEqual({
        code: -1,
        message: "failed",
        data: null,
      });
      expect(mocks.log.error).toHaveBeenCalledWith(
        "process ipc [test] failed: ",
        failure,
      );
    },
  );

  it("dispatches URLs and local paths through the existing shell channel", async () => {
    const { handlers, binder } = captureIpc();
    registerControllerHandlers([createShellController()], binder);
    await handlers.get(IPC.shell.open)?.(invokeEvent, "https://example.com");
    await handlers.get(IPC.shell.open)?.(invokeEvent, "/tmp/file.txt");
    expect(mocks.openExternal).toHaveBeenCalledWith("https://example.com");
    expect(mocks.openPath).toHaveBeenCalledWith("/tmp/file.txt");
  });

  it("uses the shared parent window for dialogs and returns read contents", async () => {
    const window = makeWindow();
    window.init();
    mocks.showOpenDialog.mockResolvedValue({
      canceled: false,
      filePaths: ["/tmp/example.txt"],
    });
    mocks.readFile.mockResolvedValue("example content");
    const { handlers, binder } = captureIpc();
    registerControllerHandlers(
      [createDialogController({ mainWindow: window })],
      binder,
    );
    expect(
      await handlers.get(IPC.dialog.open)?.(invokeEvent, {
        type: "file",
        multiple: true,
        readContent: true,
      }),
    ).toMatchObject({ code: 0, data: ["example content"] });
    expect(mocks.showOpenDialog).toHaveBeenCalledWith(window.window, {
      properties: ["openFile", "multiSelections"],
      filters: undefined,
    });
    mocks.showSaveDialog.mockResolvedValue({ canceled: true });
    expect(
      await handlers.get(IPC.dialog.save)?.(invokeEvent, {
        content: "ignored",
      }),
    ).toMatchObject({ code: 0, data: "" });
    expect(mocks.writeFile).not.toHaveBeenCalled();
  });

  it("reports an error if the parent window has not been created", async () => {
    const { handlers, binder } = captureIpc();
    registerControllerHandlers(
      [createDialogController({ mainWindow: makeWindow() })],
      binder,
    );
    expect(
      await handlers.get(IPC.dialog.open)?.(invokeEvent, { type: "directory" }),
    ).toMatchObject({ code: -1, message: "main window not ready" });
  });

  it.each([true, false])(
    "returns a selected context menu key or null on cancellation: %s",
    async (select) => {
      mocks.buildMenu.mockImplementation((items) => ({
        popup: ({ callback }: { callback: () => void }) => {
          if (select) items[0].click();
          callback();
        },
      }));
      const { handlers, binder } = captureIpc();
      registerControllerHandlers([createContextMenuController()], binder);
      expect(
        await handlers.get(IPC.contextMenu.show)?.(invokeEvent, [
          { label: "Open", key: "open" },
        ]),
      ).toMatchObject({ code: 0, data: select ? "open" : null });
    },
  );
});

describe("vendor and protocol services", () => {
  it("forwards update events and delays the startup check by 60 seconds", async () => {
    const window = makeWindow();
    window.init();
    const updater = createElectronUpdater({
      logger: createElectronLogger(),
      mainWindow: window,
    });
    await updater.init({ autoUpgrade: true, allowBeta: true });
    expect(mocks.updater.autoDownload).toBe(true);
    expect(mocks.updater.allowPrerelease).toBe(true);
    const progress = { percent: 50 };
    for (const [event, channel] of [
      ["checking-for-update", EVENT.update.checking],
      ["update-available", EVENT.update.available],
      ["update-not-available", EVENT.update.notAvailable],
      ["update-downloaded", EVENT.update.downloaded],
    ]) {
      mocks.updaterListeners.get(event)?.();
      expect(mocks.windows[0].webContents.send).toHaveBeenCalledWith(channel);
    }
    mocks.updaterListeners.get("download-progress")?.(progress);
    expect(mocks.windows[0].webContents.send).toHaveBeenCalledWith(
      EVENT.update.downloadProgress,
      progress,
    );
    await vi.advanceTimersByTimeAsync(59_999);
    expect(mocks.updater.checkForUpdates).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(mocks.updater.checkForUpdates).toHaveBeenCalledOnce();
    expect(await updater.startDownload()).toEqual(["update.exe"]);
    updater.install();
    expect(mocks.updater.quitAndInstall).toHaveBeenCalledOnce();
    updater.setAllowBeta(false);
    expect(mocks.updater.allowPrerelease).toBe(false);
  });

  it("logs update failures without rejecting the check", async () => {
    const error = new Error("offline");
    mocks.updater.checkForUpdates.mockRejectedValue(error);
    const updater = createElectronUpdater({
      logger: createElectronLogger(),
      mainWindow: makeWindow(),
    });
    await expect(updater.check()).resolves.toBeUndefined();
    expect(mocks.log.error).toHaveBeenCalledWith(
      "check for updates failed",
      error.stack,
    );
  });

  it("keeps DevTools opt-in and development-only", async () => {
    const devTools = createElectronDevtools({ logger: createElectronLogger() });
    vi.stubEnv("LOAD_DEVTOOLS", "1");
    await devTools.init();
    expect(mocks.installDevtools).not.toHaveBeenCalled();
    mocks.isDev = true;
    await devTools.init();
    expect(mocks.installDevtools).toHaveBeenCalledWith(["redux", "react"]);
  });

  it("normalizes error stacks and preserves the logging configuration", () => {
    const logger = createElectronLogger();
    const error = new Error("example");
    const { error: logError } = logger;
    logError("failure", error);
    expect(mocks.log.error).toHaveBeenCalledWith("failure", error.stack);
    expect(mocks.log.transports.file.maxSize).toBe(10 * 1024 * 1024);
    expect(mocks.log.transports.file.resolvePathFn()).toMatch(
      /\/app\/user-data\/logs\/.*-test-app\.log$/,
    );
  });

  it("serves production assets and retains the SPA fallback", async () => {
    createProtocolService().create();
    const [scheme, handler] = mocks.protocolHandle.mock.calls[0];
    expect(scheme).toBe("myapp");
    const response = await handler({ url: "myapp://index.html/example.js" });
    expect(mocks.readFile).toHaveBeenCalledWith(
      "/app/build/renderer/example.js",
    );
    expect(response.headers.get("Content-Type")).toContain("javascript");
    mocks.access.mockRejectedValueOnce(new Error("not found"));
    const fallback = await handler({ url: "myapp://index.html/route" });
    expect(mocks.readFile).toHaveBeenLastCalledWith(
      "/app/build/renderer/index.html",
    );
    expect(await fallback.text()).toBe("<html>test</html>");
  });
});
