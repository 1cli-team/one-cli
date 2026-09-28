# 可运行项目模板迁移规划

日期：2026-09-29。状态：已实施；文末记录实现差异、验证结果和边界。

推荐方向：模板源码保存为正常、可独立开发的示例项目；生成时复制文件，再执行少量有明确边界的修改。One CLI 继续用 Go 完成离线生成，复用现有工作区配置、格式保留和文件写入机制。

## 1. 调研结论与方案选择

项目脚手架已有成熟实践，但没有统一的“最佳模板引擎”。需要分别考虑文件获取、项目参数化、功能组合、生成后的持续更新。

| 参考项目 | 已核实的做法 | 对 One CLI 的意义 |
| --- | --- | --- |
| [create-vite](https://github.com/vitejs/vite/blob/main/packages/create-vite/src/index.ts) | 复制普通模板目录，修改 package.json 的 name、HTML 标题，按选项执行配置修改 | 适合现有内置模板，作为总体参考 |
| [create-vue 文件复制](https://github.com/vuejs/create-vue/blob/main/utils/renderTemplate.ts)、[生成入口](https://github.com/vuejs/create-vue/blob/main/index.ts) | 组合目录、合并 package.json，部分配置仍使用 EJS | 证明“组合文件 + 少量渲染”也是合理设计；未来有真实功能组合需求再引入目录叠加 |
| [gonew 源码](https://github.com/golang/tools/blob/master/cmd/gonew/main.go)、[官方介绍](https://go.dev/blog/gonew) | 普通 Go module 作为模板；复制时解析并修改 module 和 import | Go 模板直接参考其修改边界；工具仍标注实验性，不把它当作稳定依赖或运行时前置条件 |
| [Giget](https://github.com/unjs/giget) | 下载 Git 仓库或归档，提供来源、子目录和缓存等能力 | 解决模板获取；当前 One CLI 的内置模板迁移不需要引入下载工具 |
| [Yeoman](https://yeoman.io/authoring/file-system) | 支持普通文件复制、EJS copyTpl、内存文件系统和冲突处理 | 可参考先准备内容再写入的组织方式；整套 Node generator 运行时超出当前需要 |
| [Copier 模板](https://copier.readthedocs.io/en/stable/creating/)、[项目更新](https://copier.readthedocs.io/en/stable/updating/) | Jinja 参数化；保留生成答案并结合旧、新模板重建及差异应用来更新项目 | 如果产品目标变为持续升级已有项目，应单独评估 Copier 路线；删除 HBS 本身不会提供升级能力 |

推荐采用 create-vite 的总体方式和 gonew 的 Go 文件修改方式，用现有标准库、golang.org/x/mod 和仓库工具实现少量适配代码。以上工具覆盖的问题与 One CLI 不完全相同，没有理由为了删除 HBS 再引入 Node/Python 生成运行时。

继续使用 HBS 并非技术错误，但当前内置模板没有使用条件、循环或 helper，投入通用模板引擎带来的维护成本已经大于收益。换成 EJS、Jinja 或 Go text/template，仍然需要维护不能直接编译的模板源码。

## 2. 仓库现状与真实迁移规模

调研时工作区没有未提交变更。注册表包含 13 个模板：8 个 Node、2 个 Go、3 个 empty 模板。

| 模板 | HBS 文件数 | 实际变量 | 迁移重点 |
| --- | ---: | --- | --- |
| go-api | 17 | projectNameKebabCase | go.mod、import、README；go.sum.hbs 没有变量 |
| go-lib | 4 | projectNameKebabCase | go.mod、注释、README；greeter_test.go.hbs 没有变量 |
| electron-app | 10 | projectNameKebabCase、projectName | 根包及 3 个成员包、依赖键、scripts、源码引用、README |
| 其他 10 个模板 | 0 | 无 HBS 插值 | 行为回归验证 |

合计 31 个 HBS 文件，只用了两个变量，没有 HBS 条件、循环。没有 template.json 文件，也没有实际使用文件名变量；Expo 的 __tests__、__snapshots__ 是普通目录名，必须保持原样。

当前处理链：

```text
packages/templates 源码
  → tools/sync-resources 同步到 bundled/_templates
  → go:embed
  → template.Render 写入项目目录
  → creation 配置 package.json、工作区成员和 manifest
  → FilePlan.Apply
  → syncProject 生成开发相关文件
```

需要以实现为准的边界：

- `internal/core/template/render.go` 仅对 .hbs 调用 Raymond，其余文件原样复制；go.mod/go.sum 在渲染阶段被排除。
- `tools/sync-resources/main.go` 在打包时排除 go.mod。不能简单删除这个条件，详见第 5 节。
- `internal/modules/creation/project.go` 只接受 `local:` 模板。注册表可以来自网络，不代表模板内容已支持远程下载。
- `render.go` 中关于 template.json 变量扩展的注释没有对应的实际加载逻辑；新描述文件应明确作为新能力实现。
- `configureNodePackage` 已经负责根 package.json 的项目名和 packageManager；`json_fields.go` 保留未修改字段的格式和顺序，应继续复用。
- 模板目前直接写入目标目录。创建失败时，仅对本次新建的目录执行清理；根配置通过 FilePlan 管理；syncProject 在提交后运行。当前流程不是整个创建操作的完整事务。
- `NodeProjectPackageDirs` 虽然支持读取 FilePlan overlay，成员发现仍使用磁盘 Glob，不能只把文件放入内存就假设所有发现逻辑可用。

## 3. 本次范围与完成后的行为

本次覆盖内置模板源码、嵌入打包、参数化、生成回归和模板维护文档。

完成后应满足：

1. 开发者直接编辑 .go、.ts、.tsx、package.json、go.mod，编辑器及语言工具链能正常工作。
2. 每个 Go 模板只有一份 go.mod；go-api 只有一份 go.sum，go-lib 不人为生成空 go.sum。
3. 创建命令、模板 ID/code、preset 编码、项目目录布局和已有命名规则保持兼容。
4. `My_API` 等合法输入仍按现有规则生成项目路径、显示名称及 kebab-case 包名；不在迁移中重新定义名称规范。
5. Go 生成结果仍默认使用 `github.com/example/<projectNameKebabCase>`；Electron 仍使用 `@<projectNameKebabCase>/`，允许多个桌面项目共存。
6. 新增模板相关错误、校验提示、帮助及公开文档同步提供 zh-CN/en-US，保持具体路径、原因和错误链。
7. 创建操作不增加 Node、Python、gonew 或外部格式化器的前置依赖；依赖安装和真实构建仍按现有流程执行。

远程模板下载、任意自定义问题、脚本 hook、功能选择矩阵、已有项目升级、项目重命名和新的 module-path CLI 参数作为后续独立需求。第一版不承诺这些能力。

## 4. 目标架构与职责

```mermaid
flowchart LR
  A[正常的示例项目] --> B[资源打包与文件名映射]
  B --> C[内置文件系统]
  C --> D[复制并校验模板描述]
  D --> E[Go / Node / 文本定向修改]
  E --> F[现有 creation 工作区集成]
  F --> G[生成后的真实项目检查]
```

默认复制文件，只有声明了参数化需求的模板才添加 `template.json`。普通 Node 模板继续使用现有 `configureNodePackage`。empty 模板继续保持轻量。

第一版描述文件仅承载内置模板确实需要的数据，使用带版本的 JSON 和严格 Go 类型。它不是脚本语言，不提供表达式、条件、循环或任意命令。版本固定为 schemaVersion=1，未知版本和未知字段报错；文件不复制到用户项目中。

拟议的 Go 模板描述如下，属于待实现接口，不是现有格式：

```json
{
  "schemaVersion": 1,
  "go": {
    "modulePrefix": "github.com/example/"
  },
  "text": [
    {
      "files": ["README.md"],
      "from": "one-template-go-api",
      "value": "projectNameKebabCase",
      "minMatches": 1
    }
  ]
}
```

源码 go.mod 使用 `module github.com/example/one-template-go-api`，import 使用同一前缀。旧 module 从 go.mod 读取，目标值由 modulePrefix 与现有规范化名称生成。README 的示例名称单独替换。

第一版只提供以下配置项：

| 字段 | 语义 |
| --- | --- |
| schemaVersion | 描述文件协议版本 |
| go.modulePrefix | 生成 Go module 的固定前缀，旧 module 从源码读取 |
| node.scope | Electron 示例内部包的完整 scope，如 @one-template-electron |
| node.sourceFiles | 需要替换该 scope 引用的精确源码路径列表 |
| text | 指定文件中的字面量替换；value 只允许 projectName / projectNameKebabCase |
| exclude | 不交付给生成项目的开发文件或目录，精确相对路径，无 glob |

`node.scope` 对应的新 scope 固定为规范化项目名。源 package.json 的 workspaces 继续作为成员关系的唯一依据，不在描述文件重复维护成员清单。

文本规则按原始内容定位后一次性应用，检测区间重叠，避免一个规则改出的内容再次被另一个规则替换。每条规则每个指定文件需满足 minMatches，默认 1；源字符串为空、文件不存在、命中不足、规则重叠均报错。只处理指定的 UTF-8 文本，不做全目录字符串替换。新增文案仍需正确处理所在文档上下文。

不新增通用 JSONPath 或任意 JSON patch 配置：Node 包名、依赖和脚本处理由受限适配器负责。先复用已有 JSON 编辑器，必要时下沉为内部共享包，避免 template 反向依赖 creation。

## 5. Go 模板：源码、嵌入与参数化

### 5.1 恢复普通 Go 模块

- 将 21 个 Go HBS 文件恢复成正常文件名，源码使用合法且唯一的示例 module。
- 合并 go.mod 与 go.mod.hbs；go.sum 以当前真正交付的 go.sum.hbs 为迁移基准，在完整源码上校验后保留一份。
- 在模板源码目录以隔离的 Go workspace 配置运行检查，例如进程环境 GOWORK=off；不修改开发者全局 go env。
- 保留既有 Go 版本和依赖版本。此次迁移不顺带升级依赖。
- go-lib 的库注释以精确文本规则处理；greeter 包名保持现有语义，不自动随项目名改名。

### 5.2 必须处理 go:embed 的模块边界

Go 官方明确规定：嵌入模式不能匹配包含 go.mod 的独立模块目录。`all:` 允许隐藏文件，但不会取消模块边界。因此普通 go.mod 不能原名放入 bundled/_templates。[Go embed 文档](https://pkg.go.dev/embed)

推荐在打包层做一个明确映射：

```text
模板源码              内部打包目录          用户项目
go.mod        →       _go.mod       →      go.mod
go.sum        →       go.sum        →      go.sum
main.go       →       main.go       →      main.go
```

`_go.mod` 只存在于自动生成的打包目录。源码仍然是正常 Go 项目。同步工具预先拒绝源码中与保留文件名冲突的文件；渲染器只还原这一个明确映射，不把所有下划线文件都改成隐藏文件。

同步资源测试及 bundled 一致性测试需要通过映射比较逻辑路径与内容，既验证包含完整 Go 资源，也验证资源树内没有裸 go.mod。必须用真正的 CLI 编译和嵌入文件读取验收，单测临时目录复制不足以证明可用。

### 5.3 Go 修改边界

- 使用已有 `golang.org/x/mod/modfile` 读取并修改 module 声明，保留依赖集合与版本。
- 使用 `go/parser` 的 import 解析能力定位导入字符串，仅修改等于旧 module 或以 `旧 module + /` 开头的 import。
- 使用源码位置修改字节范围，保留别名、注释、build tags 和无关代码；避免遍历 AST 后重印整个文件产生无关 diff。
- 解析覆盖全部 .go 文件，包括当前平台不参与编译的文件；嵌套模块第一版不支持，遇到时明确报错。
- 不修改第三方路径、普通字符串、注释中的近似前缀。README 和说明性注释按 text 规则修改。
- 复用模块路径校验。创建时不执行 go mod tidy；源码及生成项目的依赖完整性由 CI 验证。

该边界参考 [gonew 实现](https://github.com/golang/tools/blob/master/cmd/gonew/main.go)，使用的是已存在的 [modfile](https://pkg.go.dev/golang.org/x/mod/modfile) 和 [go/parser](https://pkg.go.dev/go/parser) API。

## 6. Electron 与普通 Node 模板

Electron 源码使用固定合法的内部包名，例如 `@one-template-electron/preload`、`@one-template-electron/electron`、`@one-template-electron/ui`；根包名使用正常示例名称。全部 .hbs 恢复正常扩展名。

生成步骤：

1. 读取根 package.json 及声明的成员包，建立“源包名 → 目标包名”映射，校验重复包名和目标冲突。
2. 修改各成员的 name 以及 dependencies/devDependencies/peerDependencies/optionalDependencies 中对应的内部包键，保持 workspace:* 等依赖值。
3. 在 scripts 字符串中，替换精确内部包引用，覆盖 pnpm -F 等现有用法；保留未知 shell 片段及 JSON 格式。匹配完整包名边界，避免修改名称相似的外部包。
4. 在 node.sourceFiles 中替换明确的旧 scope 前缀。当前文件都是 import/export、类型引用或 require.resolve 中的字符串及相关注释，第一版不引入 JS/TS AST 运行时。
5. README 的标题和命令使用 projectName，包 scope 使用 projectNameKebabCase，保持当前两种名称的区别。
6. 继续让现有 creation 流程配置根包 name、包管理器版本、根工作区成员、锁文件策略及安装脚本策略。

JSON 修改必须覆盖依赖对象的键；只改字符串值会遗漏最重要的 workspace 依赖。现有 JSON 编辑器仅支持直接字段，扩展嵌套对象及键重命名时必须保留原有格式回归测试，禁止回退为 map 整体序列化。

模板维护需要一份仅用于开发的 pnpm-workspace.yaml，列出这三个成员，使 Electron 源码可以独立安装与运行。把它加入此模板的 exclude；生成到 One 工作区时仍由根工作区统一管理，避免产生嵌套安装边界。开发版本与仓库固定工具链需核对，不能顺带改变用户工作区的包管理器版本。

其他单包 Node 模板没有 HBS，无需添加描述文件。直接维护时需要隔离父工作区设置；实施时以仓库固定 pnpm 版本的帮助为准确认 `--ignore-workspace` 使用方式。该选项用于独立安装的语义可参考 [pnpm 官方说明](https://github.com/pnpm/pnpm.io/blob/main/docs/cli/install.md)。CI 也应在仓库外临时目录验证源码，避免误用仓库根依赖。

## 7. 复制规则、失败处理与兼容性

### 7.1 文件交付规则

- 默认按原始字节复制，图片、字体、图标等二进制不进入文本处理器。
- 打包和运行时保持一致的基础排除规则，继续排除 .git、node_modules、已有 agent 指令与 Node 锁文件；补齐开发产生的构建产物、缓存排除，避免模板变得可运行后把 node_modules/dist 带入 CLI。
- 排除规则需按模板路径和真实产物目录确定，不用宽泛后缀规则删除业务资源。
- 描述文件保留在嵌入资源中供生成器读取，不写到用户项目。
- 保持 .editorconfig、.env.example 等文件交付行为；__tests__、__snapshots__ 无条件按原名处理。
- 生成文件权限至少保持当前 .sh 行为；额外可执行文件的需求出现后再扩展描述，不把嵌入 FS 的权限当作原始权限。
- 模板输出路径必须留在目标目录；第一版内置模板禁止符号链接，防止打包阶段读出模板边界。

### 7.2 写入流程

本次新增的描述读取、文件复制及参数化先在内存文件集合中完成。所有规则验证通过后，才向目标目录写入；这使语法错误、缺失文件、命中不足等模板问题不会留下半个项目。

第一版保持后续 creation 的磁盘发现流程，复用现有 WorkspaceLock、SafeWritePath 和 FilePlan。模板文件发布也用 FilePlan，以便写入失败时恢复本次修改，并记录实际创建的空目录用于清理。

不在本次把整个 creation 改造成统一事务：需明确区分“模板准备失败”“模板文件写入失败”“后续工作区集成失败”和“提交后的 syncProject 失败”。既有新建目录清理与根配置回滚行为不能退化。已有空目录情况下的跨阶段全回滚，以及 syncProject 的整体事务化，作为单独后续改进，不能在发布说明中宣称本次已解决。

### 7.3 HBS 退出方式

本次迁移只影响后续生成，已有用户项目不需要执行迁移。内置资源随 CLI 一起发布，当前代码也不加载远程 HBS 模板，因此不建议长期保留双引擎。

实施中可以按描述文件选择新流程、无描述时暂走旧流程，以支持分批验证。全部 31 个文件完成迁移并通过回归后，在同一交付周期移除 Raymond、HBS 分支及未使用的路径插值逻辑。

删除路径插值前以调用点、测试和公开文档完成核对。若发现实际外部兼容承诺，按明确版本保留旧格式支持并记录删除条件；不要仅凭过时注释构建永久兼容层。错误码、registry JSON 字段、模板 ID/code 和 preset 编码保持稳定。

## 8. 分阶段实施与审查切分

| 阶段 | 改动及产物 | 通过条件 |
| --- | --- | --- |
| P0：固定基线 | 用当前生成器为 13 个模板记录路径清单、关键配置及命名结果；补足有意义的生成行为测试 | 能区分参数化变化、格式变化和不应出现的业务代码变化 |
| P1：描述、复制和打包 | 实现受限描述解析、逻辑文件集合、Go 打包映射、复制排除及错误信息；用 go-lib 先走通 | 普通源码能测试；实际编译后的 CLI 能生成正确 go.mod；二进制资源原样交付 |
| P2：Go 模板迁移 | 迁移 go-lib/go-api 共 21 个文件，合并 module/checksum 文件，加入 module/import 定向修改 | 源码及生成结果均可独立测试、构建；校验和与版本正确，第三方 import 未变 |
| P3：Electron 迁移 | 迁移 10 个文件、内部包映射和开发用工作区配置，保留 JSON 格式 | 源码可独立构建；同一 One 工作区两个 Electron 项目能共存并正确解析 preload |
| P4：收尾与发布门禁 | 删除 HBS/Raymond、过时注释及逻辑；同步资源测试、双语文档、CI 检查 | 13 个模板回归通过；模板源码无 HBS；mise run check 通过；最终产物完成真实 CLI 冒烟 |

这些阶段是建议的审查边界，不要求为每阶段引入新的公共命令。每阶段保留可验证结果，Go 打包变更与对应渲染还原必须同批交付。描述与渲染器协议在 P1 固定，后续只添加模板数据。

主要涉及的代码位置：

- `packages/cli/internal/core/template/`：描述、复制、Go/Node 参数化、变量收敛及测试。
- `packages/cli/internal/modules/creation/`：维持工作区集成边界，复用或下沉 JSON 编辑能力，扩充生成集成测试。
- `packages/cli/tools/sync-resources/` 和 `internal/resources/bundled/`：打包映射、排除规则、嵌入资源一致性。
- `packages/templates/go-api/`、`go-lib/`、`electron-app/`：正常项目源码与少量描述数据。
- `packages/cli/tools/verify-cli-references/`：移除 HBS README 扫描分支，保留普通 README 验证。
- CLI 的两份 locale 字典、相关中英文模板文档、`mise.toml`/Taskfile/CI 中必要的模板验证任务。
- `packages/cli/go.mod`、`go.sum`：全部迁移完成后清理 Raymond。

预计工作量为一名熟悉仓库的开发者约 4–6 个工作日，含回归和审查修改；这是规划估算。Electron 原生依赖、跨平台构建和网络下载耗时可能增加日历时间。

## 9. 验证矩阵与验收标准

### 9.1 快速且确定的检查

- 13 个模板都通过实际创建服务生成；覆盖普通名、连字符、下划线、大小写转换，以及两个输入归一化到相同包名的冲突。
- Go 精确 module/import 修改：同前缀第三方模块、别名、注释、构建标签、平台专用文件、go.sum 字节保留。
- Electron：4 份 package.json 的名称、依赖键、scripts、5 个源码文件的引用，以及 README；旧 scope 不残留，外部包不被改名。
- 普通文件不变化：用路径集合和二进制哈希证明；无 .hbs、模板描述或内部打包名泄漏。
- 描述错误、目标冲突、模板修改失败及写入失败的可预测行为；校验相关阶段没有不应发生的目标文件和根配置修改。
- JSON 保留缩进、字段顺序、紧凑数组、换行符及 shell 字符，继续覆盖重复键和无效 JSON。
- 两种语言下的错误与帮助；切换后无旧语言残留；测试使用临时 HOME/config。

### 9.2 源码和生成结果分别检查

| 对象 | 源码检查 | 生成结果检查 |
| --- | --- | --- |
| go-api/go-lib | 在源码目录隔离父 go.work，运行格式、测试和构建 | 临时 One 工作区和脱离父工作区的独立模块各检查一次；go.mod/go.sum 不被意外修改 |
| electron-app | 独立 pnpm 工作区安装、类型检查及各包构建 | 同一 One 根目录加入两个不同名称的桌面项目，检查工作区依赖、preload 路径和构建 |
| 其余 7 个 Node 模板 | 现有格式、类型与构建任务按各模板实际 scripts 执行 | 检查 package name、packageManager、根锁文件归属及生成格式 |
| 3 个 empty 模板 | 文件清单 | 目录与 manifest、工具链值和 preset 行为 |

Electron 的安装与构建属于依赖 CI；GUI 启动及各 OS 打包使用已有支持的平台环境，不把某一平台的构建通过当作所有平台打包通过。Go API 的外部服务相关启动测试需要明确配置，基础编译测试不能声称验证了数据库连接。

### 9.3 实际运行门禁

- `mise run sync-bundled` 验证生成资源。所有基于 embed 的测试运行前先同步，避免旧资源造成假通过。
- 运行 template、creation、sync-resources、bundled 相关测试和 preset/帮助快照检查。
- 在 CI 中显式提供现有 `ONE_TEST_OXFMT_BINARY` 格式检查入口；仅设置可选跳过的测试不足以作为门禁。
- 增加模板源码与生成结果的依赖检查任务，按项目固定工具链执行；对受影响模板及生成器改动触发，发布前跑全量。
- 执行仓库要求的 `mise run check`。该命令的现有内容不能替代上面的真实模板安装与构建矩阵。
- 通过编译出的 One CLI 各生成一个 Go 和 Electron 项目，验证打包到最终二进制的资源与测试所用源码一致。

最终完成标准：31 个 HBS 文件全部退出模板源码；3 个复杂模板可以以普通项目方式维护；13 个模板的现有用户行为保持兼容；源码和生成结果均有实际检查证据；没有给创建操作增加新的脚本语言运行时。

## 10. 后续演进的触发条件

当出现真实的多功能组合需求时，再评估 create-vue 式目录叠加，并先定义文件覆盖、依赖合并与冲突规则。当前不引入泛化的生成步骤 DSL。

当需要远程社区模板时，单独设计来源解析、固定版本、缓存和可信执行边界；资源获取和文件参数化保持独立。此时再评估采用成熟下载组件的收益。

当需要持续升级已有项目时，单独设计模板版本、生成参数记录和合并策略，并与 Copier 做完整取舍。仅记录 templateID 不足以安全复现旧生成结果。

当前最先值得落地的切片是 go-lib：用一个小型正常 Go module 验证“源码可开发 → 正确嵌入 → 正确改名 → 生成结果可测试”的完整链路，再推广到 Go API 和 Electron。


## 11. 实施记录（2026-09-29）

- 已将 31 个 HBS 文件迁移为普通源码；移除 Raymond、路径插值及未使用的时间变量。
- 三个描述文件使用第 4 节的受限配置；新增双语错误、未知字段与重复 JSON 键验证、文本命中与重叠验证。
- 资源打包映射 go.mod → _go.mod；同步排除开发依赖、构建产物、锁文件和本地 go.work；资源测试现在比较文件内容。
- Go 源码有开发专用 go.work；Electron 有开发专用 pnpm-workspace.yaml；二者不交付生成项目。
- Node JSON 编辑器下沉为共享包；修改包名与依赖键时保留格式，并对改名后的依赖对象排序，使不同项目名都符合格式规则。
- 模板开发原先固定的 pnpm@12.3.4 在 registry 中不存在，因此八个 Node 模板的开发版本对齐仓库 pnpm@10.14.0。生成到已有工作区时仍保留根版本。
- 为避免模板示例 scope 长度影响格式，将 Electron preload 引用提取为独立常量；相关文件按现有 Oxfmt 规则整理。
- 新增 mise run check:templates 并接入 Linux CI；检查源码构建、所有 Node 生成格式、两个 Go 项目和同工作区内两个 Electron 项目。

验证证据：

- 最初完成参数化迁移后，13 个模板 × 两组名称的全部生成文件与迁移前逐字节一致；随后可见差异仅为开发 pnpm 版本、描述文件、开发配置及上述格式/源码常量整理。
- mise run check 通过，包含 Go/E2E、文档检查及 Dashboard 95 个测试。
- Go API / Go Library 直接从模板源码测试和构建通过；Electron 直接从模板目录安装、格式检查和完整构建通过。
- React 模板使用 --ignore-workspace 直接安装与构建通过。Go API 模板实际启动后 /health 返回 HTTP 200（SQLite）。
- 依赖构建检查通过：在配置了仓库 pnpm 版本的临时工作区生成两个 Go 项目，以及 Alpha_Desktop / zulu-desktop 两个 Electron 项目，全部测试/构建和格式检查通过。
- 编译后的 CLI 在临时 HOME 中完成 go-api / electron-app 创建，module、scope 和开发文件排除正确。

边界：本次没有改动工作区创建器原有的默认 pnpm@12.3.4，也没有改写用户现有工作区版本。真实依赖构建检查使用预先配置 pnpm@10.14.0 的工作区；全新工作区默认版本不可下载的问题仍需单独处理。未在本次验证 Windows/macOS 打包，也未把 Electron 构建通过视为 GUI 与系统沙箱启动验证。
