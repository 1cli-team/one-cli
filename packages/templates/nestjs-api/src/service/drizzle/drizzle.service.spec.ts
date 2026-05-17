import { DrizzleService } from "./drizzle.service";

describe("DrizzleService", () => {
  const originalUrl = process.env.DATABASE_URL;

  afterEach(() => {
    if (originalUrl === undefined) {
      delete process.env.DATABASE_URL;
    } else {
      process.env.DATABASE_URL = originalUrl;
    }
  });

  describe("when DATABASE_URL is empty (PGlite fallback)", () => {
    beforeEach(() => {
      delete process.env.DATABASE_URL;
    });

    it("constructs without throwing and exposes a usable db handle", () => {
      const svc = new DrizzleService();
      expect(svc.db).toBeDefined();
      expect(svc.isConfigured()).toBe(false);
    });

    it("onModuleInit bootstraps the schema in-memory", async () => {
      const svc = new DrizzleService();
      await expect(svc.onModuleInit()).resolves.toBeUndefined();
      await svc.onModuleDestroy();
    });

    it("isHealthy returns true once PGlite is ready", async () => {
      const svc = new DrizzleService();
      await expect(svc.isHealthy()).resolves.toBe(true);
      await svc.onModuleDestroy();
    });
  });

  describe("when DATABASE_URL is set", () => {
    beforeEach(() => {
      // Non-reachable host — Pool construction is lazy (no TCP). Tests
      // here exercise the configured branch without dialing.
      process.env.DATABASE_URL = "postgresql://test:test@127.0.0.1:1/test";
    });

    it("constructs the Postgres driver and marks itself configured", () => {
      const svc = new DrizzleService();
      expect(svc.db).toBeDefined();
      expect(svc.isConfigured()).toBe(true);
    });
  });

  describe("when DATABASE_URL is whitespace", () => {
    beforeEach(() => {
      process.env.DATABASE_URL = "   ";
    });

    it("is treated as empty and falls back to PGlite", () => {
      const svc = new DrizzleService();
      expect(svc.isConfigured()).toBe(false);
    });
  });
});
