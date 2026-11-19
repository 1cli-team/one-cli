import { mkdir, readFile, rename, writeFile } from "node:fs/promises";
import { dirname } from "node:path";
import type { Backend } from "./backend.ts";

interface MonitoredExecution {
  id: string;
  state: string;
  questions?: { id: string; answer?: string }[];
  progress?: { id: string; continueRequested?: boolean };
  result?: { pullRequests?: { url: string }[] };
  input?: { linear?: { identifier: string; title: string } };
}

type NoticeKind = "question" | "continuation" | "failure" | "pullRequest";

interface DesktopNotification {
  once(event: "show" | "click" | "failed", listener: () => void): void;
  on(event: "close", listener: (details: { reason?: string }) => void): void;
  show(): void;
  close(): void;
}

const interactionId = /^[A-Z2-7]{26}$/;

function executionNotice(execution: MonitoredExecution) {
  if (!/^[a-f0-9]{32}$/.test(execution?.id)) return;
  const { id, state, questions, progress } = execution;
  switch (state) {
    case "waiting_for_answer": {
      const question = questions?.at(-1);
      if (question && !question.answer && interactionId.test(question.id)) {
        return { key: `${id}/question/${question.id}`, kind: "question", pending: true } as const;
      }
      break;
    }
    case "waiting_to_continue":
      if (progress && !progress.continueRequested && interactionId.test(progress.id)) {
        return {
          key: `${id}/continuation/${progress.id}`,
          kind: "continuation",
          pending: true,
        } as const;
      }
      break;
    case "failed":
      return { key: `${id}/failure`, kind: "failure", pending: false } as const;
    case "succeeded":
      return { key: `${id}/pullRequest`, kind: "pullRequest", pending: false } as const;
  }
}

interface NotificationHistory {
  delivered: Set<string>;
  retryable: Set<string>;
}

async function readHistory(path: string): Promise<NotificationHistory> {
  const keys = (value: unknown) =>
    new Set<string>(Array.isArray(value) ? value.filter((key) => typeof key === "string") : []);
  try {
    const history = JSON.parse(await readFile(path, "utf8"));
    if (Array.isArray(history)) return { delivered: keys(history), retryable: new Set() };
    return { delivered: keys(history?.delivered), retryable: keys(history?.retryable) };
  } catch {
    // Keep notifications available even if the local history can't be read.
    return { delivered: new Set(), retryable: new Set() };
  }
}

const executionsPath = "/api/gateway/executions";

export function startGatewayNotifications({
  backend,
  statePath,
  title,
  createNotification,
  openExecution,
}: {
  backend: Pick<Backend, "connection">;
  statePath: string;
  title(kind: NoticeKind): string;
  createNotification(options: { title: string; body: string }): DesktopNotification;
  openExecution(id: string): void;
}) {
  const stop = new AbortController();
  const history = readHistory(statePath);
  const ignored = new Set<string>();
  const notifications = new Map<string, { notification: DesktopNotification; pending: boolean }>();
  let writing = Promise.resolve();
  let firstSnapshot = true;
  let timer: ReturnType<typeof setTimeout> | undefined;

  function saveHistory({ delivered, retryable }: NotificationHistory) {
    const snapshot = JSON.stringify({ delivered: [...delivered], retryable: [...retryable] });
    writing = writing
      .then(async () => {
        await mkdir(dirname(statePath), { recursive: true });
        await writeFile(`${statePath}.tmp`, snapshot, { mode: 0o600 });
        await rename(`${statePath}.tmp`, statePath);
      })
      .catch(() => {
        // Keep the latest history in memory if local storage is unavailable.
      });
    return writing;
  }

  async function poll() {
    try {
      const { status, port, generation, token } = backend.connection();
      if (status !== "httpReady" || !port || !token) return;
      const signal = AbortSignal.any([stop.signal, AbortSignal.timeout(4000)]);
      async function read<T>(path: string): Promise<T | undefined> {
        try {
          const response = await fetch(`http://127.0.0.1:${port}${path}`, {
            headers: { Authorization: `Bearer ${token}`, "X-One-Backend-Generation": generation },
            redirect: "error",
            signal,
          });
          if (response.ok) return await response.json();
          await response.body?.cancel();
        } catch {
          // A failed request can be retried on the next poll.
        }
      }

      const executions = await read<MonitoredExecution[]>(executionsPath);
      if (!Array.isArray(executions) || stop.signal.aborted) return;
      const state = await history;
      const pending = new Set<string>();
      const ready = new Map<
        string,
        {
          execution: MonitoredExecution;
          notice: NonNullable<ReturnType<typeof executionNotice>>;
        }
      >();
      for (const execution of executions) {
        const notice = executionNotice(execution);
        if (!notice) continue;
        const { key } = notice;
        if (notice.pending) pending.add(key);
        else if (firstSnapshot && !state.retryable.has(key)) ignored.add(key);
        if (
          state.delivered.has(key) ||
          ignored.has(key) ||
          notifications.has(key) ||
          ready.has(key)
        ) {
          continue;
        }

        if (notice.kind === "pullRequest") {
          const detail = await read<MonitoredExecution>(`${executionsPath}/${execution.id}`);
          if (detail?.id !== execution.id || detail.state !== "succeeded") continue;
          if (!detail.result?.pullRequests?.some((pr) => pr.url)) {
            ignored.add(notice.key);
            continue;
          }
        }
        ready.set(key, { execution, notice });
      }
      firstSnapshot = false;
      if (stop.signal.aborted) return;

      for (const [key, entry] of notifications) {
        if (entry.pending && !pending.has(key)) {
          notifications.delete(key);
          entry.notification.close();
        }
      }
      // Terminal events queued for delivery must survive startup's history filter.
      const retryableCount = state.retryable.size;
      for (const { notice } of ready.values()) {
        if (!notice.pending) state.retryable.add(notice.key);
      }
      if (state.retryable.size !== retryableCount) await saveHistory(state);
      if (stop.signal.aborted) return;

      for (const { execution, notice } of ready.values()) {
        const issue = execution.input?.linear;
        const body = issue ? `${issue.identifier} · ${issue.title}` : execution.id;
        const notification = createNotification({ title: title(notice.kind), body });
        const current = () => notifications.get(notice.key)?.notification === notification;
        const release = () => {
          if (current()) notifications.delete(notice.key);
        };
        const confirm = () => {
          if (stop.signal.aborted || !current() || state.delivered.has(notice.key)) return;
          state.delivered.add(notice.key);
          state.retryable.delete(notice.key);
          void saveHistory(state);
        };
        notification.once("show", confirm);
        notification.once("click", () => {
          if (!current()) return;
          confirm();
          release();
          notification.close();
          openExecution(execution.id);
        });
        notification.once("failed", release);
        notification.on("close", ({ reason }) => {
          // A timed-out toast can still be clicked in the notification center.
          if (reason !== "timedOut") release();
        });
        notifications.set(notice.key, { notification, pending: notice.pending });
        try {
          notification.show();
        } catch {
          release();
        }
      }
    } catch {
      // Keep monitoring after an invalid response or a native notification error.
    } finally {
      if (!stop.signal.aborted) timer = setTimeout(() => void poll(), 4000);
    }
  }

  void poll();
  return async () => {
    stop.abort();
    clearTimeout(timer);
    for (const { notification } of notifications.values()) notification.close();
    notifications.clear();
    await writing;
  };
}
