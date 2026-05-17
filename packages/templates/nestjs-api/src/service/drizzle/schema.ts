import { randomUUID } from "node:crypto";
import { pgTable, text, varchar, timestamp } from "drizzle-orm/pg-core";

// users 表定义
export const users = pgTable("users", {
  id: text("id")
    .primaryKey()
    .$defaultFn(() => randomUUID()),
  name: varchar("name", { length: 255 }).unique().notNull(),
  created: timestamp("created", { precision: 3 }).defaultNow().notNull(),
  updated: timestamp("updated", { precision: 3 })
    .notNull()
    .$onUpdate(() => new Date()),
});

// 推断类型
export type UserSelect = typeof users.$inferSelect;
export type UserInsert = typeof users.$inferInsert;
