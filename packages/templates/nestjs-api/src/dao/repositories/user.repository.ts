// 封装对数据库的操作，依赖于 DrizzleService（在基础设施层已封装）。该模块负责从数据库中查询、创建 User 实体。
import { Injectable } from "@nestjs/common";
import { count } from "drizzle-orm";
import { DrizzleService } from "../../service/drizzle/drizzle.service";
import { users } from "../../service/drizzle/schema";
import { User } from "../entities/user.entity";

@Injectable()
export class UserRepository {
  constructor(private readonly drizzle: DrizzleService) {}

  async findAll(
    page: number,
    pageSize: number,
  ): Promise<{ items: User[]; total: number }> {
    const offset = (page - 1) * pageSize;

    const [result, [{ total }]] = await Promise.all([
      this.drizzle.db.select().from(users).limit(pageSize).offset(offset),
      this.drizzle.db.select({ total: count() }).from(users),
    ]);

    return {
      items: result.map((row) => new User(row.id, row.name)),
      total,
    };
  }

  async create(name: string): Promise<User> {
    const [row] = await this.drizzle.db
      .insert(users)
      .values({ name })
      .returning();
    return new User(row.id, row.name);
  }
}
