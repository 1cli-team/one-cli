# 多语言工作区与依赖初始化

日期：2026-09-14。状态：已实现并通过自动检查与 Linux 真实运行验收；等待用户测试后再提交。

延续 [mise 接入方案](./2026-09-14-mise-runtime-adoption.md)：保留现有 `create/add/dev/run` 用法，继续内置 mise，不增加额外二进制压缩。用户测试确认后再提交；实现阶段不按每个小步骤提交 commit。

## 目标和当前证据

目标是让同一个 One workspace 自然容纳纯 Node、纯 Go，以及 Node + Go 项目。首个 Go 模块加入时自动初始化 Go monorepo，首个 JS/TS 项目加入时自动初始化 Node monorepo；后续项目增量加入对应语言的工作区。用户添加项目后，正常执行 `one dev` 即可完成必要的环境准备；两套语言配置可以共存。

修复前的报错发生在工具安装完成之后，Go 编译器报告缺少依赖的 `go.sum` 记录。当时的代码中有三个直接相关的事实：

- `internal/core/template/render.go` 将原始 `go.mod`、`go.sum` 视为模板开发隔离文件并排除。项目的 `go.mod` 来自 `go.mod.hbs`，但没有对应的项目 `go.sum.hbs`。
- `internal/transport/cobra/dev/cmd.go` 的 `ensureDependencies` 只处理 Node 项目。
- `internal/core/workspace/summary.go` 的 `ProjectDependenciesInstalled` 对非 Node 项目直接返回 true。

另外，修复前的 `creation/workspace_files.go` 无条件生成根 package.json、pnpm workspace、Husky、Changesets、commitlint。这个默认值使纯 Go workspace 也带上 Node 工具，但不是此次 Go 校验错误的直接原因。

## 配置的职责

| 内容 | 管理者 | 生成时机 |
| --- | --- | --- |
| 项目身份、路径、toolchain | one.manifest.json | create / add |
| 工具版本和运行环境 | mise 配置 | 按实际项目能力刷新 |
| Node 项目与包依赖 | 根 package.json、pnpm-workspace.yaml；安装时维护 pnpm-lock.yaml | 首个 JS/TS 项目加入时初始化，后续增量维护 |
| Go 模块与第三方依赖 | 各项目自己的 go.mod、go.sum | 模板生成及模块依赖准备 |
| Go 模块的工作区成员关系 | 根 go.work；Go 按需维护 go.work.sum | 首个 Go 模块加入时初始化并登记，后续增量维护 |

