import { User } from "@/dao/entities/user.entity";
import { UserService } from "./user.service";

describe("UserService", () => {
  let service: UserService;
  let mockUserRepository: any;

  beforeEach(() => {
    mockUserRepository = {
      findAll: jest.fn().mockResolvedValue({
        items: [new User("1", "Alice"), new User("2", "Bob")],
        total: 2,
      }),
      create: jest.fn().mockResolvedValue(new User("3", "Charlie")),
    };

    service = new UserService(mockUserRepository);
  });

  describe("getAllUsers", () => {
    it("should return paginated users from repository", async () => {
      const result = await service.getAllUsers(1, 20);

      expect(mockUserRepository.findAll).toHaveBeenCalledWith(1, 20);
      expect(result.items).toHaveLength(2);
      expect(result.items[0].name).toBe("Alice");
      expect(result.total).toBe(2);
      expect(result.page).toBe(1);
      expect(result.pageSize).toBe(20);
      expect(result.totalPages).toBe(1);
    });

    it("should return empty result when no users exist", async () => {
      mockUserRepository.findAll.mockResolvedValue({ items: [], total: 0 });

      const result = await service.getAllUsers(1, 20);

      expect(result.items).toEqual([]);
      expect(result.total).toBe(0);
    });
  });

  describe("createUser", () => {
    it("should create user via repository", async () => {
      const result = await service.createUser("Charlie");

      expect(mockUserRepository.create).toHaveBeenCalledWith("Charlie");
      expect(result.name).toBe("Charlie");
    });
  });
});
