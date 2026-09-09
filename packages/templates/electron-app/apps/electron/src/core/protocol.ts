import path from "node:path";
import { access, readFile } from "node:fs/promises";
import { URL } from "node:url";
import { provide } from "@inversifyjs/binding-decorators";
import { protocol } from "electron";
import isDev from "electron-is-dev";
import { injectable } from "inversify";
import mime from "mime-types";
import { __dirname, defaultScheme } from "../utils";

// 生产环境下把渲染端打包文件通过自定义协议提供给主窗口
// 开发环境直接 loadURL("http://localhost:xxxx")，这里空转
// SPA 路由 fallback：文件不存在返回 index.html
@injectable()
@provide()
export default class ProtocolService {
  create(): void {
    if (isDev) return;

    protocol.handle(defaultScheme, async (req) => {
      const pathName = new URL(req.url).pathname;
      let filePath = path.join(__dirname, "../renderer", pathName);
      try {
        await access(filePath);
      } catch {
        filePath = path.join(__dirname, "../renderer/index.html");
      }
      const mimeType = mime.lookup(filePath);
      const data = await readFile(filePath);
      return new Response(new Uint8Array(data), {
        headers: { "Content-Type": mimeType || "text/html" },
      });
    });
  }
}
