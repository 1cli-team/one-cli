import { spawnSync } from "node:child_process";
import * as fs from "node:fs/promises";
import { createConnection } from "node:net";
import { homedir } from "node:os";
import path from "node:path";
import { messages } from "./dev-messages.mjs";

function socketReady(socket) {
  return new Promise((resolve) => {
    const connection = createConnection(socket);
    const finish = (ready) => {
      connection.destroy();
      resolve(ready);
    };
    connection.setTimeout(500, () => finish(false));
    connection.once("error", () => finish(false));
    connection.once("connect", () => finish(true));
  });
}

export function readSessionEnvironment(env, runtime, run = spawnSync) {
  // Query only the current user's service manager, then retain display keys.
  const result = run("/usr/bin/systemctl", ["--user", "show-environment"], {
    env: {
      ...env,
      XDG_RUNTIME_DIR: runtime,
      DBUS_SESSION_BUS_ADDRESS: `unix:path=${runtime}/bus`,
    },
    encoding: "utf8",
    timeout: 2000,
    maxBuffer: 1024 * 1024,
  });
  if (result.error || result.status !== 0) return {};
  const values = {};
  for (const line of result.stdout.split("\n")) {
    const match = /^(DISPLAY|WAYLAND_DISPLAY|XAUTHORITY)=(.*)$/.exec(line);
    // systemctl quotes unusual values. Ignore those and use socket discovery.
    if (match && !/^["']/.test(match[2])) values[match[1]] = match[2];
  }
  return values;
}

export async function detectDesktopEnvironment({
  env = process.env,
  platform = process.platform,
  uid = process.getuid?.(),
  home = env.HOME || homedir(),
  runtimeBase = "/run/user",
  x11Directory = "/tmp/.X11-unix",
  io = fs,
  connect = socketReady,
  sessionEnvironment = readSessionEnvironment,
  text = messages["en-US"],
  log = console.log,
} = {}) {
  const result = { ...env };
  // Explicit displays include SSH forwarding and Xvfb. Preserve their settings.
  if (platform !== "linux" || env.DISPLAY) return result;

  const owned = async (file, kind, allowRoot = false) => {
    try {
      const info = await io.stat(file);
      return (
        (info.uid === uid || (allowRoot && info.uid === 0)) && info[kind]()
      );
    } catch {
      return false;
    }
  };
  const alive = async (file, allowRoot = false) =>
    (await owned(file, "isSocket", allowRoot)) && (await connect(file));
  const entries = async (directory, pattern) => {
    try {
      return (await io.readdir(directory))
        .filter((name) => pattern.test(name))
        .toSorted();
    } catch {
      return [];
    }
  };
  const runtime = env.XDG_RUNTIME_DIR || path.join(runtimeBase, String(uid));
  const validRuntime = await owned(runtime, "isDirectory");
  if (!validRuntime && (env.XDG_RUNTIME_DIR || env.WAYLAND_DISPLAY))
    throw new Error(text.runtime(runtime));

  const desktop = validRuntime ? sessionEnvironment(env, runtime) : {};
  const waylandPath = (display) =>
    path.isAbsolute(display) ? display : path.join(runtime, display);
  const useWayland = (display) => {
    result.XDG_RUNTIME_DIR = runtime;
    result.WAYLAND_DISPLAY = display;
    result.XDG_SESSION_TYPE = "wayland";
    if (!env.WAYLAND_DISPLAY) log(text.desktop("Wayland", display));
    return result;
  };
  if (env.WAYLAND_DISPLAY) {
    if (!(await alive(waylandPath(env.WAYLAND_DISPLAY))))
      throw new Error(text.wayland(waylandPath(env.WAYLAND_DISPLAY)));
    return useWayland(env.WAYLAND_DISPLAY);
  }
  if (validRuntime) {
    if (
      desktop.WAYLAND_DISPLAY &&
      (await alive(waylandPath(desktop.WAYLAND_DISPLAY)))
    )
      return useWayland(desktop.WAYLAND_DISPLAY);
    const displays = [];
    for (const display of await entries(runtime, /^wayland-\d+$/)) {
      if (await alive(waylandPath(display))) displays.push(display);
    }
    if (displays.length > 1) throw new Error(text.ambiguous("WAYLAND_DISPLAY"));
    if (displays.length === 1) return useWayland(displays[0]);
  }

  // X11 requires the current user's authentication file as well as a socket.
  const authorities = [
    env.XAUTHORITY,
    desktop.XAUTHORITY,
    path.join(home, ".Xauthority"),
  ].filter(Boolean);
  let authority;
  for (const file of authorities) {
    if (await owned(file, "isFile")) {
      try {
        await io.access(file, fs.constants.R_OK);
        authority = file;
        break;
      } catch {
        // Keep looking for a readable session credential.
      }
    }
  }
  const x11Socket = (display) => {
    const match = /^:(\d+)(?:\.\d+)?$/.exec(display || "");
    return match ? path.join(x11Directory, `X${match[1]}`) : undefined;
  };
  if (authority) {
    let display;
    const preferred = x11Socket(desktop.DISPLAY);
    if (preferred && (await alive(preferred, true))) display = desktop.DISPLAY;
    else {
      const displays = [];
      for (const name of await entries(x11Directory, /^X\d+$/)) {
        if (await alive(path.join(x11Directory, name), true))
          displays.push(`:${name.slice(1)}`);
      }
      if (displays.length > 1) throw new Error(text.ambiguous("DISPLAY"));
      [display] = displays;
    }
    if (display) {
      result.DISPLAY = display;
      result.XAUTHORITY = authority;
      result.XDG_SESSION_TYPE = "x11";
      if (validRuntime) result.XDG_RUNTIME_DIR = runtime;
      log(text.desktop("X11", display));
      return result;
    }
  }
  throw new Error(text.headless);
}