pnpm workspace 负责 Node 包的工作区关系。[pnpm 文档](https://pnpm.io/workspaces)。Go 的 `go.work` 可以从一个模块开始，后续通过 `use` 加入其他本地模块。[Go 工作区教程](https://go.dev/doc/tutorial/workspaces)。`go mod tidy` 仍以单个模块为单位，不能用根目录的 workspace 操作代替每个模块的依赖维护。[Go 模块文档](https://go.dev/ref/mod#workspaces)。

初始化只生成有意义的配置，不创建空的 go.work.sum 或伪造 pnpm-lock.yaml；这些文件由对应工具按需生成，或由模板提供经过校验的锁文件。

## 命令与生命周期

| 操作 | 目标行为 |
| --- | --- |
| `one create demo` | 生成语言无关的基础目录、Manifest、Git 和根 mise 配置 |
| `one add go-api --name api` | 生成完整 Go 模块；首次加入时初始化根 go.work，并将该模块加入 use；登记项目，配置 Go / Task |
| `one add react-spa --name web` | 首次加入 JS/TS 项目时初始化根 Node workspace；生成并纳入项目，登记并刷新 mise |
| `one create --preset ...` | 全部项目生成后统一刷新工作区能力，避免重复写根文件 |
| `one dev api` | 准备所选 API 所需工具和依赖，再启动服务 |
| `one dev` | 准备选中的开发项目，Node 根安装去重，准备全部成功后再启动 supervisor |
| `one run api -- <cmd>` | 保持通用命令语义，不在用户命令之前偷偷运行 tidy 或安装脚本 |
| `--dry-run` | 只展示可静态计算的项目、工具和准备计划，不执行工具、网络或写文件 |

`create/add` 保持生成配置的操作，不把联网安装塞进模板渲染事务。默认开发路径由 `dev` 完成准备；需要手工维护时仍可使用现有 `one run api -- go mod tidy` 或 `one mise exec -- pnpm install`。

## 第一阶段：修复 Go 首次启动

先完成模板和依赖准备的正确性修复。第二阶段接入首个模块触发的语言工作区初始化；新工作区流程的完整交付必须包含这一行为。

### 模板交付完整性

1. 将模板开发用模块文件与生成项目用模块文件明确分开。保留原始 go.mod / go.sum 的排除规则，新增明确交付给项目的 go.sum.hbs，或等价的显式资源声明。
2. 以完整渲染后的项目维护和校验 go.mod.hbs / go.sum.hbs，不能在含有未渲染 `.go.hbs` 的模板目录中直接 tidy 后当成发布结果。
3. 渲染不同项目名，验证内部 module import 替换正确，依赖校验记录保持一致。仅标准库的 Go 模板允许不存在 go.sum。
4. 将渲染产物的只读依赖解析和编译加入模板验收，避免“模板生成成功”掩盖“首次运行失败”。

### 统一依赖准备

提取独立的依赖准备服务，内部区分 Node 和 Go；Cobra 只负责参数、进度和结果展示。

- Node：继续在工作区根目录安装一次，沿用现有包管理器选择规则，不逐个项目执行安装。
- Go：在目标模块目录，通过相同 mise runtime 使用兼容 Go 版本。独立模块使用临时 `-modfile` 下载固定构建列表，确认模块声明未改变后仅发布 go.sum；工作区模式由 `go list -mod=readonly -buildvcs=false -deps ./...` 按实际包依赖准备本地成员和外部依赖。不能在工作区中无条件下载 `all`，否则可能查找本地成员未发布的历史版本。
- 自动准备允许补充 go.sum；不得隐式升级已有依赖版本、替换 go.mod 或忽略校验失败。具体 Go 命令组合先通过缺失 sum、只含 go.mod 校验、缺少 indirect requirement 等 fixtures 验证。
- 不把 `go mod tidy` 放在每次 dev 前无条件执行。若需要改变模块声明，返回项目名、模块目录和明确的 tidy 修复命令；现有问题可先用 `one run api -- go mod tidy` 修复。
- 不在 `run` 中设置安装钩子，避免执行 tidy 本身时又触发依赖准备，以及影响用户自定义命令。
- 沿用语言工具自己的缓存。One 只保存必要的准备状态；状态不能只凭 go.sum 存在、node_modules 目录存在或旧的“成功”标记判定。输入变化、缓存被清理和工具版本变化都需要重新验证。

`go mod tidy` 可能增删模块声明、增删校验记录，因此它应作为明确的维护动作；自动下载和 tidy 必须分开。[Go tidy 说明](https://go.dev/ref/mod#go-mod-tidy)。

### 失败和状态

- 准备失败时不启动任何选中的服务，保留已生成项目和成功下载的缓存，允许原命令重试。
- 网络错误、模块声明不完整、校验不匹配、工具缺失分别报告，保留底层错误详情；不把网络错误统一当成“执行 tidy 即可”。
- 日常 `dev` 自动完成普通缺失依赖的准备，不再让用户逐个切目录安装。非交互执行同样给出确定的结果和退出码，日志进入 stderr。
- 对已有非交互脚本增加说明：首次 dev 可能联网下载；若仓库锁文件不一致需要更新，明确失败，不静默更新锁文件。先确保仓库已有锁文件完整。
- Go 依赖状态区分“未验证”和“确定缺失”，避免工作区摘要继续对全部 Go 项目报告已就绪。现有输出字段保持兼容，新增状态字段采用可选方式。
- 准备按根 Node workspace / Go 模块加锁，避免两个 `one dev` 同时写同一份依赖元数据。命令取消必须停止准备进程，释放锁，不进入 supervisor。
- 传递 provider 返回的完整 argv、cwd 和 env；当前 Node 安装路径只取 prepared.Argv，这一点随提取服务一并修正。

## 第二阶段：首个项目触发语言 monorepo 初始化

### 新工作区

基础 create 仅生成公共文件。由 Manifest 中的项目 toolchain 和实际模块配置识别语言能力，首次添加对应语言项目时，自动完成该语言 monorepo 的初始化。一个 Go 模块也创建 go.work，用户无需额外执行 go work init。

| 当前状态 | 添加项目 | 根目录和成员关系变化 |
| --- | --- | --- |
| 空工作区 | 首个 Go 模块 | 创建 go.work，use 包含新模块路径；配置 Go / Task |
| 空工作区 | 首个 JS/TS 项目 | 默认创建 pnpm 所需的根 package.json、pnpm-workspace.yaml 和 Node 开发工具配置；确保新项目被 workspace 覆盖 |
| 已有 Go | 首个 JS/TS 项目 | 增量初始化 Node workspace，保留 go.work 和 Go 配置 |
| 已有 JS/TS | 首个 Go 模块 | 增量初始化 go.work 并登记新模块，保留 Node workspace 配置 |
| 已有对应语言 | 同语言后续项目 | Go 增量维护 use；JS/TS 确认现有 workspace 范围覆盖新项目，必要时增量补齐；保留原有成员和配置 |

初始化、项目登记和 mise 配置刷新使用同一份工作区变更计划：先检查冲突再写入，避免新增项目后缺少对应根配置。重复执行协调操作不产生重复 use 或 workspace 条目；preset 批量创建统一协调，最终结果与逐个 add 一致。对于旧工作区，若根 go.work 缺失，添加 Go 模块时一并登记 Manifest 中已有的 Go 模块。

go.work 的 go 版本须满足所有已登记模块；mise 为这些 Go 项目提供兼容工具版本。该规则从首个模块开始生效，Node 项目不因此被强制安装 Go。

这会改变新工作区的初始文件集合，但不改变用户命令语法。需要同步更新 create 的提示、JSON 中 package_manager 的含义、文档和快照：尚无 Node 能力时不能继续返回“pnpm 已配置”。Manifest v1 的项目 toolchain 已足够作为依据，首轮不新增全局“工作区语言”字段。

工作区 Git hooks 统一由 hk 提供，不生成 Husky 或 commitlint。纯 Go、JS 和混合工作区均不默认生成 Changesets 配置或依赖；版本管理和发布流程由项目按需配置。

### 旧工作区与用户配置

- 不删除现有 package.json、pnpm 配置、Git hooks 或用户 mise.toml，不因升级自动清理 Node 文件。
- 已有根 package.json 表示用户可能使用 Node 工具，即使 Manifest 暂无 Node 项目也要保留其工具需求。
- `add` 新增语言能力时先检查冲突。文件不存在才生成；用户文件仅合并必要且无冲突的字段，遇到不兼容包管理器配置给出具体差异。已有 Node 工作区沿用其包管理器，不强制改为 pnpm。
- 已有用户 go.work 保留 use、replace、toolchain、注释及仓库外路径。仅增量加入缺失的模块路径；如解析失败、版本约束冲突或无法安全保留现有内容，在写入前报告具体冲突，不覆盖整个文件。
- 不通过运行子进程、写 .env 或加载项目密钥来判断工作区类型。
- 现有 `one configure mise` 继续负责刷新 mise 配置，不能悄悄变成清理整个工作区的命令。

## 第三阶段：Go 多模块协作与一致性验收

根 go.work 已在首个 Go 模块加入时建立，后续 Go 模块通过同一协调逻辑增量加入 use。本阶段完善和验证模块之间的本地引用、依赖准备和冲突诊断，不再另设初始化门槛。

- 从第二阶段初始化开始，One 新生成的 go.work 带初始化说明，使用 Go 官方 modfile 解析器增量维护 use 路径；写入前校验读取快照，并使用文件锁和原子替换。go.work 可以由用户编辑，已有注释和指令按增量合并规则保留。
- 验证已有用户 go.work 的增量合并，确保 use、replace、toolchain、注释和仓库外路径均保留。
- 验证工作区 Go 版本满足所有成员模块及 go.work 的要求，避免根版本高、项目 mise 配置覆盖成低版本。
- go.work 会影响整个 Go 构建图及依赖选取。工作区模式下以该构建图准备依赖，不能把相关本地模块当成远程依赖下载，也不能承诺只下载单个模块的依赖。
- 不在普通 add/dev 中自动执行 go work sync，因为它可能修改成员 go.mod。每个模块继续维护自己的 go.sum，go.work.sum 不替代它。
- 新生成且仅含仓库内成员的 go.work 纳入项目版本控制；包含个人外部路径的用户文件保持用户策略。Go 库发布前额外验证 GOWORK=off 能独立构建；纯本地、未发布共享模块需要单独声明发布边界。
- 模块删除、移动、重复 module path、外部 GOWORK 覆盖和多个 Go 版本要求都纳入冲突诊断；不扫描整块磁盘自动登记模块。

## 代码落点和验收

| 阶段 | 主要代码落点 | 核心验收 |
| --- | --- | --- |
| 1 | core/template/render、go-api 模板资源、modules/dependencies（新增）、dev/cmd、core/workspace/summary、bootstrap | 从生成到 API 首次启动，覆盖缺失 sum 和空依赖缓存；重复启动复用缓存 |
| 2 | creation/workspace_files、workspace_content、service、project、modules/gowork（新增）、miseconfig、create/add 输出 | 首个 Go 即生成 go.work 并含该模块；首个 JS/TS 即生成 Node workspace；纯 Go 无强制 Node；两种添加顺序和 preset 得到一致能力；后续登记幂等；旧项目和用户文件保留 |
| 3 | modules/gowork、creation/project、miseconfig、依赖准备服务 | 第二个及后续 Go 模块增量登记；API 引用本地 Go 库；修改库后 API 使用本地代码；用户 go.work 安全合并；Go 版本满足整个构建图 |

模块服务通过 ports/runtime 调用工具；解析和计划逻辑与网络/进程执行分离，遵守现有架构依赖规则。直接调用现有 Go、pnpm 和 mise 能力，不重新实现模块解析器或依赖求解器。

每个阶段需要验证 dry-run 无副作用、Windows 路径含空格、明确的失败退出码、并发准备、取消和重试。真实模板测试用临时项目及隔离缓存，先验证依赖解析/编译，再验证服务启动；不将数据库连接失败混同为依赖失败。网络受限的常规测试使用本地模块代理 fixture；发布前补一次真实工具与真实模板验收。

实施后执行 task check，对锁和进程生命周期执行相关 race 检查。macOS/Windows 的真实运行验收独立记录，交叉编译不能替代。

建议按 1 → 2 → 3 实施，每阶段提供可测试结果。第一阶段解决当前依赖报错；第二阶段交付“第一个 Go / JS 项目即自动初始化对应 monorepo”的默认行为；第三阶段补齐多个 Go 模块协作的验收。阶段划分是实现顺序，最终用户流程以首次添加就完成初始化为准。

## 实施记录

- `creation/languages.go` 负责首次语言配置初始化；Manifest、语言配置和 mise 配置共享文件变更计划，冲突检查完成后发布。新 Node 项目使用实际项目包名和根包管理器，避免重复使用模板包名。
- 新增 `modules/gowork` 和 `modules/dependencies`。Go 摘要增加可选的 `dependencies_status: unverified`，避免静态检查误报就绪。
- Node 缓存指纹包含项目声明、根配置、锁文件和实际工具版本。pnpm 12 的锁文件需要解析全部 YAML 文档；仅有 `packageManagerDependencies` / `configDependencies` 的环境文档以及其后的空文档，不视为应用依赖已锁定。已有应用锁文件继续冻结安装。参见 [pnpm 上游说明](https://github.com/pnpm/pnpm/issues/13805)。
- `go.sum.hbs` 来自完整渲染的 API 项目，经 Go 1.27.0 tidy 与只读依赖解析校验；go.mod 除项目名以外与模板一致。真实渲染模板编译通过。
- 验证日志：`/tmp/one-polyglot-check.log`、`/tmp/one-polyglot-race.log`、`/tmp/one-polyglot-mise.log`；真实项目在 `/tmp/one-polyglot-acceptance/demo`。
- Linux 上内置 mise 启动 Go API，`/health` 返回 200；停止后端口释放。Windows amd64 和 macOS arm64 已交叉编译，尚未在对应操作系统实际运行。
- JS 首次 `one dev web` 自动生成应用锁文件、完成依赖安装并返回 HTTP 200；第二次启动复用缓存，正常停止。生成的 Go 库通过 `GOWORK=off go test` 独立验收。
