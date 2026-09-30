---
title: dev 任务
description: 通过 mise 执行已经存在的开发任务。
---

`dev` 是普通 mise 任务。使用 `one run dev`；`one dev` 只是通用任务简写，没有独立的内置实现。

```sh
one run dev
one run dev -p web -p api
one run dev -p apps/web --dry-run -o json
one run dev --ui tui
one run dev -p web -- --port 4300
```

不指定 `-p` 时，根 dev 任务必须存在；指定 `-p` 时，对应项目的 dev 任务必须存在。缺失时直接报错，不从 manifest、start 脚本或 Go 源码目录推断命令。显式运行 `one init mise`，可将已有包脚本和 Taskfile 任务投影为原生 mise 命令。自定义命令直接编辑 mise 任务。

## 准备与环境变量

所有命名任务共用准备策略。Node 依赖在工作区根目录准备；pnpm 10.14 及以上会检查现有安装并复用匹配的依赖，包括手动安装的依赖。需要安装时执行 `pnpm install --no-frozen-lockfile`。Go 准备会解析固定模块构建列表，不自动执行 `go mod tidy` 或 `go work sync`。准备失败则不启动任务；`one exec` 不安装依赖。

One 按任务实际工作目录匹配登记项目。项目启用环境变量时，启动只向 Infisical 发起一次递归读取，获取所选环境的变量，再在内存中按项目目录分发：根目录与父目录的变量共享，项目目录的同名变量覆盖父目录，其他项目和更深层子目录的变量不会混入。并行项目各自使用独立变量表，孙进程继承对应项目的环境；下次启动重新获取最新值。无需在 mise 中声明 env 插件或填写密钥值。

## 终端与退出

默认并发为任务图中的每个节点分配一个槽位；显式 `--concurrency` 会被遵守。并发过低时，长期运行任务可能占满其他服务需要的槽位。有限时长的前置任务完成后，才启动依赖它的服务。

`--ui stream` 显示前缀日志；`--ui tui` 支持任务选择、搜索、滚动和跟随。交互式/raw 任务需要独占终端。`--ui raw` 由 mise 处理终端并关闭产物缓存。TUI 与 stream 的调度和退出行为一致。

Ctrl+C 或 SIGTERM 会停止本次调用及子进程。子进程日志保持原样；One 自己不打印注入的变量值。JSON/YAML 模式将子进程日志写入 stderr。Dry-run 静态读取配置，不加载密钥或写入文件。

配置、缓存行为和输出偏好见 [one run](/zh/docs/run/)。
