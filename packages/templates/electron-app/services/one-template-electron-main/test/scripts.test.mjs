import { mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { waitForRenderer } from "../scripts/dev.mjs";
import { prepareResources } from "../scripts/resources.mjs";

const cleanup = [];
afterEach(async () => {
  await Promise.all(cleanup.splice(0).map((dispose) => dispose()));
});

async function rendererServer() {
  let ready = false;
  let requests = 0;
  const server = createServer((_request, response) => {
    requests++;
    response.writeHead(ready ? 200 : 503);
    response.end();
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  cleanup.push(() => new Promise((resolve) => server.close(resolve)));
  return {
    url: `http://127.0.0.1:${server.address().port}/`,
    setReady: () => {
      ready = true;
    },
    requests: () => requests,
  };
}

describe("desktop development", () => {
  it("waits through unavailable responses before launching main", async () => {
    const server = await rendererServer();
    const timer = setTimeout(server.setReady, 150);
    try {
      await waitForRenderer(server.url);
      expect(server.requests()).toBeGreaterThan(1);
    } finally {
      clearTimeout(timer);
    }
  });
  it("reports an unavailable renderer and honors cancellation", async () => {
    const server = await rendererServer();
    await expect(waitForRenderer(server.url, { timeout: 100 })).rejects.toThrow(
      server.url,
    );
    const controller = new AbortController();
    const pending = waitForRenderer(server.url, { signal: controller.signal });
    controller.abort();
    await expect(pending).rejects.toThrow();
  });
});

describe("desktop packaging resources", () => {
  async function fixture() {
    const root = await mkdtemp(path.join(tmpdir(), "electron resources "));
    cleanup.push(() => rm(root, { recursive: true, force: true }));
    const mainDir = path.join(root, "services/desktop-main");
    const rendererDir = path.join(root, "apps/desktop-renderer/dist");
    const preloadEntry = path.join(
      root,
      "packages/desktop-preload/dist/index.cjs",
    );
    for (const dir of [
      path.join(mainDir, "build/main"),
      path.join(rendererDir, "assets"),
      path.dirname(preloadEntry),
    ]) {
      await mkdir(dir, { recursive: true });
    }
    await writeFile(path.join(mainDir, "build/main/index.js"), "main");
    await writeFile(path.join(rendererDir, "index.html"), "renderer");
    await writeFile(
      path.join(rendererDir, "assets/icon.bin"),
      Buffer.from([0, 255, 12]),
    );
    await writeFile(preloadEntry, "bridge");
    return { mainDir, rendererDir, preloadEntry };
  }
  it("assembles isolated copies and removes stale renderer assets", async () => {
    const files = await fixture();
    await prepareResources(files);
    const stale = path.join(files.mainDir, "build/renderer/stale.js");
    await writeFile(stale, "stale");
    await prepareResources(files);
    await expect(readFile(stale)).rejects.toThrow();
    expect(
      await readFile(
        path.join(files.mainDir, "build/renderer/assets/icon.bin"),
      ),
    ).toEqual(Buffer.from([0, 255, 12]));
    expect(
      await readFile(
        path.join(files.mainDir, "build/preload/index.cjs"),
        "utf8",
      ),
    ).toBe("bridge");
    expect(
      await readFile(path.join(files.rendererDir, "index.html"), "utf8"),
    ).toBe("renderer");
    expect(await readFile(files.preloadEntry, "utf8")).toBe("bridge");
  });
  it("keeps the existing assembly when an input build is missing", async () => {
    const files = await fixture();
    await prepareResources(files);
    await rm(files.preloadEntry);
    await expect(prepareResources(files)).rejects.toThrow();
    expect(
      await readFile(
        path.join(files.mainDir, "build/renderer/index.html"),
        "utf8",
      ),
    ).toBe("renderer");
  });
});
