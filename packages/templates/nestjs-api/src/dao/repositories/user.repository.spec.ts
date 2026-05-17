import { Test } from "@nestjs/testing";
import { User } from "../entities/user.entity";
import { DrizzleService } from "../../service/drizzle/drizzle.service";
import { UserRepository } from "./user.repository";

describe("UserRepository", () => {
  let repository: UserRepository;
  let drizzleService: any;

  // 模拟 Drizzle 链式查询
  let mockFrom: jest.Mock;
  let mockSelect: jest.Mock;
  let mockInsert: jest.Mock;
  let mockValues: jest.Mock;
  let mockReturning: jest.Mock;
  let mockLimit: jest.Mock;
  let mockOffset: jest.Mock;

  beforeEach(async () => {
    mockOffset = jest.fn();
    mockLimit = jest.fn().mockReturnValue({ offset: mockOffset });
    mockFrom = jest.fn().mockReturnValue({ limit: mockLimit });
    mockSelect = jest.fn().mockReturnValue({ from: mockFrom });
    mockReturning = jest.fn();
    mockValues = jest.fn().mockReturnValue({ returning: mockReturning });
    mockInsert = jest.fn().mockReturnValue({ values: mockValues });

    drizzleService = {
      db: {
        select: mockSelect,
        insert: mockInsert,
      },
    };

    const module = await Test.createTestingModule({
      providers: [
        UserRepository,
        { provide: DrizzleService, useValue: drizzleService },
      ],
    }).compile();

    repository = module.get(UserRepository);
  });

  describe("findAll", () => {
    it("should return paginated User entities", async () => {
      // Mock for data query (select().from().limit().offset())
      mockOffset.mockResolvedValue([
        { id: "1", name: "Alice", created: new Date(), updated: new Date() },
        { id: "2", name: "Bob", created: new Date(), updated: new Date() },
      ]);

      // Mock for count query - second call to select().from()
      const mockCountFrom = jest.fn().mockResolvedValue([{ total: 5 }]);
      const mockCountSelect = jest
        .fn()
        .mockReturnValue({ from: mockCountFrom });

      // Override select to return different chains for data vs count queries
      let selectCallCount = 0;
      drizzleService.db.select = jest
        .fn()
        .mockImplementation((...args: any[]) => {
          selectCallCount++;
          if (selectCallCount === 1) {
            return { from: mockFrom };
          }
          return mockCountSelect(...args);
        });

      const result = await repository.findAll(1, 20);

      expect(result.items).toHaveLength(2);
      expect(result.items[0]).toBeInstanceOf(User);
      expect(result.items[0].name).toBe("Alice");
      expect(result.total).toBe(5);
    });

    it("should return empty items when no users exist", async () => {
      mockOffset.mockResolvedValue([]);
      const mockCountFrom = jest.fn().mockResolvedValue([{ total: 0 }]);

      let selectCallCount = 0;
      drizzleService.db.select = jest
        .fn()
        .mockImplementation((...args: any[]) => {
          selectCallCount++;
          if (selectCallCount === 1) {
            return { from: mockFrom };
          }
          return { from: mockCountFrom };
        });

      const result = await repository.findAll(1, 20);

      expect(result.items).toEqual([]);
      expect(result.total).toBe(0);
    });
  });

  describe("create", () => {
    it("should create and return a User entity", async () => {
      mockReturning.mockResolvedValue([
        {
          id: "3",
          name: "Charlie",
          created: new Date(),
          updated: new Date(),
        },
      ]);

      const result = await repository.create("Charlie");

      expect(result).toBeInstanceOf(User);
      expect(result.id).toBe("3");
      expect(result.name).toBe("Charlie");
    });
  });
});
