import * as fs from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  detectDesktopEnvironment,
  readSessionEnvironment,
} from "../scripts/desktop-env.mjs";
import { developmentMessages, messages } from "../scripts/dev-messages.mjs";
import {
  sandboxProfile,
  sandboxProfileName,
} from "../scripts/sandbox-profile.mjs";
import {
  checkUbuntuSandbox,
  isUbuntu,
  probeUserNamespace,
  shellQuote,
} from "../scripts/ubuntu-sandbox.mjs";

const cleanup = [];
afterEach(async () => {
  for (const dispose of cleanup.splice(0).toReversed()) await dispose();
});

async function temporary() {
  const root = await fs.mkdtemp(path.join(tmpdir(), "one-desktop-"));
  cleanup.push(() => fs.rm(root, { recursive: true, force: true }));
  return root;
}

function sandboxOptions({
  release = 'ID="ubuntu"\nID_LIKE=debian',
  restricted = "1",
  clone = "1",
  helper,
} = {}) {
  const settings = {
    "/etc/os-release": release,
    "/proc/sys/kernel/apparmor_restrict_unprivileged_userns": restricted,
    "/proc/sys/kernel/unprivileged_userns_clone": clone,
  };
  return {
    binary: "/workspace/node_modules/electron/dist/electron",
    platform: "linux",
    env: { LANG: "C" },
    probe: vi.fn(() => ({ status: 0 })),
    io: {
      ...fs,
      readFile: vi.fn(async (file) => {
        if (!(file in settings)) throw new Error("missing");
        return settings[file];
      }),
      stat: vi.fn(async () => {
        if (!helper) throw new Error("missing");
        return { isFile: () => true, uid: 0, mode: 0o104755, ...helper };
      }),
      mkdtemp: vi.fn(fs.mkdtemp),
    },
  };
}

