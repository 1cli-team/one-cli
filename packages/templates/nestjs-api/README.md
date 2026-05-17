# Nest Server 模板项目

项目主要包含一个 Nest 工程模板，并包含一些常用的模块和工具函数。

## 架构总览

```mermaid
graph TB
    subgraph Client["客户端"]
        Browser["Browser / API Client"]
    end

    subgraph Docker["Docker 部署层"]
        subgraph NestApp["NestJS Application"]

            subgraph Entry["入口层"]
                Main["main.ts<br/>应用引导"]
                Swagger["Swagger UI<br/>/api/docs"]
            end

            subgraph Middleware["全局中间件 & 管道"]
                VP["ValidationPipe<br/>参数校验"]
                AuthGuard["AuthGuard<br/>JWT 认证守卫"]
                RI["ResponseInterceptor<br/>响应格式化"]
                MI["MetricsInterceptor<br/>指标采集"]
                LI["LoggingInterceptor<br/>请求日志"]
                EF["HttpExceptionFilter<br/>异常过滤器"]
            end

            subgraph Modules["业务模块层"]
                AuthMod["Auth Module<br/>登录 / Token 刷新"]
                UserMod["User Module<br/>用户 CRUD"]
                CommonMod["Common Module<br/>公共接口"]
                HealthMod["Health Module<br/>健康检查"]
                MetricsMod["Metrics Module<br/>Prometheus 指标"]
            end

            subgraph DAO["数据访问层"]
                Repo["UserRepository<br/>数据仓储"]
                Entity["User Entity<br/>领域实体"]
            end

            subgraph Infra["基础设施层"]
                Drizzle["DrizzleService<br/>ORM 服务"]
                Config["ConfigModule<br/>配置管理"]
                Logger["Pino Logger<br/>日志系统"]
                JWT["JwtModule<br/>JWT 签发/验证"]
            end

        end
    end

    subgraph External["外部服务"]
        PG[("PostgreSQL 15<br/>数据库")]
        Prom["Prometheus<br/>监控采集"]
    end

    Browser -->|"HTTP Request"| Main
    Main --> Swagger

    Main --> VP --> AuthGuard --> RI --> MI --> LI --> EF

    EF --> AuthMod
    EF --> UserMod
    EF --> CommonMod
    EF --> HealthMod
    EF --> MetricsMod

    AuthMod --> JWT
    UserMod --> Repo
    Repo --> Entity
    Repo --> Drizzle

    HealthMod --> Drizzle
    MetricsMod --> Prom

    Drizzle --> PG
    Config -.->|"环境变量"| Drizzle
    Config -.->|"JWT_SECRET"| JWT
    Logger -.->|"日志文件"| LogFiles["logs/<br/>info / warn / error / debug"]
```

## 请求处理流程

```mermaid
sequenceDiagram
    participant C as Client
    participant VP as ValidationPipe
    participant AG as AuthGuard
    participant RI as ResponseInterceptor
    participant MI as MetricsInterceptor
    participant LI as LoggingInterceptor
    participant Ctrl as Controller
    participant Svc as Service
    participant Repo as Repository
    participant DB as PostgreSQL

    C->>VP: HTTP Request
    VP->>VP: 参数校验 (class-validator)
    VP->>AG: 校验通过
    AG->>AG: 检查 @Public() 装饰器
    AG->>AG: 验证 JWT Token
    AG->>RI: 认证通过
    RI->>MI: 进入拦截器链
    MI->>LI: 记录开始时间
    LI->>Ctrl: 记录请求日志
    Ctrl->>Svc: 调用业务逻辑
    Svc->>Repo: 数据操作
    Repo->>DB: SQL 查询
    DB-->>Repo: 查询结果
    Repo-->>Svc: 返回数据
    Svc-->>Ctrl: 返回结果
    Ctrl-->>LI: 原始响应
    LI-->>MI: 记录响应日志
    MI-->>RI: 记录请求耗时
    RI-->>C: { code: 0, message: "success", data: ... }
```

## 模块依赖关系

```mermaid
graph LR
    App["AppModule"] --> Config["ConfigModule<br/>(Global)"]
    App --> Drizzle["DrizzleModule<br/>(Global)"]
    App --> JWT["JwtModule<br/>(Global)"]
    App --> Prom["PrometheusModule"]
    App --> Auth["AuthModule"]
    App --> User["UserModule"]
    App --> Common["CommonModule"]
    App --> Health["HealthModule"]
    App --> Metrics["MetricsModule"]

    Auth -->|"APP_GUARD"| AuthGuard["AuthGuard"]
    Auth --> JWT
    User --> UserRepo["UserRepository"]
    UserRepo --> Drizzle
    Health --> Drizzle
    Metrics --> Prom

    style App fill:#e1f5fe
    style Config fill:#fff9c4
    style Drizzle fill:#fff9c4
    style JWT fill:#fff9c4
```

