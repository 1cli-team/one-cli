import { Injectable } from "@nestjs/common";
import { DrizzleService } from "@/service/drizzle/drizzle.service";

@Injectable()
export class HealthService {
  constructor(private readonly drizzle: DrizzleService) {}

  async getHealthStatus() {
    const dbConfigured = this.drizzle.isConfigured();
    const dbHealthy = await this.drizzle.isHealthy();
    const uptime = process.uptime();

    // PGlite fallback (dbConfigured=false) is always healthy as long as
    // the process is up. Real Postgres can degrade when DATABASE_URL is
    // set but the server is unreachable.
    const status = dbHealthy ? "ok" : "degraded";

    return {
      status,
      timestamp: new Date().toISOString(),
      uptime: `${Math.floor(uptime)}s`,
      environment: process.env.NODE_ENV || "development",
      version: process.env.npm_package_version || "0.1.0",
      memory: {
        used: `${Math.round(process.memoryUsage().heapUsed / 1024 / 1024)}MB`,
        total: `${Math.round(process.memoryUsage().heapTotal / 1024 / 1024)}MB`,
      },
      database: {
        driver: dbConfigured ? "postgres" : "pglite",
        status: dbHealthy ? "connected" : "disconnected",
      },
    };
  }
}
