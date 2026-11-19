import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test, type TestContext } from "node:test";
import { startGatewayNotifications } from "./gateway-notifications.ts";

class TestNotification extends EventEmitter {
  readonly options: { title: string; body: string };
  shown = false;
  closed = false;

  constructor(options: { title: string; body: string }) {
    super();
    this.options = options;
  }

  show() {
    this.shown = true;
    this.emit("requested");
  }

  close() {
    this.closed = true;
    this.emit("close", { reason: "applicationHidden" });
  }
}

async function notificationHarness(t: TestContext, onRequest?: (notice: TestNotification) => void) {
  const directory = await mkdtemp(join(tmpdir(), "one-gateway-notification-test-"));
  const statePath = join(directory, "notifications.json");
  let stop: (() => Promise<void>) | undefined;
  t.after(async () => {
    await stop?.();
    await rm(directory, { recursive: true, force: true });
  });

  let pollScheduled: (run: () => void) => void;
  t.mock.method(globalThis, "setTimeout", (run: () => void) => {
    pollScheduled(run);
    return 0;
  });
  function afterPoll(run: () => void) {
    return new Promise<() => void>((resolve) => {
      pollScheduled = resolve;
      run();
    });
  }

  const notices: TestNotification[] = [];
  const opened: string[] = [];
  function start() {
    stop = startGatewayNotifications({
      backend: {
        connection: () => ({
          status: "httpReady",
          port: 12345,
          generation: "test",
          token: "test",
          sequence: 1,
          error: "",
        }),
      },
      statePath,
      title: (kind) => kind,
      createNotification: (options) => {
        const notification = new TestNotification(options);
        notification.once("requested", () => onRequest?.(notification));
        notices.push(notification);
        return notification;
      },
      openExecution: (id) => opened.push(id),
    });
  }
  return { afterPoll, start, stop: () => stop?.(), notices, opened, statePath };
}

test(
  "Gateway notifications report new events once and open their execution",
  { timeout: 5000 },
  async (t) => {
    const { afterPoll, start, stop, notices, opened } = await notificationHarness(t, (notice) =>
      notice.emit("show"),
    );
    const listPath = "/api/gateway/executions";
    const historicalFailure = { id: "1".repeat(32), state: "failed" };
    const historicalSuccess = { id: "2".repeat(32), state: "succeeded" };
    const question = {
      id: "3".repeat(32),
      state: "waiting_for_answer",
      questions: [{ id: "A".repeat(26) }],
      input: { linear: { identifier: "MG-10", title: "Example issue" } },
    };
    const continuation = {
      id: "4".repeat(32),
      state: "waiting_to_continue",
      progress: { id: "B".repeat(26) },
    };
    const failure = { id: "5".repeat(32), state: "failed" };
    const success = { id: "6".repeat(32), state: "succeeded" };
    const successWithoutPR = { id: "7".repeat(32), state: "succeeded" };
    let executions: object[] = [historicalFailure, historicalSuccess, question];
    const requests: string[] = [];
    t.mock.method(globalThis, "fetch", (url: string) => {
      const pathname = new URL(url).pathname;
      if (pathname === listPath) return Response.json(executions);
      const id = pathname.split("/").at(-1)!;
      requests.push(id);
      if (id === successWithoutPR.id) return Response.json({ ...successWithoutPR, result: {} });
      assert.equal(id, success.id);
      if (requests.filter((requested) => requested === id).length === 1) {
        return new Response(null, { status: 503 });
      }
      return Response.json({
        ...success,
        result: { pullRequests: [{ url: "https://example.com/pr/1" }] },
      });
    });

    let poll = await afterPoll(start);
    assert.deepEqual(
      notices.map((notice) => notice.options),
      [{ title: "question", body: "MG-10 · Example issue" }],
    );
    assert.deepEqual(requests, []);

    executions = [
      historicalFailure,
      historicalSuccess,
      continuation,
      failure,
      success,
      successWithoutPR,
    ];
    poll = await afterPoll(poll);
    assert.deepEqual(
      notices.map((notice) => notice.options.title),
      ["question", "continuation", "failure"],
    );
    assert.equal(notices[0].closed, true);
    assert.equal(notices[1].closed, false);

    poll = await afterPoll(poll);
    assert.deepEqual(
      notices.map((notice) => notice.options.title),
      ["question", "continuation", "failure", "pullRequest"],
    );
    assert.deepEqual(requests, [success.id, successWithoutPR.id, success.id]);
    assert.ok(notices.every((notice) => notice.shown));
    notices[3].emit("close", { reason: "timedOut" });
    notices[3].emit("click");
    assert.deepEqual(opened, [success.id]);

    executions = [
      { ...continuation, progress: { ...continuation.progress, continueRequested: true } },
      failure,
      success,
      successWithoutPR,
    ];
    poll = await afterPoll(poll);
    assert.equal(notices[1].closed, true);
    assert.deepEqual(requests, [success.id, successWithoutPR.id, success.id]);

    const nextRound = { ...continuation, progress: { id: "C".repeat(26) } };
    executions = [nextRound, failure, success, successWithoutPR];
    await afterPoll(poll);
    assert.equal(notices[4].options.title, "continuation");
    await stop();

    executions = [question, nextRound, failure, success];
    await afterPoll(start);
    assert.equal(notices.length, 5);
  },
);

test(
  "Gateway retries failed delivery and persists only confirmed notices",
  { timeout: 5000 },
  async (t) => {
    const { afterPoll, start, stop, notices, statePath } = await notificationHarness(t);
    const question = {
      id: "8".repeat(32),
      state: "waiting_for_answer",
      questions: [{ id: "D".repeat(26) }],
    };
    const legacyQuestion = { ...question, id: "9".repeat(32) };
    const failure = { id: "a".repeat(32), state: "failed" };
    const key = `${question.id}/question/${question.questions[0].id}`;
    const legacyKey = `${legacyQuestion.id}/question/${legacyQuestion.questions[0].id}`;
    const failureKey = `${failure.id}/failure`;
    const titles = () => notices.map((notice) => notice.options.title);
    const stored = async () => JSON.parse(await readFile(statePath, "utf8"));
    await writeFile(statePath, JSON.stringify([legacyKey]));
    let executions: object[] = [question, legacyQuestion];
    t.mock.method(globalThis, "fetch", () => Response.json(executions));

    let poll = await afterPoll(start);
    assert.deepEqual(titles(), ["question"]);
    assert.deepEqual(await stored(), [legacyKey]);
    poll = await afterPoll(poll);
    assert.deepEqual(titles(), ["question"]);
    notices[0].emit("failed");

    executions = [question, legacyQuestion, failure];
    poll = await afterPoll(poll);
    assert.deepEqual(titles(), ["question", "question", "failure"]);
    notices[0].emit("show");
    assert.deepEqual(await stored(), { delivered: [legacyKey], retryable: [failureKey] });
    notices[1].emit("show");
    notices[2].emit("failed");
    await stop();
    assert.deepEqual(await stored(), { delivered: [legacyKey, key], retryable: [failureKey] });

    poll = await afterPoll(start);
    assert.deepEqual(titles(), ["question", "question", "failure", "failure"]);
    poll = await afterPoll(poll);
    assert.equal(notices.length, 4);
    notices[3].emit("show");
    await afterPoll(poll);
    assert.equal(notices.length, 4);
    await stop();
    assert.deepEqual(await stored(), { delivered: [legacyKey, key, failureKey], retryable: [] });

    await afterPoll(start);
    assert.equal(notices.length, 4);
  },
);
