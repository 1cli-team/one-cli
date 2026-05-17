// 负责业务流程的编排，调用 Repository 完成具体数据操作。这里的 UserService 对外提供获取用户列表和创建用户的业务逻辑。
import { Injectable } from "@nestjs/common";
import type { User } from "@/dao/entities/user.entity";
// 不能用 `import type`：UserRepository 是构造函数注入的 DI token，
// NestJS 依靠 emitDecoratorMetadata 反射出参数类型；type-only import
// 会被 TS 编译时擦除，运行时反射拿到 Function，触发
// UnknownDependenciesException。
import { UserRepository } from "@/dao/repositories/user.repository";
import { PaginatedResult } from "@/shared/dto/pagination.dto";

@Injectable()
export class UserService {
  constructor(private readonly userRepository: UserRepository) {}

  async getAllUsers(
    page: number,
    pageSize: number,
  ): Promise<PaginatedResult<User>> {
    const { items, total } = await this.userRepository.findAll(page, pageSize);
    return new PaginatedResult(items, total, page, pageSize);
  }

  async createUser(name: string): Promise<User> {
    return this.userRepository.create(name);
  }
}
