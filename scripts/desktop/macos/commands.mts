import { retry } from "../../lib/retry.mts";
import { CommandError, shell } from "../../lib/shell.mts";
import type { ShellArgument } from "../../lib/shell.mts";

export const run = shell({ stdio: "inherit" });
export const capture = shell({ nothrow: true });
const privateExecution = shell();
const networkError =
  /A timestamp was expected but was not found|The network connection was lost|NSURLErrorDomain Code=-(?:1001|1003|1004|1005|1006|1009)\b/;

export function secret(pieces: TemplateStringsArray, ...args: ShellArgument[]) {
  try {
    return privateExecution(pieces, ...args).stdout.trim();
  } catch {
    // Arguments and captured output may contain credentials. Never attach the
    // original error as a cause or include its message.
    const program = pieces[0].trim().split(/\s+/, 1)[0];
    throw new Error(`${program} failed / 执行失败 (credential output withheld / 隐藏凭据输出)`);
  }
}

export function appleCommand(pieces: TemplateStringsArray, ...args: ShellArgument[]) {
  return retry(
    () => {
      const result = capture(pieces, ...args);
      process.stdout.write(result.stdout);
      process.stderr.write(result.stderr);
      if (!result.ok) throw new CommandError(result);
      return result;
    },
    {
      delays: [5000, 15000, 30000],
      shouldRetry: (error) =>
        error instanceof CommandError &&
        networkError.test(error.result.stdout + error.result.stderr),
      onRetry: (_error, delay, attempt) =>
        console.warn(
          `Apple network request failed; retrying in ${delay / 1000}s (${attempt}/3) / Apple 网络请求失败，${delay / 1000} 秒后重试`,
        ),
    },
  );
}
