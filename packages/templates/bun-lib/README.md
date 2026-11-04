# one-template-bun-lib

Bun TypeScript library in a pnpm workspace. One prepares dependencies and runs the package scripts.

pnpm 工作区中的 Bun TypeScript 共享库。One 准备依赖并执行 package.json 中的任务。

```sh
one run check -p one-template-bun-lib
one run build -p one-template-bun-lib
```

Add this package with `workspace:*` in a consuming project.

在使用方的 package.json 中通过 `workspace:*` 引用此包。
