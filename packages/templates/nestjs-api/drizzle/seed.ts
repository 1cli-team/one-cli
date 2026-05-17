/// <reference types="node" />
import "dotenv/config";
import { drizzle } from "drizzle-orm/node-postgres";
import { Pool } from "pg";
import { users } from "../src/service/drizzle/schema";

async function main() {
  const pool = new Pool({
    connectionString: process.env.DATABASE_URL,
  });

  const db = drizzle(pool);

  // 创建默认账户
  const [defaultUser] = await db
    .insert(users)
    .values({
      name: "default-user",
    })
    .returning();

  console.log("Default user created:", defaultUser);

  await pool.end();
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
