import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { randomBytes, randomUUID, createHash } from "node:crypto";
import { readFile, mkdir, writeFile } from "node:fs/promises";
import { isAbsolute, join } from "node:path";
import { EventEmitter } from "node:events";
import type { Connection } from "../../../packages/desktop-preload/src/api.ts";

export interface Manifest {
  version: string;
  commit: string;
  channel: string;
  protocol: number;
  api: number;
  target: string;
  sha256: string;
  cliSha256: string;
}

interface BackendOptions {
  executable: string;
  manifest: Manifest;
  origin: string;
  env?: NodeJS.ProcessEnv;
  timeout?: number;
  verifyBinary?: boolean;
  executableArgs?: string[];
}

// One owner for the child, private control pipe, and each session generation.
// Business calls continue to use the authenticated loopback HTTP API.
export function createBackend(options: BackendOptions) {
  const { executable, manifest, origin, env = process.env, timeout = 15000 } = options;
  const events = new EventEmitter();
  let snapshot: Connection = {
    sequence: 0,
    status: "starting",
    generation: "",
    port: 0,
    error: "",
  };
  let child: ChildProcessWithoutNullStreams | undefined;
  let storageRoot = "";
  let launching = false;
  let stopping = false;
  let pendingQuiesce:
    | { resolve: (active: number) => void; reject: (error: Error) => void }
    | undefined;

  function publish(status: string, error = "") {
    snapshot = { ...snapshot, sequence: snapshot.sequence + 1, status, error };
    if (status !== "httpReady") {
      snapshot.port = 0;
      delete snapshot.token;
    }
    // Never put credentials in a broadcast event or log.
    events.emit("state");
  }

  function send(type: string) {
    if (!child || child.stdin.destroyed) throw new Error("backend_stopped");
    child.stdin.write(JSON.stringify({ type }) + "\n");
  }

  async function start() {
    if (launching || child || stopping) return;
    launching = true;
    publish("starting");
    try {
      if (options.verifyBinary !== false) {
        const digest = createHash("sha256")
          .update(await readFile(executable))
          .digest("hex");
        if (digest !== manifest.sha256) throw new Error("backend_identity");
      }
      if (stopping) return;
      const generation = randomUUID();
      const token = randomBytes(32).toString("hex");
      const processChild = spawn(executable, options.executableArgs ?? [], {
        env,
        stdio: "pipe",
        windowsHide: true,
        detached: false,
      });
      child = processChild;
      let buffer = Buffer.alloc(0);
      let ready = false;
      let failure = "";
      const timer = setTimeout(() => fail("backend_timeout"), timeout);
      const fail = (error: string) => {
        if (child !== processChild || failure || stopping) return;
        failure = error;
        clearTimeout(timer);
        publish("failed", error);
        processChild.kill();
      };
      processChild.once("error", () => fail("backend_missing"));
      processChild.stdin.on("error", () => fail("backend_handshake"));
      // Drain stderr without logging credentials or buffering unbounded output.
      processChild.stderr.resume();
      processChild.stdout.on("data", (chunk: Buffer) => {
        if (failure || child !== processChild) return;
        buffer = Buffer.concat([buffer, chunk]);
        let newline: number;
        while ((newline = buffer.indexOf(10)) >= 0) {
          if (newline > 16384) return fail("backend_protocol");
          const line = buffer.subarray(0, newline).toString("utf8");
          buffer = buffer.subarray(newline + 1);
          try {
            const value = JSON.parse(line);
            if (value.type === "failed" && !ready) {
              return fail(typeof value.error === "string" ? value.error : "backend_storage");
            }
            if (value.type === "ready" && !ready) {
              if (
                !Number.isInteger(value.port) ||
                value.port < 1 ||
                value.port > 65535 ||
                typeof value.storageRoot !== "string" ||
                !isAbsolute(value.storageRoot) ||
                value.generation !== generation ||
                value.version !== manifest.version ||
                value.commit !== manifest.commit ||
                value.protocol !== manifest.protocol ||
                value.api !== manifest.api ||
                value.channel !== manifest.channel
              ) {
                return fail("backend_identity");
              }
              ready = true;
              clearTimeout(timer);
              storageRoot = value.storageRoot;
              snapshot = { ...snapshot, generation, port: value.port, token };
              publish("httpReady");
            } else if (value.type === "quiesced" && pendingQuiesce) {
              if (!Number.isSafeInteger(value.active) || value.active < 0)
                return fail("backend_protocol");
              pendingQuiesce.resolve(value.active);
              pendingQuiesce = undefined;
            }
          } catch {
            return fail("backend_protocol");
          }
        }
        if (buffer.length > 16384) fail("backend_protocol");
      });
      processChild.once("close", () => {
        clearTimeout(timer);
        if (child !== processChild) return;
        child = undefined;
        pendingQuiesce?.reject(new Error("backend_stopped"));
        pendingQuiesce = undefined;
        if (stopping) publish("stopped");
        else if (!failure) publish("failed", "backend_exited");
      });
      processChild.stdin.write(
        JSON.stringify({
          type: "bootstrap",
          protocol: manifest.protocol,
          generation,
          token,
          origin,
        }) + "\n",
      );
    } catch (error) {
      publish(
        "failed",
        error instanceof Error && error.message === "backend_identity"
          ? "backend_identity"
          : "backend_missing",
      );
    } finally {
      launching = false;
    }
  }

  async function quiesce(): Promise<number> {
    if (!child || snapshot.status === "failed") return 0;
    // A not-yet-ready child has accepted no HTTP writes and may be stopped.
    if (snapshot.status === "starting") return 0;
    if (snapshot.status !== "httpReady" || pendingQuiesce) throw new Error("backend_busy_timeout");
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        pendingQuiesce = undefined;
        try {
          send("resume");
        } catch {
          /* Child already exited. */
        }
        reject(new Error("backend_busy_timeout"));
      }, 4000);
      pendingQuiesce = {
        resolve: (active) => {
          clearTimeout(timer);
          resolve(active);
        },
        reject: (error) => {
          clearTimeout(timer);
          reject(error);
        },
      };
      try {
        send("quiesce");
      } catch (error) {
        pendingQuiesce.reject(error as Error);
        pendingQuiesce = undefined;
      }
    });
  }

  async function stop() {
    stopping = true;
    const current = child;
    if (!current) {
      publish("stopped");
      return;
    }
    publish("stopping");
    await new Promise<void>((resolve) => {
      const force = setTimeout(() => current.kill("SIGKILL"), 12000);
      current.once("close", () => {
        clearTimeout(force);
        resolve();
      });
      try {
        send("shutdown");
        current.stdin.end();
      } catch {
        current.kill();
      }
    });
  }

  async function retry() {
    if (launching || stopping) return;
    if (snapshot.status === "httpReady") return;
    if (child) {
      const old = child;
      await new Promise<void>((resolve) => {
        old.once("close", resolve);
        old.kill("SIGKILL");
      });
    }
    await start();
  }

  async function confirmHealth(generation: string) {
    if (snapshot.status !== "httpReady" || snapshot.generation !== generation) {
      throw new Error("backend_session_changed");
    }
    const directory = join(storageRoot, "state", "desktop");
    const path = join(directory, "update-transaction.json");
    try {
      const record = JSON.parse(await readFile(path, "utf8"));
      if (record.to === manifest.version) {
        record.phase = "healthy";
        await mkdir(directory, { recursive: true });
        await writeFile(path, JSON.stringify(record), { mode: 0o600 });
      }
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw new Error("backend_storage");
    }
  }

  return {
    start,
    stop,
    retry,
    quiesce,
    confirmHealth,
    resume: () => send("resume"),
    connection: (): Connection => ({ ...snapshot }),
    onState(callback: () => void) {
      events.on("state", callback);
      return () => events.off("state", callback);
    },
  };
}

export type Backend = ReturnType<typeof createBackend>;
