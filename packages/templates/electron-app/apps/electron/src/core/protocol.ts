import path from "node:path";
import { access, readFile } from "node:fs/promises";
import { URL } from "node:url";
import { protocol } from "electron";
import isDev from "electron-is-dev";
import mime from "mime-types";
import { __dirname, defaultScheme } from "../utils";

const create = (): void => {
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
};

export function createProtocolService() {
  return { create };
}

export type ProtocolService = ReturnType<typeof createProtocolService>;
