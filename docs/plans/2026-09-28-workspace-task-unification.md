# One CLI 工作区任务与 mise 缓存统一规划

> 后续调整：dev/build 已统一为 mise 任务入口，支持 `one <任务名>` 简写，项目使用重复的 `-p` 选择。开发 supervisor 与 TUI 已移除；当前行为见 [任务管理说明](../../apps/docs/content/docs/zh/run.md)。下文保留原规划。

日期：2026-09-28  
状态：本地实现已完成，检查与验收记录见第 16、17 节。Windows/macOS 原生运行和远端 Actions 验证待对应 runner 执行。

## 1. 已确认的方向

1. Node 项目使用 pnpm scripts，Go 项目使用 Taskfile，项目命令正文由对应工具管理。
2. mise 负责工作区层面的任务依赖、执行顺序、并行和构建缓存。
3. One 封装 mise，提供项目选择、环境与密钥、任务发现及结果展示。
4. 开启 mise 实验性功能，为满足条件的任务开启产物缓存。
5. 原来的任意命令执行入口改名为 `one exec`；`one run` 执行具名任务。
6. 直接采用新语义，不设计旧命令别名、旧语法猜测或迁移工具。
7. 已按确认范围修改功能、模板、工作流和中英文用户文档，不保留旧入口的兼容或迁移逻辑。
8. 移除 `one ci` 及工作流生成体系；保留仓库 Actions、非交互任务执行和普通 `ci` 聚合任务。
9. 移除现有 skills 功能，后续独立重做；不合并为 `one init skills`。
10. One 自动管理的 Node 项目统一使用 pnpm，删除 npm、yarn 和 bun 的适配分支。
11. 保留 hooks 集成和 preset 编码及创建能力。

保留 pnpm 和 Task 是最终分工。此前讨论中“逐步移除 Task”的建议由本方案替代。暂不引入 Turbo，也不自行实现缓存存储服务。

## 2. 目标与交付边界

### 2.1 目标

- 用户在日常操作和 Actions 中使用相同的 One 任务入口。
- 工作区依赖图由 mise 调度；One 不再另行调度同一组有限任务。
- 子项目可以独立执行 pnpm scripts / Taskfile 中的命令。
- 每条命令正文只维护一份；跨项目依赖和缓存规则在 mise 配置中可检查。
- 支持本地重复构建命中、删除产物后恢复，以及 GitHub Actions 的缓存复用。
- 环境、工具版本、项目依赖、脚本或锁文件变化能够正确使缓存失效。
- 中英文帮助、Dashboard、日志摘要、错误提示和文档同时更新。

### 2.2 本次实施包含

命令调整、通用任务能力、mise 配置生成、环境与缓存衔接、模板规范、Node 包管理器统一为 pnpm、CI 与 skills 功能移除、Actions 使用示例、One 自身仓库任务、hooks 接入、终端行为、跨平台验证和使用文档。

### 2.3 范围外

- Turbo 集成、自建分布式执行器、自建远程缓存服务。
- 修改包管理器或编译器自身的内部缓存。
- 恢复已经移除的部署、容器业务。
- 重做 skills 安装、分发或 Agent 集成。
- 旧命令兼容层和历史项目自动迁移。
- 在这次任务中重新设计账号、Infisical 登录或语言选择机制。

远程缓存客户端配置和验证属于方案；实际部署服务、申请凭据需要独立确定环境。GitHub Actions 先通过保存和恢复 mise 本地产物缓存完成跨运行复用。

### 2.4 功能精简范围

- `ci`：删除命令、应用服务、GitHub Actions provider 和 toolchain 中的工作流渲染接口，清理无用途的结果字段、文案和文档。已有仓库工作流继续维护，One 不再负责生成或启停用户工作流。
- `skills`：删除命令、安装器、Agent 目录映射、内置 skill 资源及同步/嵌入逻辑，清理帮助、文档、错误恢复建议和专属测试。
- 新工作区 `AGENTS.md` 的内容与生成逻辑同步调整，移除要求安装和使用已删除 skill 的指引。现有用户目录中已安装的 skill 与用户维护的 `AGENTS.md` 不属于本次删除范围。
- 清理 container/deploy/ci 遗留抽象；仍被环境配置与 Dashboard 使用的 backend schema 保留实际用途。
- 多包管理器适配：One 自动管理的 Node 项目仅支持 pnpm，删除 npm、yarn 和 bun 的识别、安装、锁文件选择、脚本调用与 hooks 适配分支，同步配置校验、文档和测试。
- hooks：保留 `one hk`、`one init hooks`、配置生成、安装、暂存文件处理和提交信息检查；检查定义继续与统一任务体系衔接。
- preset：保留 `one create --preset`、编码与解析、模板映射以及官网模板组合入口。

## 3. 实施前的实现与本次变化

| 当前实现 | 目标变化 |
| --- | --- |
| `transport/cobra/run` 执行任意命令 | 移至 exec，保留环境、参数、信号和退出码语义 |
| `modules/build` 自行建立依赖并调用 taskrun 调度 | 项目识别逻辑用于生成 mise 依赖；有限任务由 mise 执行 |
| miseconfig 只发现 dev/build/test/lint | 增加完整任务发现、根聚合任务、依赖与缓存声明 |
| mise 任务通过 `__exec --operation` 末端加载环境 | 建立缓存计算前已冻结的环境上下文；执行时消费同一份数据 |
| Node 任务来自 package scripts | 保留，扩展任意具名 script |
| Go 任务来自 Taskfile | 保留，支持具名任务和标准检查约定 |
| 用户 CI 由 Node / Go adapter 各写一套步骤 | 删除生成器；文档示例通过工作区任务接入 Actions |
| skills 安装器分发内置 skill，并由生成的 AGENTS.md 引用 | 移除当前功能及生成指引中的强制依赖，后续独立重做 |
| 仓库根 Taskfile 编排 Go、Dashboard、资源同步 | mise 编排跨项目流程，Go 子项目保留 Task |
| 单任务 auto 使用 raw，多任务使用 One TUI | 可缓存有限任务使用 mise 可捕获输出；交互任务单独处理 |