describe("Ubuntu sandbox preflight", () => {
  it("matches Ubuntu's ID rather than derivatives, descriptions, or shell expressions", () => {
    for (const release of ["ID=ubuntu\n", 'ID="ubuntu"', "ID='ubuntu'"])
      expect(isUbuntu(release)).toBe(true);
    for (const release of [
      'ID=linuxmint\nID_LIKE="ubuntu debian"',
      'NAME="Ubuntu"\nID=debian',
      "ID=$(echo ubuntu)",
      "ID=ubuntu-test",
    ])
      expect(isUbuntu(release)).toBe(false);
  });
  it.each(["darwin", "win32"])(
    "never accesses Linux configuration on %s",
    async (platform) => {
      const options = sandboxOptions();
      await checkUbuntuSandbox({ ...options, platform });
      expect(options.io.readFile).not.toHaveBeenCalled();
      expect(options.probe).not.toHaveBeenCalled();
    },
  );
  it("skips non-Ubuntu systems and kernels without the AppArmor restriction", async () => {
    for (const settings of [
      { release: "ID=fedora\nID_LIKE=ubuntu" },
      { restricted: "0" },
      { restricted: undefined },
    ]) {
      const options = sandboxOptions(settings);
      if (settings.restricted === undefined && "restricted" in settings)
        options.io.readFile.mockImplementation(async (file) => {
          if (file === "/etc/os-release") return "ID=ubuntu";
          throw new Error("missing");
        });
      await checkUbuntuSandbox(options);
      expect(options.probe).not.toHaveBeenCalled();
      expect(options.io.mkdtemp).not.toHaveBeenCalled();
    }
  });
  it("accepts a correctly installed SUID helper but rejects user-owned or writable helpers", async () => {
    const options = sandboxOptions({ helper: {} });
    await checkUbuntuSandbox(options);
    expect(options.probe).not.toHaveBeenCalled();
    for (const helper of [
      { uid: 1000 },
      { mode: 0o104777 },
      { mode: 0o100755 },
    ]) {
      const unsafe = sandboxOptions({ helper });
      await checkUbuntuSandbox(unsafe);
      expect(unsafe.probe).toHaveBeenCalledOnce();
    }
  });
  it("checks actual permissions each time without generating profiles once configured", async () => {
    const options = sandboxOptions();
    await checkUbuntuSandbox(options);
    await checkUbuntuSandbox(options);
    expect(options.probe).toHaveBeenCalledTimes(2);
    expect(options.probe).toHaveBeenCalledWith(options.binary, options.env);
    expect(options.io.mkdtemp).not.toHaveBeenCalled();
  });
  it.each(["en-US", "zh-CN"])(
    "prepares a private exact-binary profile and manual installation guidance in %s",
    async (locale) => {
      const root = await temporary();
      const options = sandboxOptions();
      options.binary = "/workspace/a 'quoted' [project]/electron";
      options.probe.mockReturnValue({
        status: 1,
        stderr: "unshare: Operation not permitted",
      });
      let failure;
      try {
        await checkUbuntuSandbox({
          ...options,
          text: messages[locale],
          temporaryDirectory: root,
        });
      } catch (error) {
        failure = error;
      }
      const [directory] = await fs.readdir(root);
      const profile = path.join(
        root,
        directory,
        sandboxProfileName(options.binary),
      );
      expect(await fs.readFile(profile, "utf8")).toBe(
        sandboxProfile(options.binary),
      );
      expect((await fs.stat(profile)).mode & 0o777).toBe(0o600);
      expect(failure.message).toContain(profile);
      expect(failure.message).toContain(
        "sudo install -o root -g root -m 644 --",
      );
      expect(failure.message).toContain("sudo apparmor_parser -r");
      expect(failure.message).toContain(
        locale === "zh-CN" ? "重新运行开发任务" : "rerun the development task",
      );
      expect(failure.message).not.toContain("--no-sandbox");
      expect(options.probe).toHaveBeenCalledOnce();
    },
  );
  it("does not misdiagnose disabled namespaces or a failed probe as missing AppArmor configuration", async () => {
    const disabled = sandboxOptions({ clone: "0" });
    await expect(checkUbuntuSandbox(disabled)).rejects.toThrow(
      "disabled unprivileged",
    );
    expect(disabled.probe).not.toHaveBeenCalled();
    const options = sandboxOptions();
    options.probe.mockReturnValue({ status: null, error: "unshare ENOENT" });
    await expect(checkUbuntuSandbox(options)).rejects.toThrow("unshare ENOENT");
    options.probe.mockImplementation(() => {
      throw new Error("Electron ENOENT");
    });
    await expect(checkUbuntuSandbox(options)).rejects.toThrow(
      "Electron ENOENT",
    );
    expect(options.io.mkdtemp).not.toHaveBeenCalled();
  });
  it("uses Electron's profile for the namespace probe without opening a GUI", () => {
    const run = vi.fn(() => ({
      status: 0,
      stdout: '{"status":0}',
      stderr: "",
    }));
    expect(
      probeUserNamespace("/path/electron", { TEST: "preserved" }, run),
    ).toEqual({ status: 0 });
    const [binary, args, options] = run.mock.calls[0];
    expect(binary).toBe("/path/electron");
    expect(args[0]).toBe("--eval");
    expect(args[1]).toContain("'/usr/bin/unshare'");
    expect(args[1]).toContain("'--mount'");
    expect(options.env).toEqual({
      TEST: "preserved",
      ELECTRON_RUN_AS_NODE: "1",
    });
    expect(options.shell).toBeUndefined();
    expect(shellQuote("/tmp/a'b $HOME `command`")).toBe(
      "'/tmp/a'\\''b $HOME `command`'",
    );
  });
});

async function desktop() {
  const root = await temporary();
  const uid = process.getuid?.() ?? 1000;
  const runtimeBase = path.join(root, "users");
  const runtime = path.join(runtimeBase, String(uid));
  const home = path.join(root, "home");
  const x11Directory = path.join(root, "x11");
  for (const dir of [runtime, home, x11Directory])
    await fs.mkdir(dir, { recursive: true });
  const sockets = new Map();
  const options = {
    env: {},
    platform: "linux",
    uid,
    home,
    runtimeBase,
    x11Directory,
    sessionEnvironment: vi.fn(() => ({})),
    log: vi.fn(),
    io: {
      ...fs,
      stat: async (file) => {
        const info = await fs.stat(file);
        info.uid = uid;
        if (sockets.has(file)) info.isSocket = () => true;
        return info;
      },
    },
    connect: vi.fn(async (file) => sockets.get(file) === true),
  };
  const socket = async (file, active = true) => {
    await fs.writeFile(file, "socket fixture");
    sockets.set(file, active);
  };
  return { options, runtime, home, x11Directory, socket };
}

