import {
  Injectable,
  Logger,
  type OnModuleDestroy,
  type OnModuleInit,
} from "@nestjs/common";
import { PGlite } from "@electric-sql/pglite";
import { sql } from "drizzle-orm";
import {
  drizzle as drizzleNodePg,
  type NodePgDatabase,
} from "drizzle-orm/node-postgres";
import { drizzle as drizzlePglite } from "drizzle-orm/pglite";
import { Pool } from "pg";
import * as schema from "./schema";

/**
 * DrizzleService picks a driver based on DATABASE_URL.
 *
 * - DATABASE_URL set: connect to real Postgres via pg.Pool.
 * - DATABASE_URL empty: spin up an in-memory PGlite (Postgres compiled
 *   to WASM, runs in-process — no daemon, no Docker, no extra install).
 *   Schema is bootstrapped via plain CREATE TABLE IF NOT EXISTS so the
 *   first `one dev` succeeds and endpoints actually work, just with
 *   ephemeral data.
 *
 * Repositories always read `this.drizzle.db`. The public API is a
 * single field; the dual-driver branching is private.
 */
@Injectable()
export class DrizzleService implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(DrizzleService.name);
  private readonly mode: "postgres" | "pglite";
  private readonly pool?: Pool;
  private readonly pglite?: PGlite;
  public readonly db: NodePgDatabase<typeof schema>;

  constructor() {
    const url = process.env.DATABASE_URL?.trim();
    if (url) {
      this.mode = "postgres";
      this.pool = new Pool({ connectionString: url });
      this.db = drizzleNodePg(this.pool, { schema });
    } else {
      this.mode = "pglite";
      this.pglite = new PGlite();
      // PgliteDatabase shares the query-builder API with NodePgDatabase;
      // we cast so repositories type-check against a single shape.
      this.db = drizzlePglite(this.pglite, {
        schema,
      }) as unknown as NodePgDatabase<typeof schema>;
      this.logger.warn(
        "DATABASE_URL 未设置，使用内存 PGlite 作为开发兜底。" +
          "数据不持久化（每次重启都清空）；需要持久化时" +
          "`one env set DATABASE_URL=postgres://... -p <project>`",
      );
    }
  }

  async onModuleInit() {
    if (this.mode === "pglite") {
      await this.bootstrapPgliteSchema();
      this.logger.log("PGlite (in-memory) ready");
      return;
    }
    try {
      const client = await this.pool!.connect();
      client.release();
      this.logger.log("Database connection established");
    } catch (error) {
      this.logger.error(
        "Failed to connect to database",
        (error as Error).stack,
      );
      throw error;
    }
  }

  async onModuleDestroy() {
    if (this.mode === "pglite") {
      await this.pglite!.close();
      return;
    }
    await this.pool!.end();
    this.logger.log("Database connection closed");
  }

  async isHealthy(): Promise<boolean> {
    if (this.mode === "pglite") return true;
    try {
      const client = await this.pool!.connect();
      client.release();
      return true;
    } catch {
      return false;
    }
  }

  /** isConfigured returns true when a real DATABASE_URL was set. */
  isConfigured(): boolean {
    return this.mode === "postgres";
  }

  /**
   * bootstrapPgliteSchema applies the schema DDL to a fresh in-memory
   * PGlite instance. We hand-write the DDL here because drizzle-kit
   * migrations aren't generated for this template. Keep this block in
   * sync with `schema.ts` whenever you add a table.
   */
  private async bootstrapPgliteSchema(): Promise<void> {
    await this.db.execute(sql`
      CREATE TABLE IF NOT EXISTS "users" (
        "id" text PRIMARY KEY,
        "name" varchar(255) NOT NULL UNIQUE,
        "created" timestamp(3) DEFAULT now() NOT NULL,
        "updated" timestamp(3) NOT NULL
      )
    `);
  }
}
