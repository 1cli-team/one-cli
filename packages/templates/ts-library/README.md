# TypeScript 通用库模板

面向可发布 npm 包的通用库模板，内置：

- 构建：`tsdown`
- 测试：`vitest`
- 代码质量：`oxlint` + `oxfmt`

## 开发

```bash
pnpm install
pnpm dev
```

## 构建

```bash
pnpm build
```

## 测试

```bash
pnpm test
pnpm test:coverage
```

## 发包

发包前请先修改 `package.json` 中的 `name`、`version` 等信息。

```bash
pnpm prepublishOnly
pnpm publish:public
```
