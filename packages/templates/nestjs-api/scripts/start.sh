#!/bin/sh

# 执行 Drizzle 数据库迁移
npx drizzle-kit migrate

# 启动应用
pm2-runtime dist/main.js
