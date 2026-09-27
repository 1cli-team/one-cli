---
title: one build
description: 构建全部项目或指定项目。
---

`one build` 自动准备工具和应用依赖，再逐个执行项目构建任务。

```bash
one build
one build web
one build apps/web
one build -p web --env prod
one build --dry-run -o json
```

不指定项目时，构建 manifest 中所有可构建项目；从项目子目录执行也保持这个行为。项目名或工作区相对路径只选择该项目，不自动构建它依赖的其他本地项目。

## 构建命令

- Node：读取当前 `package.json`，使用工作区包管理器（pnpm / npm / yarn / bun）执行 build 脚本。
- Go：使用项目 `Taskfile.yml` 中的 `task build`。Go API 模板生成 `bin/server`；Go 库模板使用 `go build ./...` 编译包。旧的 Go 库可手动给 Taskfile 补上该任务。

Go 项目必须有 Taskfile。全量构建跳过没有 build 任务的项目，记录 `no-build-task`；显式选择这类项目时返回 `RUNTIME_TASK_NOT_FOUND`。配置损坏或必要文件缺失会在准备依赖前报错。没有任何构建任务的工作区也会报错。产物位置由项目自己的构建脚本决定。

## 执行顺序

全量构建串行执行，先构建本地 Node 依赖，再构建使用它们的项目。依赖来源为 dependencies、devDependencies、optionalDependencies，按 package.json 的 name 匹配；file: / link: 依赖按目录匹配。无依赖约束的项目保持 manifest 遍历顺序。重复包名和循环依赖在执行前报错。Go 的包依赖由 Go 工具链处理。

首个构建失败后停止，剩余任务记录为 `not_run`，保留失败子进程的退出码。Ctrl+C 停止当前构建及其子进程。日志带项目名前缀。

## 工具、依赖与环境变量

沿用 `one dev` 的 mise/builtin 自动选择与依赖准备方式，也覆盖没有 dev 命令的库项目。Node 依赖在工作区根目录统一安装一次。每个构建通过 `one run` 使用项目环境变量和 PATH，在项目目录执行。`--env` 指定环境，省略时使用 manifest 默认环境。

`--dry-run` 展示排序后的命令、目录和跳过项目，不安装工具或依赖、不读取密钥、不联网、不写文件。

## 输出

`-o json` / `-o yaml` 预览返回 `one-cli/build-plan/v1`，实际执行返回 `one-cli/build-result/v1`。过程日志写入 stderr，stdout 保持可解析。结果包含每个项目的状态、命令、耗时和退出码。依赖准备失败时返回错误，构建任务保持 `not_run`。

镜像构建使用 [`one container build`](/zh/docs/container/)，自定义命令使用 [`one run`](/zh/docs/run/)。
