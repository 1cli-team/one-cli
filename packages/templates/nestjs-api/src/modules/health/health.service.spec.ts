import { HealthService } from "./health.service";

describe("HealthService", () => {
  let service: HealthService;
  let mockDrizzle: any;

  beforeEach(() => {
    mockDrizzle = {
      isConfigured: jest.fn().mockReturnValue(true),
      isHealthy: jest.fn().mockResolvedValue(true),
    };
    service = new HealthService(mockDrizzle);
  });

  describe("getHealthStatus", () => {
    it('should return status "ok" when database is healthy', async () => {
      const result = await service.getHealthStatus();
      expect(result.status).toBe("ok");
      expect(result.database.status).toBe("connected");
    });

    it('should return status "degraded" when database is unhealthy', async () => {
      mockDrizzle.isHealthy.mockResolvedValue(false);
      const result = await service.getHealthStatus();
      expect(result.status).toBe("degraded");
      expect(result.database.status).toBe("disconnected");
    });

    it("should include a valid ISO timestamp", async () => {
      const result = await service.getHealthStatus();
      expect(() => new Date(result.timestamp)).not.toThrow();
      expect(new Date(result.timestamp).toISOString()).toBe(result.timestamp);
    });

    it('should include uptime ending with "s"', async () => {
      const result = await service.getHealthStatus();
      expect(result.uptime).toMatch(/^\d+s$/);
    });

    it('should include memory.used and memory.total ending with "MB"', async () => {
      const result = await service.getHealthStatus();
      expect(result.memory.used).toMatch(/^\d+MB$/);
      expect(result.memory.total).toMatch(/^\d+MB$/);
    });

    it("should return environment from NODE_ENV", async () => {
      const result = await service.getHealthStatus();
      expect(result.environment).toBeDefined();
    });
  });
});