调查中确认的细节：

- 当前仓库没有 turbo.json、turbo 依赖或 turbo run 执行路径。
- One 托管的 mise 版本和最低要求目前均为 2026.9.7。
- Go 模板的 check 包含 go mod tidy；需要移出只读检查。
- go-api Taskfile 声明 dotenv；必须核实并统一它与 One 环境选择的优先级。
- Expo 模板的 test 默认 watch；需要单独提供有限测试与交互测试入口。
- Electron 是复合项目，内部已有 pnpm 子包编排；One 应将其整体任务作为一个项目边界。
- One 自身编译前必须准备 Go embed 的模板、注册表和 Dashboard 资源。
- 当前 CI 生成器只按项目路径触发，且部分版本和锁文件路径单独声明；生成器将删除，新的 Actions 示例需要覆盖根配置和上游变化。

## 4. 命令约定

以下为本次实现的命令语义。用户使用说明见 `apps/docs/content/docs/zh/run.md` 和对应英文页面。

### 4.1 one exec：任意命令

~~~sh
one exec web -- pnpm add axios
one exec api -- go mod tidy
one exec api --env prod -- ./bin/server
one exec --global --env dev -- command
~~~

- 复用现有项目名 / 路径解析、环境选择、global 能力、PATH 处理和退出码。
- `--` 后始终是原始命令与参数；使用参数数组传递。
- 不参与任务图，也不承诺任务产物缓存。
- 不自动执行应用依赖安装，保持任意命令入口的可控性。
- 命令、日志和协议中的相关 run 引用同步改为 exec；内部协议入口是否保留 `__exec` 由实现组织决定。
- 稳定错误码、JSON 字段名不要仅因命令改名机械重命名；新增任务输出使用独立 schema。

### 4.2 one run：具名任务

~~~sh
one run
one run --list -p web
one run build
one run build -p web
one run build -p web -p api
one run check
one run test -p api
one run test -p web -- --watch
one run docs:build
one run build --dry-run -o json
one run build --concurrency 4
one run build --cache off --force
~~~

约定：

- 无任务名时列出任务，展示名称、项目、来源、描述、交互属性和缓存配置；不自动执行 default 任务。
- `-p/--project` 可以重复，支持项目名和相对路径，去重后保持可解释的顺序。
- 无 `-p` 时解析工作区根任务；标准 build/test/check 等由根任务明确聚合。
- 有 `-p` 时解析各项目同名任务；显式选择的任一项目缺少任务时，在执行前报错。
- 未识别的根任务直接报错并提示任务列表，不猜测 shell 命令。
- `--` 后为任务参数。参数只传给请求的入口任务，不自动广播给其依赖。
- 多项目同时传任务参数时拒绝执行，提示分别运行；自定义聚合任务可通过自己的参数定义明确分发。
- `--env` 沿用 One 的环境含义，不直接映射成 mise 的配置环境参数。
- `--concurrency` 映射 mise jobs；默认保持 1，允许显式提高。
- `--cache` 对应 mise 的 read-write/read-only/write-only/off/local-only 模式；默认 local-only，直到明确配置远程服务。
- `--force` 绕过新鲜度跳过；要保证实际执行，使用 `--cache off --force`。
- `--dry-run` 不安装工具或依赖、不联网获取密钥、不恢复或写入缓存、不运行任务输入命令。
- dry-run 展示请求项目、展开的依赖、任务来源、工作目录、命令、缓存声明和“命中情况尚未计算”。
- 不复制完整 mise 模板解释器。静态预览遇到动态原生配置时，明确标注无法解析的部分；无法确定执行范围时返回具体原因。

### 4.3 常用入口

| 命令 | 目标关系 |
| --- | --- |
| one build web api | 与 one run build -p web -p api 共用计划和执行实现 |
| one dev web api | 共用任务发现、工具与环境准备；常驻服务生命周期见第 10 节 |
| one mise ... | 保留原生 runtime 诊断、信任、缓存检查入口 |
| one init mise | 生成或刷新 One 管理的任务配置，支持静态 dry-run |

`one build` 与 `one run build` 必须具有相同的任务集合、依赖、产物、缓存和退出行为。

## 5. 配置归属与任务来源

### 5.1 文件布局

~~~text
workspace/
  one.manifest.json
  package.json
  pnpm-workspace.yaml
  pnpm-lock.yaml
  mise.toml                         工作区工具、聚合任务与自定义配置
  .config/hk.pkl                    工作区检查与 Git hooks
  apps/web/
    package.json                    Node 命令正文
    mise.toml                       项目任务引用、自定义任务与缓存
  services/api/
    go.mod
    go.sum
    Taskfile.yml                    Go 命令正文
    mise.toml                       项目工具、任务引用与缓存
~~~

