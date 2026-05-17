import { Test } from "@nestjs/testing";
import { HealthController } from "./health.controller";
import { HealthService } from "./health.service";

describe("HealthController", () => {
  let controller: HealthController;
  let healthService: HealthService;

  const mockHealthStatus = {
    status: "ok",
    timestamp: "2024-01-15T00:00:00.000Z",
    uptime: "100s",
    environment: "test",
    version: "0.1.0",
    memory: { used: "50MB", total: "100MB" },
    database: { status: "connected" },
  };

  beforeEach(async () => {
    const module = await Test.createTestingModule({
      controllers: [HealthController],
      providers: [
        {
          provide: HealthService,
          useValue: {
            getHealthStatus: jest.fn().mockResolvedValue(mockHealthStatus),
          },
        },
      ],
    }).compile();

    controller = module.get(HealthController);
    healthService = module.get(HealthService);
  });

  describe("getHealth", () => {
    it("should call healthService.getHealthStatus and return result", async () => {
      const result = await controller.getHealth();

      expect(healthService.getHealthStatus).toHaveBeenCalled();
      expect(result).toEqual(mockHealthStatus);
    });
  });
});