describe("desktop discovery", () => {
  it("queries only the current user's bus and retains only display variables", () => {
    const run = vi.fn(() => ({
      status: 0,
      stdout:
        "DISPLAY=:0\nWAYLAND_DISPLAY=wayland-0\nXAUTHORITY=/run/user/42/auth\nSECRET=private\n",
    }));
    expect(readSessionEnvironment({}, "/run/user/42", run)).toEqual({
      DISPLAY: ":0",
      WAYLAND_DISPLAY: "wayland-0",
      XAUTHORITY: "/run/user/42/auth",
    });
    expect(run.mock.calls[0][2].env.DBUS_SESSION_BUS_ADDRESS).toBe(
      "unix:path=/run/user/42/bus",
    );
    run.mockReturnValue({ status: 1 });
    expect(readSessionEnvironment({}, "/run/user/42", run)).toEqual({});
  });

  it.each(["darwin", "win32"])(
    "preserves all variables and skips discovery on %s",
    async (platform) => {
      const io = { stat: vi.fn() };
      const env = { TEST: "preserved" };
      expect(await detectDesktopEnvironment({ env, platform, io })).toEqual(
        env,
      );
      expect(io.stat).not.toHaveBeenCalled();
    },
  );
  it.each(["localhost:10.0", ":99", ":0"])(
    "preserves explicit display %s including SSH forwarding and Xvfb",
    async (DISPLAY) => {
      const sessionEnvironment = vi.fn();
      const env = {
        DISPLAY,
        XAUTHORITY: "/forwarded/auth",
        XDG_SESSION_TYPE: "x11",
        WAYLAND_DISPLAY: "stale-wayland",
        SSH_CONNECTION: "remote",
        OTHER: "value",
      };
      expect(
        await detectDesktopEnvironment({
          env,
          platform: "linux",
          sessionEnvironment,
        }),
      ).toEqual(env);
      expect(sessionEnvironment).not.toHaveBeenCalled();
    },
  );
  it("recovers a live current-user Wayland display and tells SSH users where windows appear", async () => {
    const fixture = await desktop();
    await fixture.socket(path.join(fixture.runtime, "wayland-3"));
    fixture.options.env = { SSH_CONNECTION: "remote", OTHER: "preserved" };
    const result = await detectDesktopEnvironment(fixture.options);
    expect(result).toMatchObject({
      WAYLAND_DISPLAY: "wayland-3",
      XDG_RUNTIME_DIR: fixture.runtime,
      XDG_SESSION_TYPE: "wayland",
      OTHER: "preserved",
    });
    expect(fixture.options.env).not.toHaveProperty("WAYLAND_DISPLAY");
    expect(fixture.options.log).toHaveBeenCalledWith(
      expect.stringContaining("windows will appear on that desktop"),
    );
  });
  it("fills runtime and session type for an explicit Wayland display", async () => {
    const fixture = await desktop();
    const socket = path.join(fixture.runtime, "wayland-2");
    await fixture.socket(socket);
    fixture.options.env = { WAYLAND_DISPLAY: socket };
    expect(await detectDesktopEnvironment(fixture.options)).toMatchObject({
      WAYLAND_DISPLAY: socket,
      XDG_SESSION_TYPE: "wayland",
      XDG_RUNTIME_DIR: fixture.runtime,
    });
    expect(fixture.options.log).not.toHaveBeenCalled();
  });
  it("chooses the service manager's live Wayland display when multiple sockets exist", async () => {
    const fixture = await desktop();
    for (const name of ["wayland-0", "wayland-1"])
      await fixture.socket(path.join(fixture.runtime, name));
    fixture.options.sessionEnvironment.mockReturnValue({
      WAYLAND_DISPLAY: "wayland-1",
      SECRET: "never copy",
    });
    const result = await detectDesktopEnvironment(fixture.options);
    expect(result.WAYLAND_DISPLAY).toBe("wayland-1");
    expect(result).not.toHaveProperty("SECRET");
    fixture.options.sessionEnvironment.mockReturnValue({});
    await expect(detectDesktopEnvironment(fixture.options)).rejects.toThrow(
      "Set WAYLAND_DISPLAY",
    );
  });
  it("ignores lock files, regular files, and stale display sockets", async () => {
    const fixture = await desktop();
    await fs.writeFile(path.join(fixture.runtime, "wayland-0"), "not a socket");
    await fs.writeFile(path.join(fixture.runtime, "wayland-1.lock"), "lock");
    await fixture.socket(path.join(fixture.runtime, "wayland-2"), false);
    await expect(detectDesktopEnvironment(fixture.options)).rejects.toThrow(
      "No desktop display",
    );
    fixture.options.env = { WAYLAND_DISPLAY: "wayland-0" };
    await expect(detectDesktopEnvironment(fixture.options)).rejects.toThrow(
      "Wayland display is unavailable",
    );
  });
  it("rejects runtime directories and sockets owned by another user", async () => {
    const fixture = await desktop();
    await fixture.socket(path.join(fixture.runtime, "wayland-0"));
    fixture.options.env = { XDG_RUNTIME_DIR: fixture.runtime };
    fixture.options.uid++;
    await expect(detectDesktopEnvironment(fixture.options)).rejects.toThrow(
      "belongs to another user",
    );
    fixture.options.uid--;
    const io = fixture.options.io;
    fixture.options.io = {
      ...io,
      stat: async (file) => {
        const info = await io.stat(file);
        if (file.endsWith("wayland-0")) info.uid++;
        return info;
      },
    };
    await expect(detectDesktopEnvironment(fixture.options)).rejects.toThrow(
      "No desktop display",
    );
  });
  it("recovers X11 with the session's current-user authentication file", async () => {
    const fixture = await desktop();
    const auth = path.join(fixture.runtime, "session-auth");
    await fs.writeFile(auth, "cookie");
    await fixture.socket(path.join(fixture.x11Directory, "X2"));
    fixture.options.sessionEnvironment.mockReturnValue({
      DISPLAY: ":2.0",
      XAUTHORITY: auth,
    });
    expect(await detectDesktopEnvironment(fixture.options)).toMatchObject({
      DISPLAY: ":2.0",
      XAUTHORITY: auth,
      XDG_SESSION_TYPE: "x11",
    });
  });
  it("uses a unique X11 socket with .Xauthority, and refuses ambiguous or unauthenticated displays", async () => {
    const fixture = await desktop();
    const auth = path.join(fixture.home, ".Xauthority");
    await fixture.socket(path.join(fixture.x11Directory, "X7"));
    await expect(detectDesktopEnvironment(fixture.options)).rejects.toThrow(
      "No desktop display",
    );
    await fs.writeFile(auth, "cookie");
    expect(await detectDesktopEnvironment(fixture.options)).toMatchObject({
      DISPLAY: ":7",
      XAUTHORITY: auth,
    });
    await fixture.socket(path.join(fixture.x11Directory, "X8"));
    await expect(detectDesktopEnvironment(fixture.options)).rejects.toThrow(
      "Set DISPLAY",
    );
  });
});