项目配置放在子目录，是为了让 mise 采用对应项目的工具与环境作用域；整次工作区运行仍由根启动的一个 mise 调度器管理。[mise monorepo 配置](https://mise.jdx.dev/tasks/monorepo.html)

锁文件本轮明确保持每个配置根独立的现有模式，设置 `monorepo.lockfile = false`。提交和验证全部相关 mise lockfile，不同时引入统一 monorepo lockfile 布局变更。精确工具版本、平台解析结果和 CI 安装参数必须相互匹配。

生成配置可直接编辑。One 通过文件内的跟踪注释更新未改动的默认项，保留用户修改与注释；同一项双方都有变更时报告冲突。

### 5.2 权威来源

| 内容 | 来源 |
| --- | --- |
| 项目身份、路径、环境绑定 | one.manifest.json |
| Node 命令正文 | package.json scripts |
| Go 命令正文 | Taskfile.yml 与其声明的包含文件 |
| 工具版本 | mise 配置；pnpm 默认从根 packageManager 派生 |
| 跨项目任务依赖 | 最终生效的 mise depends 等声明 |
| 输入、产物、缓存策略 | mise 任务配置 |
| 自定义工作区任务 | 用户根 mise.toml 或 mise-tasks |
| 自动生成规则 | One 模板元数据和任务配置生成器 |

One 自动管理的 Node 项目统一使用 pnpm。删除 npm、yarn 和 bun 的自动适配；`packageManager` 继续作为 pnpm 版本来源。遇到声明其他包管理器的项目时，在自动准备或执行前明确提示不支持，不静默按 pnpm 执行，也不自动迁移或改写锁文件。`one exec` 的任意命令透传能力保持不变。

### 5.3 发现与优先级

- Node scripts 扩展为任意具名任务；标准名称单独用于根聚合。
- Go 读取静态 Taskfile 任务。对本地 includes 解析命名空间和文件依赖；动态 includes/模板化任务声明不得静默漏掉，提示显式声明 mise 任务。
- 原生自定义 mise 任务由 mise 发现；运行时优先使用其结构化任务清单，不重新实现执行语义。
- One 无 runtime 时可列出静态发现的项目任务，标注原生动态任务尚未解析，不为“列任务”下载工具或登录。
- 可用 runtime 下，验证 `mise tasks ls --all --local --json` 的固定版本行为及无副作用条件。
- 优先级为用户显式 mise 任务覆盖生成任务，生成任务引用项目原生命令。
- 对同名任务覆盖，显示实际来源；不把多个来源的 run 数组拼接。
- One 不写用户 mise.toml；生成片段继续使用内容校验、冲突报告、文件锁和原子写入。
- create/add/init mise 更新生成配置。run/build 执行前检查已管理配置的输入变化，按现有管理规则刷新；dry-run 只展示拟刷新内容。
- Taskfile/package scripts 的正文在叶子执行时读取并核对本次输入版本，避免配置生成时复制一份过期命令。

### 5.4 命名

项目任务采用 mise 的原生路径形式，例如：

~~~sh
mise run //apps/web:build
mise run //services/api:test
~~~

One 将 `-p web` 映射成对应路径。无需额外创造 build:web / web:build 两套别名。

根任务 build/check/test 引用明确的项目任务列表。根自定义名称如 docs:build 可以正常使用，项目内同名任务通过 `-p` 选择。

根聚合任务不缓存。项目叶子任务按策略缓存。

### 5.5 配置结构示例

下例说明任务和缓存的组织方式，省略工具精确版本、管理标记及 One 环境上下文桥接；不能直接当成完整生成结果。

~~~toml
# workspace/mise.toml
monorepo_root = true

[monorepo]
config_roots = ["apps/web", "services/api"]
lockfile = false

[settings]
experimental = true

[tasks.build]
depends = ["//apps/web:build", "//services/api:build"]

[tasks.check]
depends = ["//apps/web:check", "//services/api:check"]
~~~

~~~toml
# apps/web/mise.toml
[tasks.build]
run = "pnpm run build"
sources = [
  "src/**", "public/**", "index.html",
  "package.json", "vite.config.*", "tsconfig*.json",
  "../../package.json", "../../pnpm-lock.yaml", "../../pnpm-workspace.yaml",
]
outputs = ["dist"]
cache = { enabled = true, env = ["NODE_ENV", "VITE_API_URL"] }
~~~

实际带 One 环境能力的叶子允许使用轻量内部入口。该入口只负责已准备环境和原生 argv，不重新进入 mise，也不调度其他项目。

## 6. 任务图与执行流程

### 6.1 任务图

- Node 根据真实 package 名称及 workspace/file/link 依赖生成构建关系，复用当前代码中的解析能力。
- 只对真实需要先构建的项目产生 build 依赖。源码直接被消费的共享包即使没有 build，其源码也必须进入下游缓存输入。
- Go 的 go.work 仅说明模块协作，不代表必须先分别 go build。编译器可自行处理的依赖不生成额外任务。
- Go 跨模块源码、replace 指向的本地源码必须纳入相关缓存输入。
- 代码生成、跨语言协议等关系在 mise depends 中显式声明。
- Electron 复合项目按一个项目任务调度，项目内部继续由 pnpm 管理；覆盖所有内部产物和输入。
- 不启用 mise 自动推断和 One 推断两份并行关系。首版由 One 生成显式任务边，mise 是执行图的唯一调度者。
- 检测重复包名、未知任务、循环、工作区外路径和输出冲突。

### 6.2 选择项目与依赖闭包

本方案选择：显式构建一个项目时，执行其声明的必要任务依赖。

因此，`one build web` 在 web 依赖 shared 构建时会包含 shared。这是相对当前实现的明确行为变化，dry-run 和帮助必须展示。不提供默认跳过依赖的第二套模式。

根“全部项目”任务只聚合有相应能力的项目，并在清单中说明缺失能力。显式选择缺少任务的项目直接报错。全部项目均无该任务时失败，不能把空集合显示成检查通过。

### 6.3 一次运行

1. 解析命令、项目和任务，检查生成配置及冲突。
2. 解析入口和依赖闭包；静态预览在此返回。
3. 使用现有 runtime resolver 准备固定的 mise，完成必要的配置信任处理。
4. 确定项目工具作用域和应用依赖准备范围。
5. 一次性准备所需项目环境上下文，完成缓存输入核对。
6. 从工作区根启动一次 mise run，将请求的入口和并行限制交给 mise。
7. mise 决定依赖顺序和缓存命中，未命中的叶子运行 pnpm / Task。
8. One 处理外层取消、日志展示和结构化结果，清理本次上下文。

One 不通过“每个项目分别启动 mise”重新实现有限任务调度。多个入口在一个 mise 调用中提交，共享依赖应只运行一次。

### 6.4 失败和退出

- 首版使用 mise 默认失败策略；具体在固定版本中验证后写入 CLI 文档。
- 依赖失败时，下游不运行；独立任务的停止行为由同一次 mise 调度管理。
- 任一必需任务失败，One 返回非零；保留 mise 的实际退出码，不另按项目排序伪造失败码。
- 用户中断清理整个子进程树；验证 Unix 信号和 Windows Job Object。
- 取消后不能留下服务、下载锁或环境上下文占用。
- 缓存损坏按 mise 的未命中策略处理并可诊断；不能显示为任务成功恢复。

## 7. One 环境变量与缓存的衔接

这是实施的第一项技术验证，必须先于批量开启缓存。

### 7.1 正确性要求

- 同一任务的缓存判定和实际执行使用同一份项目环境。
- 多项目可使用相同变量名和不同值，彼此隔离。
- 不把项目密钥合并成全局环境后传给全部项目。
- 构建相关值变化必须失效，例如 API 地址、功能开关、Go 构建目标、构建标签。
- 只用于认证且不影响结果的凭据可以透传；依赖其远端数据的任务还需要声明数据版本，否则不缓存。
- One 的 --env 与 mise 的配置环境是不同维度，不能混用。
- 缓存命中仍需具备确定有效输入的能力；不能在远端环境读取失败时无依据复用旧结果。

### 7.2 已采用的实现

为一次 one run 准备按项目划分、不可变的运行上下文：

1. One 解析环境名与绑定，提前获取本次使用的项目变量。
2. 上下文同时保存项目、操作映射、环境输入分类和读取的配置修订。
3. mise 的 `cache.command_inputs` 调用 One 内部只读入口，取得影响结果的有效环境及执行器版本的稳定指纹。
4. 叶子执行入口读取同一个上下文，将 One 值覆盖到该任务已经由 mise 准备的环境中，并执行原生命令。
5. 叶子不再次读取 Infisical，不重新选择环境，不再次调用 mise。

上下文用受限权限的临时存储或本机 IPC 传递；具体方式在原型中选择。路径或会话标识只作继承环境的定位信息，不写入任务正文、sources 或缓存指纹，避免每次运行产生新的缓存键。敏感值不写入受版本控制的 TOML、命令行、计划输出或缓存解释结果。

指纹覆盖环境名、绑定身份、影响结果的有效值、项目操作解析协议和执行器实现版本。变量名排序，区分未设置与空字符串。依赖 mise task.env 的最终覆盖顺序必须实测；命令输入与真正执行必须一致。

这是一种待验证的桥接方案，不声称现有 One 已支持。验证其单次读取、一致性、跨平台和缓存稳定性后再采用；若当前 mise 无法提供正确的计算时机，则相关任务保持缓存关闭并报告原因，不能用省略输入来获得命中。

### 7.3 原生 mise 使用边界

- 用户自定义的纯 mise 任务可以直接 `mise run`。
- 依赖 One 项目环境的托管任务以 `one run` 为完整准备入口。
- 直接原生运行托管任务且缺少上下文时，在缓存查找前明确失败并提示对应 one run 命令。
- 不能让“原生运行”悄悄使用另一套环境后仍共享缓存。
- `one mise` 继续作为原生转发入口，不暗中等同于 one run；用户文档说明区别。

### 7.4 工具和环境优先级

复用“进程环境 → mise 对应项目环境 → One 当前项目变量 → node_modules/.bin 路径补充”的执行意图。对于会改变可执行文件或工具链选择的变量，必须在工具准备和缓存计算时一致处理。

Go 模板的 Task dotenv 和框架自动加载的 .env 文件属于实际输入。统一验证 prod/dev 选择和覆盖顺序；不能只缓存 One 变量而漏掉底层再次加载的配置。

## 8. 缓存设计

### 8.1 默认策略

工作区启用 experimental。首版仅为已知模板的 build 自动开启缓存，且要求构建脚本与影响输出的配置仍匹配内置模板。自定义配置在项目 mise.toml 中声明完整输入、输出和环境指纹后可开启；其他任务默认关闭。

| 任务 | 默认缓存 | 条件 |
| --- | --- | --- |
| build | 开启 | 有完整输入及明确产物 |
| 确定性代码生成 | 按配置开启 | 工具、schema、参数、产物完整 |
| lint/typecheck/只读 check | 按配置开启 | 无未声明文件副作用 |
| 有限单元测试 | 按配置开启 | 无外部服务依赖，稳定输入 |
| 生成覆盖率的测试 | 按配置开启 | 声明覆盖率文件 |
| 集成测试 | 关闭 | 明确外部状态版本后才能开启 |
| dev/watch | 关闭 | 常驻或交互任务 |
| 依赖安装、fmt/fix/tidy | 关闭 | 会修改依赖或源码 |
| release/publish | 关闭 | 外部副作用 |
| 根聚合任务 | 关闭 | 各叶子独立缓存 |

产物缓存是对整个任务成功结果的复用。编译器和包管理器自身缓存继续保留。

### 8.2 输入与产物

- Node：源码、静态资源、scripts、框架和 TypeScript 配置、根锁文件与 workspace 配置。
- Go：源码、go.mod/go.sum、go.work/go.work.sum、Taskfile 与 includes、构建标签、GOOS/GOARCH、CGO 条件，以及相关本地模块源码。
- 工具由 mise 声明；系统 C 编译器等未被 mise 管理的工具需要额外版本输入，无法确定时不启用该任务缓存。
- 托管任务加入影响行为的 One 配置、操作协议版本或执行器指纹。
- 不把整个 node_modules、整个工作区或全部密钥粗暴当作默认输入。
- 不把随机临时路径、运行 ID、日志时间、checkout 绝对路径当作自定义缓存输入。
- 产物路径限定在任务目录内；仓库级产物使用明确的根任务目录。
- 同次并行任务不得写入重叠产物目录。
- 声明完整目录或精确文件，包含必要的执行权限、平台后缀、源映射和生成声明。
- 不能仅凭目录名假设框架产物：Vite、Next、Astro、Electron、Go 分别验收。
- `outputs = []` 仅适用于命中后不需要恢复任何文件的任务。TS 增量文件、覆盖率和 Go 导出二进制需要分别判断。
- 同次运行前后关键输入发生变化时，不发布可复用结果；工具版本、配置和环境上下文一致性由原型验证。

### 8.3 消除双重跳过

模板中不再为同一构建任务维护一套与 mise 重复的 Task sources/status 缓存策略。Task 保留项目内部步骤和 Go 自身编译缓存。

依赖准备放在本次调度前的独立阶段，不作为每个缓存叶子的“永远执行且无稳定键”的 depends，避免下游缓存每次被失效。

### 8.4 本地与 CI

- 本地默认使用 mise 本地产物缓存。
- GitHub Actions 第一版保存 / 恢复专用的 mise 产物缓存目录，mise 仍在恢复后检查每个任务的缓存键。
- GitHub 缓存的外层 key 只负责存储分组和更新，不能替代 mise 的输入判断。设计不可变条目的更新策略，避免命中同一外层 key 后新产物永远无法保存。
- OS、架构、mise 版本和缓存格式纳入外层分组；任务内容由 mise 自己校验。
- 工具下载、pnpm store、Go 模块 / 编译缓存、mise 产物缓存分别配置。
- 空缓存运行必须完整通过。缓存服务或恢复失败应退化为执行。
- 远程协议可作为后续同一缓存模块的可选配置；接入时验证该版本的写入条件、认证和命名空间。[缓存配置](https://mise.jdx.dev/tasks/caching.html)
- 日志和产物都可能进入缓存；只允许缓存适合相应共享范围的输出。

## 9. 项目模板与标准任务

### 9.1 约定名称

- dev：常驻开发。
- build：有限构建。
- check：只读静态检查。
- test：有限测试。
- test:watch：交互测试。
- fmt/fix/tidy：显式修改。
- 其他自定义名称原样保留，不强行重命名用户脚本。

根 check 汇总项目 check；根 test 汇总项目 test；根 ci 明确依赖 check、test、build。不能把同一测试同时塞进 check 和 test，再在 CI 重复运行。

缺少 test 的项目正常列为“未提供测试任务”；CI 结果不能把它记作测试通过。

### 9.2 Node 模板

- 校准各模板 check / format 的只读含义。
- Expo 的默认 test 改成有限执行，watch 独立。
- 编译和类型检查产生的额外文件纳入 outputs 或关闭对应缓存。
- Electron 顶层脚本继续负责 preload/electron/ui 内部编排，避免外层重复注册其内部包为独立 One 项目。
- 任意脚本可发现，但 release、迁移数据库等脚本不自动进入根 ci 或启用缓存。
- pnpm 的 pre/post 脚本及框架配置也属于任务实际行为和缓存输入。

### 9.3 Go 模板

- 保留 Taskfile。
- check 移除 go mod tidy 和写文件格式化；测试由 test 独立提供。
- 对齐 fmt:check、vet/lint 等命名，并处理 Windows shell 差异。
- 向任务传递参数时使用 Task 支持的参数方式；模板明确消费 CLI_ARGS，验收空格、中文和引号。
- go-api 声明实际二进制及 Windows 后缀；go-lib 的 go build ./... 不能假造 bin 输出。
- go-lib 如仅验证可编译且无需恢复产物，可以将该任务按无产物检查配置。

## 10. 终端、日志和 dev

### 10.1 有限任务

缓存执行需要 mise 捕获输出；其 raw/interactive 模式会绕过产物缓存。[固定版本缓存文档](https://github.com/jdx/mise/blob/v2026.9.7/docs/tasks/caching.md)

默认行为：

- build/check/有限 test 的 auto 使用 mise 可缓存的输出方式。
- 单任务尽量保持原始输出字节、ANSI 和 stdout/stderr；不能宣称管道完全等价于真实 TTY。
- 明确请求 raw 或执行 interactive 任务时，关闭该次产物缓存并显示原因。
- `-o json/yaml` 的 stdout 仅输出最终结构化结果，子进程日志写 stderr。
- 缓存统计和任务结果只使用可靠的机器接口或受测试的事件机制；不从彩色日志猜测缓存命中。
- 无法取得逐任务状态时先提供整体退出结果，并标注逐任务状态未知，不能编造 succeeded/cached。

### 10.2 TUI

固定版本未提供本实现可依赖的完整结构化运行事件。有限任务采用 stream；单任务结果状态保持 unknown，整体成功、失败、取消与退出码来自实际进程。开发任务继续使用原有 TUI。

- 优先使用 mise 作为唯一调度器，One TUI 仅观察事件和展示日志。
- 不为保留 TUI 而在 One 中重新调度相同任务图。
- 若拿不到可靠事件，有限任务采用 stream；明确记录该体验变化，不承诺原有逐项目 TUI 已保留。
- 后续扩展输出适配的前提是可靠标识真实执行、缓存恢复、失败和取消。

### 10.3 dev / watch

One 现有开发 supervisor 保留服务输入、重启和整组清理能力。它只管理常驻服务生命周期。

- dev 使用同一任务发现与项目环境准备。
- 有限前置构建作为一个 mise 工作区任务图先运行。
- 开发服务各自使用已准备的环境和原生命令，不另建有限构建调度图。
- 单项目保持原生终端；多项目沿用现有 PTY/TUI 和平台降级。
- `one run dev`、`one run dev -p web` 进入相同的开发执行路径。
- `one run test -p web -- --watch` 按显式交互模式处理并关闭缓存；不能把未知任意参数都猜成 watch。
- 长任务不得被根 ci 聚合。取消、输入和孙进程清理继续跨平台验证。

## 11. 工具与应用依赖准备

mise 安装工具；pnpm / Go 安装项目依赖；One 协调本次实际需要的准备范围。

- 工具版本继续使用已有配置来源，pnpm 版本与根 packageManager 保持一致。
- build/check/test/CI 使用 frozen lockfile / readonly 语义；开发准备沿用已有允许同步的规则。
- build 与 run build 共享准备阶段，不重复 pnpm install。
- Node 在工作区根解析和安装；保留复合项目 workspace 成员规则。
- Go 使用真实 go.work 与本地 replace 关系，保留防止依赖准备悄悄改写 go.mod 的处理。
- 单独运行 one exec 不隐式安装应用依赖。
- 准备阶段不是可缓存的副作用叶子；成功后才启动需要该依赖的任务图。
- 首版允许缓存命中前完成依赖准备，优先保证一致性。缓存命中免安装是后续优化，不能破坏干净机器运行。

## 12. Actions、hooks 与 One 自身仓库

### 12.1 用户工作区的 Actions

删除 `one ci`、CI provider 与工作流生成器，提供由用户维护的 Actions 示例。示例从工作区根目录执行，流程为：

1. checkout。
2. 准备固定 One 和 mise；工具版本读取仓库配置。
3. 恢复工具、语言依赖和任务产物缓存。
4. 执行 one run ci，或由多个 job 分别调用同一组 check/test/build。
5. 保存允许写入的缓存、测试报告和构建产物。

`one run ci` 中的 ci 是普通 mise 聚合任务；不提供独立的 CI 管理服务。仓库已有工作流继续维护。

示例与执行约束：

- 不再按语言复制 pnpm build / task build 的业务步骤。
- 默认 PR 全工作区检查，先保证上游变化不漏检。受影响项目优化留到依赖图覆盖证明之后。
- 工作流路径条件包含根锁文件、manifest、mise 配置、Taskfile、上游源码和生成器相关文件。
- CI 非交互，版本和锁文件固定；缺少必需环境时快速返回具体项目与变量名，不输出值。
- 默认检查不要求个人 Infisical 交互登录；确需远端变量的项目使用配置好的 CI 身份。
- 同一根目录和同一配置运行，本地任务与 CI 任务结果一致。
- PR 和主分支缓存写入范围、凭据和产物共享范围显式配置；远程缓存协议接入独立验证。

### 12.2 hooks

- 保留 `one hk` 与 `one init hooks`，以及配置生成、Git hook 安装、暂存文件处理、修复和提交信息检查能力。
- 全工作区检查复用统一任务；文件级检查继续可由 hk 传文件列表调用原生项目任务。
- 不把带外部副作用的任务塞进 pre-commit。
- 防止 check → hk → check 递归。
- hooks 只负责何时检查，任务定义仍来自项目和 mise。

### 12.3 One 自身仓库

One 自身的贡献者和 CI 入口使用 `mise run`，保证首次构建不依赖已安装 One。

- 根 mise 定义 check、check:static、check:test、build、test、test:go（含 race）、docs:build、dev、install、pre-push 等任务。
- Go 子项目的命令下沉至 packages/cli 和 packages/kernel 的 Taskfile。
- Dashboard / Docs 保留 package scripts。
- 根 mise 明确编排资源同步、Dashboard 构建、Go embed 同步、Go 检查和构建。
- 根 Taskfile 不再承担工作区编排；最终删除对应重复入口，而不是长期套一层 mise run → task check。
- .githooks/pre-commit、CONTRIBUTING、README、根 package scripts、AGENTS 的验收入口同步更新。
- docs.yml、ci.yml、发布流程和 GoReleaser 的前置资源路径一起校准。
- 文档站外部托管的 build/install 配置核实实际平台能力后更新；本地规划不直接改线上项目设置。

切换验收入口之前，实施各阶段仍执行当前仓库要求的 `task check`；切换之后执行与新 AGENTS 一致的 mise 检查。

## 13. 代码改动地图

| 模块 | 主要变化 |
| --- | --- |
| transport/cobra/run | 旧任意执行实现迁到 exec；新 run 参数和任务输出 |
| transport/cobra/build | 作为通用任务服务的快捷入口 |
| transport/cobra/dev | 接入共同任务发现和环境准备，保留服务生命周期 |
| bootstrap/cli | 注册 exec/run，移除 ci/skills 注册与依赖，更新命令帮助和隐藏协议 |
| application/execution | 通用任务选择、操作解析、环境上下文边界 |
| modules/build | 提取任务图生成能力，移除重复有限任务调度 |
| 新任务应用服务 | 统一 catalog/plan/prepare/execute，包位置服从现有架构检查 |
| modules/miseconfig | 根聚合、项目任务、depends、输入输出、实验性缓存配置 |
| adapters/runtime/mise | mise run 调用、参数与取消、版本能力检查、结果适配 |
| modules/dependencies | 共用准备与去重，保留严格依赖策略 |
| platform/taskrun | 分离有限任务调度与 dev 会话/显示能力 |
| adapters/toolchain + pkg/toolchain | Node 仅保留 pnpm 适配，保留 Go/Task 与项目操作元数据，删除工作流渲染接口及实现 |
| transport/cobra/ci + application/ci + adapters/ci + pkg/ci | 删除 CI 命令、生成器及 provider 体系 |
| transport/cobra/skills + modules/skills + skills 资源 | 删除当前 skill 分发功能及资源同步/嵌入逻辑 |
| modules/creation | 移除生成指引中的 skill 安装要求，清理已无用途的 CI 结果字段 |
| modules/hooks | 保留集成与命令入口，Node 检查统一 pnpm，统一检查引用，避免递归和重复门禁 |
| application/workspace + Dashboard | 展示命令来源、有效任务、依赖和缓存配置 |
| packages/templates | 标准任务、有限测试、只读检查和缓存元数据 |
| locales/help snapshots/docs | exec/run 新语义及中英文完整更新，清理 ci/skills 引用 |
| 根构建文件与 .github | mise 贡献者入口、自举与发布链路 |

Dashboard 首版展示已配置能力和实际命令来源，不新增长驻任务服务器，也不把未知缓存状态展示成命中。用户修改命令仍落在既有权威配置中，不另存一份 Dashboard 专用命令。

## 14. 实施顺序与每阶段出口

### 阶段 0：固定版本技术验证

使用临时 fixture，验证：

- mise 2026.9.7 的 monorepo、配置覆盖、目录、锁文件和工具作用域。
- 一次 mise 调度多个入口，共享依赖只执行一次。
- 本地产物缓存、删产物恢复、上游失效、参数变化及缓存关闭。
- 环境上下文方案：两个项目同名变量隔离、相同输入跨次命中、变量变化失效。
- command_inputs 与实际运行读取同一环境，真实 One 更新会使缓存失效。
- raw/interactive、stdout/stderr、信号、退出码和日志接口。
- CI 保存恢复缓存目录能否在新 checkout 中复用。
- 动态配置的静态预览边界。

出口：记录经过真实二进制验证的能力矩阵、输出策略和环境桥接选择。文档示例不是测试结果。若必须升级 mise，同步版本、摘要、最低版本和各平台验证，不假设“更高版本一定兼容”。

### 阶段 1：命令与通用任务入口

- run 改 exec。
- 新 run 的 catalog、选择器、参数、dry-run 和结果 schema。
- build 复用通用服务；内部调用、恢复建议和测试同步。
- 删除 ci/skills 命令和对应实现，清理内置资源与新工作区指引中的依赖。
- Node 包管理器统一 pnpm，清理其他适配分支、校验、帮助与测试；保留 hooks 和 preset。
- 同步中英文与帮助快照。
- 此阶段先不宣称缓存可用。

出口：命令语义明确，任意命令和具名任务不能混淆，dry-run 无副作用。

### 阶段 2：mise 工作区调度

- 生成根聚合与项目叶子。
- 导出完整依赖图，处理复合项目和显式构建的依赖闭包。
- 共用工具及应用依赖准备。
- 单个 mise 调度有限任务，删除旧 build 调度路径。
- dev 接入共享解析与准备，验证常驻服务边界。

出口：本地 Node + Go 混合工作区正确运行，失败和取消行为明确。

### 阶段 3：环境与本地缓存

- 落实阶段 0 选定的环境桥接。
- 新模板缓存声明及输入输出验证。
- Go check 去除修改操作，Expo 有限测试，Task 双重跳过处理。
- 缓存控制、诊断与统计；清理上下文。

出口：验收矩阵中的失效、恢复、隔离和不缓存任务均通过。

### 阶段 4：Actions 与 hooks

- 用户维护的工作区级 Actions 示例；验证不再生成或管理用户工作流。
- 缓存目录恢复、外层 key 更新策略和空缓存验证。
- 非交互环境准备、报告与产物。
- hooks 复用检查，必要的文件级能力继续保留。

出口：干净 runner 和本地执行同一任务，重复运行与变化运行均正确。

### 阶段 5：自身仓库采用与收尾

- 根 mise 与 Go 子项目 Taskfile。
- 资源、Dashboard、Docs、CLI、kernel、发布的任务关系。
- 更新 AGENTS、贡献文档、hooks 和工作流，核查 skills 资源与引用已清理。
- Dashboard 展示和中英文 mise 使用说明。
- 清理废弃入口、重复配置和内部 dead code。

出口：首次 clone、无已安装 One、Windows/macOS/Linux 均可构建与检查；发布链路保持可用。

阶段可以分步开发和验证，最终交付采用新命令语义，不保留旧命令迁移流程。

## 15. 验收矩阵

| 场景 | 必须满足 |
| --- | --- |
| one exec 任意命令 | argv、cwd、环境、原日志和退出码保持正确 |
| run 未知任务 / 显式缺少任务 | 准备或执行前失败，给出具体项目和恢复说明 |
| run 无参数 | 列出实际任务来源，不安装或获取密钥 |
| dry-run | 无网络、安装、密钥读取、任务或缓存副作用 |
| build 与 run build | 相同计划、产物、缓存结果和失败结果 |
| web 依赖 shared | 先满足 shared；共享依赖只运行一次 |
| 依赖循环 | 不启动任务，指出循环路径 |
| 两次相同输入 | 第二次命中，叶子命令不再执行 |
| 删除产物 | 恢复完整目录、执行权限和平台文件 |
| 源码 / 脚本 / Taskfile / 锁文件变化 | 对应任务失效 |
| 共享包或 Go 本地模块变化 | 正确影响下游 |
| 环境值变化 | 相关任务失效，其他项目环境不串用 |
| 环境未设置与空值 | 指纹可区分，结果可验证 |
| CLI 参数 / 工具 / One 执行器变化 | 对应缓存失效 |
| 缓存关闭并强制运行 | 实际执行，便于诊断 |
| 失败或中断 | 不保存成功结果，不留下进程或上下文 |
| 重叠 outputs | 执行前阻止不安全的并行写入 |
| fmt/tidy/install/dev/watch | 不因产物缓存跳过真实操作 |
| cached check/test | 未声明副作用不能遗漏；报告产物可恢复 |
| CI 冷启动 / 恢复缓存 / 缓存不可用 | 均能正确完成或报告实际任务失败 |
| 新 checkout / 路径含空格中文 | 路径、argv、缓存复用正确 |
| JSON/YAML 模式 | stdout 仅协议数据，日志不污染结果 |
| raw/TUI/stream | 缓存边界明确，状态真实，Ctrl-C 可用 |
| Windows/macOS/Linux | 原生运行验证，交叉编译不足以替代 |
| 中英文 | 字典键、占位符、帮助快照、切换与窄屏通过 |
| One 自身空仓库构建环境 | 无现成 One 二进制也可准备 embed 并构建 |
| 所有外部调用入口 | Actions、hooks、Dashboard 和文档均使用新语义 |
| ci/skills 移除 | 命令、专属实现、内置 skill 资源与失效安装指引均已清理，现有 Actions 可运行 |
| Node 统一 pnpm | 新项目、依赖准备、任务与 hooks 使用 pnpm；非 pnpm 声明在自动执行前明确报错，无静默迁移 |
| hooks 与 preset 保留 | hooks 暂存文件与提交检查可用，官网 preset 命令仍能创建对应项目组合 |

测试采用临时 HOME/config/state/cache 与本地环境 fixture。真实 mise 集成测试固定二进制版本；普通单元测试不依赖真实凭据或外网。

实施中执行受影响测试和当前仓库完整门禁；新的缓存测试验证“命令是否真的执行”和恢复内容，不能只检查输出中出现 cache hit。

## 16. 交付清单与本地验证

- [x] exec / run / build / dev 命令、中英文帮助与快照。
- [x] 单个 mise 进程调度有限任务；build 与 run build 共用服务。
- [x] pnpm scripts、Taskfile 与本地 includes、复合项目依赖发现。
- [x] 托管配置覆盖、幂等、变更冲突、循环与输出重叠校验。
- [x] 环境在查找缓存前冻结，每项目读取一次，临时文件退出时清理。
- [x] 缓存指纹包含注入环境、操作与 One 执行器；实际运行通过 mise tasks info 校验生效的指纹命令。
- [x] 已知模板 build 缓存、Go 只读 check、CLI_ARGS、Windows GOEXE、Expo 有限测试。
- [x] 本地缓存命中、删除产物后恢复、环境失效、强制执行及跨 checkout 复用。
- [x] 移除 ci/skills 命令、provider、生成器、资源与安装指引。
- [x] Node 统一 pnpm；hooks 和 preset 的现有测试继续通过。
- [x] Actions 使用 mise、保存恢复本地产物缓存；用户工作区示例同步。
- [x] 根 mise 自举、Go 子项目 Taskfile、hooks、贡献文档与验收入口同步。
- [x] 原始日志、JSON/YAML 隔离、取消、后代进程清理及开发终端测试。
- [x] Dashboard 只读展示任务来源、依赖、输出和缓存配置；不推测缓存命中。
- [x] 中英文任务说明、文档站构建、Windows/macOS 交叉编译。
- [ ] Windows/macOS 原生测试与 GitHub Actions 实际跨运行缓存恢复：等待对应 runner。

本地验证命令与范围：

~~~sh
mise run check
mise run test:go
mise run docs:build
ONE_TEST_MISE_BINARY=/path/to/mise-2026.9.7 go test ./packages/cli/tests/e2e -run 'TestE2E_(Tasks|NativeMise|DevPrepares|DevSynchronizes)'
GOOS=windows GOARCH=amd64 go build ./packages/cli/cmd/one
GOOS=darwin GOARCH=arm64 go build ./packages/cli/cmd/one
~~~

完整门禁包含架构检查、字典键及格式参数、文档引用、Go 测试和 84 个 Dashboard 测试；race 测试独立执行。真实 mise 集成固定为 2026.9.7，使用临时 HOME/config/cache 与离线包工具 fixture；缓存断言检查实际执行次数和恢复文件内容。真实 Go + Task 集成还验证了可执行产物恢复与含空格的 CLI_ARGS。开发首次启动测试验证先准备依赖、再构建上游，且整次只准备一次。

## 17. 实施边界与后续验证

1. 列表、Dashboard 和 dry-run 是静态配置视图，不运行动态表达式、远程任务 includes 或依赖参数。不能确定静态执行范围时给出错误；实际有限任务通过 mise 有效任务清单及单任务信息执行。
2. 有限任务使用 stream/raw。开发保留 TUI；逐任务成功与命中状态为 unknown，整体状态和退出码有效。
3. 自动缓存只覆盖内置且构建配置未修改的 build。其他任务需显式声明缓存规则；Electron、Expo 构建不自动推断产物。传入额外任务参数时关闭缓存并强制执行。
4. Linux 上已验证跨 checkout 的本地缓存恢复。仓库工作流已接入 Actions 缓存，实际远端 job 尚未运行；交叉编译也不能代替 Windows/macOS 原生验证。
5. 远程模式透传 mise，但没有提供远程缓存服务地址或凭据，本次不部署或宣称验证远程缓存后端。
6. Vercel 项目内仍使用 pnpm 的 install/build 命令，符合项目内工具分工；未修改线上项目设置。

## 18. 参考依据

已读取官方文档及 2026.9.7 标签中的对应文档；经过真实二进制验证的范围见第 16、17 节。

- [mise 任务概览](https://mise.jdx.dev/tasks/)
- [任务配置](https://mise.jdx.dev/tasks/task-configuration.html)
- [Monorepo 任务](https://mise.jdx.dev/tasks/monorepo.html)
- [缓存与远程服务配置](https://mise.jdx.dev/tasks/caching.html)
- [mise run 参数](https://mise.jdx.dev/cli/run.html)
- [CI 接入](https://mise.jdx.dev/continuous-integration.html)
- [2026.9.7 缓存文档](https://github.com/jdx/mise/blob/v2026.9.7/docs/tasks/caching.md)
- [2026.9.7 任务配置](https://github.com/jdx/mise/blob/v2026.9.7/docs/tasks/task-configuration.md)

现有设计背景：

- [mise 接入](2026-09-14-mise-runtime-adoption.md)
- [mise 获取与托管](2026-09-26-mise-runtime-bootstrap.md)
- [终端与多项目执行](2026-09-28-dev-build-terminal-ui.md)

旧设计文件保留历史背景；其中任意命令 run、有限 build 调度及缓存行为以本方案和当前使用文档为准。
