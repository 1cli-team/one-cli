import { BrowserWindow, type BrowserWindowConstructorOptions } from "electron";
import isDev from "electron-is-dev";

// 所有业务窗口继承这个基类
// 职责：封装 BrowserWindow 生命周期 + 提供 send() 快捷方法
// 子类只需要：设置 this.url、实现 init()，按需重写 readyToShow/windowClose 钩子
export default class Window {
  window: BrowserWindow | null = null;
  options: BrowserWindowConstructorOptions;
  url: string = "";

  constructor(options: BrowserWindowConstructorOptions) {
    this.options = options;
  }

  create(): BrowserWindow {
    if (!this.url) throw new Error("url is required");

    const window = new BrowserWindow(this.options);
    void window.loadURL(this.url);

    window.once("ready-to-show", this.readyToShow);
    window.on("close", this.windowClose);

    return window;
  }

  readyToShow = () => {
    if (!this.window) return;
    this.window.show();
    if (isDev) this.window.webContents.openDevTools();
  };

  windowClose = () => {
    if (!this.window) return;
    this.window = null;
  };

  send(channel: string, ...args: unknown[]): void {
    if (!this.window) return;
    this.window.webContents.send(channel, ...args);
  }
}
