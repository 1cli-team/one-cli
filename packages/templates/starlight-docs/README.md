# Starlight 文档站点模板

基于 Astro + Starlight 的开箱即用文档模板，适合产品文档、开发者文档和团队知识库。

## 快速开始

```bash
pnpm install
pnpm dev
```

默认本地地址：`http://localhost:4321`

## 常用命令

| 命令             | 说明                   |
| ---------------- | ---------------------- |
| `pnpm dev`       | 启动本地开发服务器     |
| `pnpm build`     | 构建生产产物           |
| `pnpm preview`   | 预览构建产物           |
| `pnpm check`     | 执行代码检查和格式检查 |
| `pnpm check:fix` | 自动修复检查问题       |

## 文档目录

- `src/content/docs/`：文档正文（`.md` / `.mdx`）
- `src/content/docs/guides/`：指南内容
- `src/content/docs/reference/`：参考文档（自动生成侧边栏）
- `astro.config.mjs`：站点标题、侧边栏和社交链接配置
- `src/styles/custom.css`：Starlight 主题变量与全局外观定制
