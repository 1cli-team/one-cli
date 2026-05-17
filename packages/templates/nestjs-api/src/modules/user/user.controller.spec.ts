import { Test } from "@nestjs/testing";
import { User } from "@/dao/entities/user.entity";
import { PaginatedResult } from "@/shared/dto/pagination.dto";
import { UserController } from "./user.controller";
import { UserService } from "./user.service";

describe("UserController", () => {
  let controller: UserController;
  let userService: UserService;

  const mockPaginatedResult = new PaginatedResult(
    [new User("1", "Alice"), new User("2", "Bob")],
    2,
    1,
    20,
  );

  beforeEach(async () => {
    const module = await Test.createTestingModule({
      controllers: [UserController],
      providers: [
        {
          provide: UserService,
          useValue: {
            getAllUsers: jest.fn().mockResolvedValue(mockPaginatedResult),
            createUser: jest.fn().mockResolvedValue(new User("3", "Charlie")),
          },
        },
      ],
    }).compile();

    controller = module.get(UserController);
    userService = module.get(UserService);
  });

  describe("getUsers", () => {
    it("should return paginated users", async () => {
      const result = await controller.getUsers({ page: 1, pageSize: 20 });

      expect(userService.getAllUsers).toHaveBeenCalledWith(1, 20);
      expect(result.items).toHaveLength(2);
      expect(result.total).toBe(2);
    });
  });

  describe("createUser", () => {
    it("should create user with dto name", async () => {
      const result = await controller.createUser({ name: "Charlie" } as any);

      expect(userService.createUser).toHaveBeenCalledWith("Charlie");
      expect(result.name).toBe("Charlie");
    });
  });
});
