import { readFile } from "node:fs/promises";
import { homedir } from "node:os";
import path from "node:path";

// Follow One's existing preference and auto-locale order without changing it.
export async function developmentMessages(env = process.env) {
  let locale;
  try {
    const config =
      env.XDG_CONFIG_HOME || path.join(env.HOME || homedir(), ".config");
    ({ locale } = JSON.parse(
      await readFile(path.join(config, "one/preferences.json"), "utf8"),
    ));
  } catch {
    // Missing/invalid preferences use the same auto fallback as One.
  }
  if (locale !== "zh-CN" && locale !== "en-US") {
    const systemLocale = env.LC_ALL || env.LC_MESSAGES || env.LANG || "";
    locale = /^zh/i.test(systemLocale) ? "zh-CN" : "en-US";
  }
  return messages[locale];
}

export const messages = {
  "en-US": {
    sandbox: (binary, profile, commands) =>
      `Ubuntu blocks Electron's sandbox permissions.\nElectron: ${binary}\nReview the generated AppArmor profile: ${profile}\nInstall it once with administrator privileges:\n\n${commands}\n\nThen rerun the development task. Repeat after Electron's binary path changes.`,
    probe: (reason) => `Could not check Ubuntu sandbox permissions: ${reason}`,
    userns:
      "Ubuntu has disabled unprivileged user namespaces. Ask your administrator to configure Electron sandbox support; an AppArmor profile alone cannot enable it.",
    runtime: (dir) =>
      `The desktop runtime directory is unavailable or belongs to another user: ${dir}. Run from your desktop terminal or set XDG_RUNTIME_DIR for your own session.`,
    wayland: (socket) =>
      `The Wayland display is unavailable: ${socket}. Check WAYLAND_DISPLAY and XDG_RUNTIME_DIR in your desktop terminal.`,
    ambiguous: (variable) =>
      `Multiple desktop displays are available. Set ${variable} from the desktop session you want to use.`,
    headless:
      "No desktop display was found for the current user. Run from a graphical desktop terminal, forward an X11 display over SSH, or use Xvfb for headless tests.",
    desktop: (type, display) =>
      `Using the current user's local ${type} desktop (${display}). Electron windows will appear on that desktop.`,
    renderer: (url) => `Renderer did not become ready at ${url}`,
    build: (code) => `Main build exited with code ${code}`,
  },
  "zh-CN": {
    sandbox: (binary, profile, commands) =>
      `Ubuntu 限制了 Electron 的沙箱权限。\nElectron：${binary}\n请审阅已生成的 AppArmor 规则：${profile}\n首次使用时，通过管理员权限执行：\n\n${commands}\n\n完成后重新运行开发任务。Electron 可执行文件路径变化后，需要重新配置。`,
    probe: (reason) => `无法检查 Ubuntu 沙箱权限：${reason}`,
    userns:
      "Ubuntu 已禁用非特权用户命名空间。请联系管理员配置 Electron 沙箱支持；仅安装 AppArmor 规则无法启用它。",
    runtime: (dir) =>
      `桌面运行目录不存在或属于其他用户：${dir}。请在桌面终端运行，或将 XDG_RUNTIME_DIR 设置为自己的会话目录。`,
    wayland: (socket) =>
      `Wayland 显示服务不可用：${socket}。请在桌面终端检查 WAYLAND_DISPLAY 和 XDG_RUNTIME_DIR。`,
    ambiguous: (variable) =>
      `发现多个桌面显示服务。请从目标桌面会话中获取并设置 ${variable}。`,
    headless:
      "未找到当前用户的桌面显示服务。请在图形桌面终端运行，通过 SSH 转发 X11 显示服务，或在无桌面的测试环境使用 Xvfb。",
    desktop: (type, display) =>
      `已识别当前用户的本机 ${type} 桌面（${display}）。Electron 窗口会显示在该桌面上。`,
    renderer: (url) => `Renderer 未能就绪：${url}`,
    build: (code) => `主进程构建失败，退出码：${code}`,
  },
};