## 项目结构

```bash
nest-template/
├── Dockerfile                 # 多阶段构建，PM2 生产运行
├── docker-compose.yml         # 本地编排 (app + postgres)
├── .env                       # 环境变量配置
├── drizzle.config.ts          # Drizzle Kit 迁移配置
├── drizzle/                   # 数据库迁移文件 & seed
├── package.json               # 项目依赖与脚本
├── tsconfig.json              # TypeScript 配置
├── nest-cli.json              # NestJS CLI 配置
└── src/
    ├── main.ts                # 应用入口：全局管道、守卫、拦截器、过滤器
    ├── app.module.ts          # 根模块：整合各个功能模块
    ├── app.controller.ts      # 根路由：API 信息
    │
    ├── core/                  # 核心层：框架级别的公共组件
    │   ├── config/
    │   │   └── configuration.ts     # 配置对象 (jwt / server / database / app)
    │   ├── constants/
    │   │   └── error-code.ts        # 业务错误码枚举
    │   ├── decorators/
    │   │   ├── public.decorators.ts # @Public() 公共路由装饰器
    │   │   └── current-user.decorator.ts  # @CurrentUser() 用户提取装饰器
    │   ├── exceptions/
    │   │   └── business.exception.ts      # 自定义业务异常
    │   ├── filters/
    │   │   └── http-exception.filter.ts   # 全局异常过滤器
    │   └── interceptors/
    │       ├── response.interceptor.ts    # 响应格式统一 {code, message, data}
    │       ├── metrics.interceptor.ts     # Prometheus 指标采集
    │       └── logging.interceptor.ts     # 请求/响应日志 (敏感字段脱敏)
    │
    ├── dao/                   # 数据访问层
    │   ├── entities/
    │   │   └── user.entity.ts       # 用户领域实体
    │   └── repositories/
    │       └── user.repository.ts   # 用户数据仓储
    │
    ├── modules/               # 业务模块层
    │   ├── auth/              # 认证模块
    │   │   ├── auth.module.ts
    │   │   ├── auth.controller.ts   # POST /login, /refresh
    │   │   ├── auth.service.ts      # Token 签发/验证
    │   │   ├── auth.guard.ts        # JWT 全局守卫
    │   │   └── dto/
    │   ├── user/              # 用户模块
    │   │   ├── user.module.ts
    │   │   ├── user.controller.ts   # GET /users, POST /users
    │   │   ├── user.service.ts
    │   │   └── dto/
    │   ├── common/            # 公共接口模块
    │   │   ├── common.module.ts
    │   │   ├── common.controller.ts
    │   │   └── common.service.ts
    │   ├── health/            # 健康检查模块
    │   │   ├── health.module.ts
    │   │   ├── health.controller.ts # GET /health
    │   │   └── health.service.ts
    │   └── metrics/           # 指标模块
    │       └── metrics.module.ts    # Prometheus 注册
    │
    ├── service/               # 基础设施层
    │   └── drizzle/
    │       ├── drizzle.module.ts    # 全局数据库模块
    │       ├── drizzle.service.ts   # 数据库连接 & 健康检查
    │       └── schema.ts           # Drizzle 表定义
    │
    └── shared/                # 共享工具层
        ├── dto/
        │   └── pagination.dto.ts    # 分页 DTO
        └── utils/
            ├── index.ts
            └── logger.ts            # Pino 多目标日志 (按级别/日期切分)
```

## 技术栈

| 类别     | 技术              | 说明                              |
| -------- | ----------------- | --------------------------------- |
| 框架     | NestJS 11         | Node.js 企业级框架                |
| 语言     | TypeScript 5.8    | 类型安全                          |
| 数据库   | PostgreSQL 15     | 关系型数据库                      |
| ORM      | Drizzle ORM       | 类型安全的 SQL 查询构建器         |
| 认证     | JWT               | 无状态令牌认证                    |
| 文档     | Swagger / OpenAPI | 自动生成 API 文档                 |
| 监控     | Prometheus        | 指标采集 (请求计数、耗时)         |
| 日志     | Pino              | 高性能 JSON 日志，按级别/日期切分 |
| 校验     | class-validator   | DTO 参数校验                      |
| 部署     | Docker + PM2      | 多阶段构建，进程管理              |
| 代码质量 | oxlint + oxfmt    | 快速 lint 和格式化                |
| 版本管理 | pnpm + Git        | 依赖锁定与源码版本控制            |

## 项目启动

