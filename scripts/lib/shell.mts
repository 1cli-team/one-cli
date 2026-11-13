import { spawnSync } from "node:child_process";

export type ShellArgument = string | number | readonly (string | number)[];
type ShellOptions = {
  cwd?: string;
  env?: NodeJS.ProcessEnv;
  stdio?: "pipe" | "inherit";
  nothrow?: boolean;
};
type CommandResult = {
  stdout: string;
  stderr: string;
  exitCode: number | null;
  signal: NodeJS.Signals | null;
  ok: boolean;
};

export class CommandError extends Error {
  readonly result: CommandResult;

  constructor(result: CommandResult) {
    const reason = result.signal ?? result.exitCode;
    super(`Command failed (${reason}) / 命令执行失败\n${result.stderr}`.trim());
    this.name = "CommandError";
    this.result = result;
  }
}

function quote(value: string | number) {
  return "'" + String(value).replaceAll("'", "'\\''") + "'";
}

// Literal template parts are Bash syntax; each interpolated value is one
// argument, and arrays expand to separate arguments without shell evaluation.
export function shell({ nothrow = false, stdio = "pipe", ...options }: ShellOptions = {}) {
  return (pieces: TemplateStringsArray, ...args: ShellArgument[]): CommandResult => {
    let command = pieces[0];
    for (let index = 0; index < args.length; index++) {
      const arg = args[index];
      command += typeof arg === "object" ? arg.map(quote).join(" ") : quote(arg);
      command += pieces[index + 1];
    }
    const child = spawnSync("bash", ["-euo", "pipefail", "-c", command], {
      ...options,
      stdio,
      encoding: "utf8",
      maxBuffer: 64 * 1024 * 1024,
    });
    const result: CommandResult = {
      stdout: child.stdout ?? "",
      stderr: child.stderr || child.error?.message || "",
      exitCode: child.status,
      signal: child.signal,
      ok: !child.error && child.status === 0,
    };
    if (!nothrow && !result.ok) throw new CommandError(result);
    return result;
  };
}

export const $ = shell();