describe("development guidance languages", () => {
  it("keeps both catalogs and parameter counts in sync", () => {
    expect(Object.keys(messages["zh-CN"]).toSorted()).toEqual(
      Object.keys(messages["en-US"]).toSorted(),
    );
    for (const key of Object.keys(messages["en-US"])) {
      expect(typeof messages["zh-CN"][key]).toBe(typeof messages["en-US"][key]);
      expect(messages["zh-CN"][key].length).toBeGreaterThan(0);
      if (typeof messages["en-US"][key] === "function")
        expect(messages["zh-CN"][key].length).toBe(
          messages["en-US"][key].length,
        );
    }
  });
  it("reads One's existing language preference each time and honors the auto-locale order", async () => {
    const config = await temporary();
    await fs.mkdir(path.join(config, "one"));
    const env = {
      XDG_CONFIG_HOME: config,
      LC_ALL: "zh_CN.UTF-8",
      LC_MESSAGES: "en_US.UTF-8",
    };
    expect(await developmentMessages(env)).toBe(messages["zh-CN"]);
    const file = path.join(config, "one/preferences.json");
    await fs.writeFile(file, JSON.stringify({ locale: "en-US" }));
    expect(await developmentMessages(env)).toBe(messages["en-US"]);
    await fs.writeFile(file, JSON.stringify({ locale: "zh-CN" }));
    expect(await developmentMessages({ ...env, LC_ALL: "C" })).toBe(
      messages["zh-CN"],
    );
    await fs.writeFile(file, JSON.stringify({ locale: "auto" }));
    expect(await developmentMessages({ ...env, LC_ALL: "C" })).toBe(
      messages["en-US"],
    );
  });
});