```bash
# 启动 PostgreSQL
docker compose up -d postgres

# 安装依赖
pnpm install

# 开发环境同步数据库
pnpm db:push

# 初始化数据库迁移
pnpm db:generate

# 执行迁移
pnpm db:migrate

# 初始化默认数据
pnpm db:seed

# 开发环境启动 (带热更新)
pnpm run start:dev

# 生产环境启动
pnpm run start
```

## 环境变量

```bash
PORT=3000
DATABASE_URL=postgresql://postgres:postgres@127.0.0.1:5432/demo
JWT_SECRET=your-jwt-secret-here
JWT_EXPIRES_IN=30d
ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173
```

## API 接口

| 方法   | 路径                    | 说明            | 认证     |
| ------ | ----------------------- | --------------- | -------- |
| `GET`  | `/`                     | API 信息        | 公开     |
| `GET`  | `/api/docs`             | Swagger 文档    | 公开     |
| `POST` | `/api/v1/auth/login`    | 登录获取 Token  | 公开     |
| `POST` | `/api/v1/auth/refresh`  | 刷新 Token      | 公开     |
| `GET`  | `/api/v1/users`         | 用户列表 (分页) | 需要认证 |
| `POST` | `/api/v1/users`         | 创建用户        | 需要认证 |
| `POST` | `/api/v1/common`        | 受保护接口示例  | 需要认证 |
| `POST` | `/api/v1/common/public` | 公共接口示例    | 公开     |
| `GET`  | `/health`               | 健康检查        | 公开     |
| `GET`  | `/metrics`              | Prometheus 指标 | 公开     |

## 示例接口

### 登录

```bash
curl -X POST http://localhost:3001/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"code": "123456"}'
```

```json
{
  "code": 0,
  "message": "success",
  "data": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

### 获取用户列表

```bash
curl -X GET "http://localhost:3001/api/v1/users?page=1&pageSize=20" \
  -H "Authorization: Bearer <token>"
```

### 健康检查

```bash
curl -X GET http://localhost:3001/health
```

```json
{
  "status": "ok",
  "timestamp": "2026-02-08T12:00:00.000Z",
  "uptime": "1234s",
  "environment": "production",
  "version": "0.1.0",
  "memory": { "used": "123MB", "total": "256MB" },
  "database": { "status": "connected" }
}
```

## 集成说明

### 认证

默认集成 JWT 认证，全局启用 `AuthGuard`。使用 `@Public()` 装饰器标记公共接口，使用 `@CurrentUser()` 获取当前用户。

```ts
import { Public } from '@/core/decorators/public.decorators';
import { CurrentUser } from '@/core/decorators/current-user.decorator';

@Public()
@Get('open')
getPublicData() { ... }

@Get('profile')
getProfile(@CurrentUser() user: JwtPayload) { ... }
```

### 数据库

使用 Drizzle ORM 进行类型安全的数据库操作。
默认物理表名采用复数命名，例如 `users`。

```ts
import { DrizzleService } from "@/service/drizzle/drizzle.service";
import { users } from "@/service/drizzle/schema";

@Injectable()
export class UserRepository {
  constructor(private readonly drizzle: DrizzleService) {}

  async findAll() {
    return this.drizzle.db.select().from(users);
  }
}
```

### 响应格式

所有接口统一返回格式（`/metrics` 和 `/health` 除外）：

```json
{ "code": 0, "message": "success", "data": "..." }
```

异常响应：

```json
{
  "code": 10001,
  "message": "错误描述",
  "data": null,
  "timestamp": "...",
  "path": "/api/v1/..."
}
```

### 业务异常

```ts
import { BusinessException } from "@/core/exceptions";
import { ErrorCode } from "@/core/constants";

throw new BusinessException(ErrorCode.USER_NOT_FOUND);
```

### 日志

使用 Pino 进行日志管理，按级别和日期自动切分日志文件，敏感字段自动脱敏。

```ts
import { logger } from "@/shared/utils";

logger.info("这是一条普通日志");
logger.error({ err }, "发生错误");
```

### 数据库命令

```bash
pnpm db:generate   # 生成迁移文件
pnpm db:migrate    # 执行迁移
pnpm db:push       # 推送 schema (开发环境)
pnpm db:studio     # 打开 Drizzle Studio
pnpm db:seed       # 初始化数据
```

## 测试

```bash
pnpm test           # 运行所有测试
pnpm test:cov       # 带覆盖率
pnpm test:watch     # 监听模式
pnpm test:e2e       # 端到端测试
```

## Docker 部署

```bash
# 构建镜像
pnpm docker:build

# 启动全部服务 (app + postgres)
docker compose up -d

# 仅启动应用
docker compose up -d app
```
