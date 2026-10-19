# One Core

CLI 的本地执行与工程模型 Go 库；模块路径保持为 `github.com/torchstellar-team/one-cli/packages/one-core`。

采用 [golang-standards/project-layout](https://github.com/golang-standards/project-layout) 的 `pkg` / `internal` 边界。该文档是社区目录约定，并非 Go 官方标准。Core 是库，无独立可执行文件，因此不创建空的 `cmd`、`deployments` 或 `api` 目录。

```text
one-core/
  go.mod
  go.sum
  Taskfile.yml
  pkg/
    bootstrap/       组装共享服务，返回显式依赖
    application/     应用用例
    core/            领域模型
    modules/         任务、创建、技能等本地功能
    platform/        语言、偏好、进程、安装和更新能力
    ports/           运行时、变量和存储契约
  internal/
    adapters/        本地运行时等具体实现
    resources/       模板、registry 和技能的内嵌资源
```

`packages/cli` 和 `services/server` 通过 `pkg/...` 引用共享能力。具体适配器由 `pkg/bootstrap` 组装，消费方不直接导入 `internal`。HTTP 和 Cobra 传输层仍分别归属 API 与 CLI。

内嵌资源由仓库根的 `mise run sync-bundled` 生成，源码中的 `go:embed` 保留在资源包内。运行 `one run build -p one-core` 构建此库，`one run check -p one-core` 执行静态检查；首次使用可直接运行根级 `mise run check`。

## English

One Core owns local execution and shared engineering models. Management use cases, persistence and provider credentials live in Server; the shared HTTP client lives in one-client. Its module identity remains unchanged. The community project-layout convention separates intentionally importable packages under `pkg/` from private adapters and embedded resources under `internal/`. Go enforces the latter boundary.

Consumers import `pkg/...`; `pkg/bootstrap` assembles private adapters. The library has no executable, so it needs no `cmd/` directory. HTTP and Cobra remain in their respective applications. Generate embedded resources with `mise run sync-bundled`, build with `one run build -p one-core`, and run the complete repository gate with `mise run check`.
