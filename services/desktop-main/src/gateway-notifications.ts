import { Notification } from "electron";
import type { Backend } from "./backend.ts";

interface MonitoredExecution {
  id: string;
  state: string;
  questions?: { id: string; answer?: string }[];
  input?: { linear?: { identifier: string; title: string } };
}

export function startGatewayNotifications({
  backend,
  title,
  openExecution,
}: {
  backend: Backend;
  title(): string;
  openExecution(id: string): void;
}) {
  const stop = new AbortController();
  const notifications = new Map<string, Notification>();
  let timer: ReturnType<typeof setTimeout> | undefined;

  async function poll() {
    try {
      const { status, port, generation, token } = backend.connection();
      if (status !== "httpReady" || !port || !token) return;
      const response = await fetch(`http://127.0.0.1:${port}/api/gateway/executions`, {
        headers: { Authorization: `Bearer ${token}`, "X-One-Backend-Generation": generation },
        redirect: "error",
        signal: AbortSignal.any([stop.signal, AbortSignal.timeout(4000)]),
      });
      if (!response.ok) {
        await response.body?.cancel();
        return;
      }
      const executions: MonitoredExecution[] = await response.json();
      if (!Array.isArray(executions) || stop.signal.aborted) return;
      const pending = new Set<string>();
      for (const execution of executions) {
        if (execution?.state !== "waiting_for_answer" || !/^[a-f0-9]{32}$/.test(execution.id))
          continue;
        const question = execution.questions?.at(-1);
        if (!question || question.answer || !/^[A-Z2-7]{26}$/.test(question.id)) continue;
        const key = `${execution.id}/${question.id}`;
        pending.add(key);
        if (notifications.has(key)) continue;
        const issue = execution.input?.linear;
        const body = issue ? `${issue.identifier} · ${issue.title}` : execution.id;
        const notification = new Notification({ title: title(), body });
        notification.on("click", () => openExecution(execution.id));
        notification.show();
        notifications.set(key, notification);
      }
      for (const [key, notification] of notifications) {
        if (!pending.has(key)) {
          notification.close();
          notifications.delete(key);
        }
      }
    } catch {
      // Gateway may be offline. Keep delivered notifications until a successful poll.
    } finally {
      if (!stop.signal.aborted) timer = setTimeout(() => void poll(), 4000);
    }
  }

  if (Notification.isSupported()) void poll();
  return () => {
    stop.abort();
    clearTimeout(timer);
    for (const notification of notifications.values()) notification.close();
    notifications.clear();
  };
}
