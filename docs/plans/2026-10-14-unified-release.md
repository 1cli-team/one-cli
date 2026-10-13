# Unified CLI and Desktop Release Implementation Plan

**Goal:** 从 v0.1.22 开始，CLI 和 Desktop 的 13 个附件统一进入同一个稳定 Release，历史发布保持原样。

**Architecture:** 保留 `mise run release` 的干净副本、CLI 构建和冒烟验证。该入口只准备稳定 tag 与 CLI 草稿附件并显式触发 GitHub Actions；流水线固定源码、草稿 ID 和 CLI 校验文件摘要，构建五个平台桌面包，公证 macOS，下载校验全部 13 个附件后一次公开并设为 Latest。

**Tech Stack:** TypeScript / Node.js、GitHub Actions、GoReleaser 2.18.0、Tauri 2、GitHub CLI。

## 1. 发布协议

- 新增 `scripts/release-assets.mts` 作为版本分界及产物名称的单一来源。
- `scripts/release-rules.mts` 保留 CLI 六附件验证，另增加历史 / 统一 Release 的完整集合和草稿子集验证。
- 旧 `v0.1.21` 及更早版本仍只验证六个 CLI 附件；新版本缺少任何桌面附件都不能视为完整发布。

## 2. 本地入口

- 修改 `scripts/release.mts`：保持源码冻结、检查和 CLI 构建；草稿上传后下载校验，再 dispatch `desktop.yml`。
- 已有 CLI 附件只验证、不覆盖；重试只补缺失附件。已公开 Release 只读返回，旧版本禁止进入新的写入流程。
- 明确区分“已准备草稿并触发流水线”和“已公开发布”。

## 3. 桌面流水线及最终发布

- 修改 `.github/workflows/desktop.yml`，移除 Release published 触发，使用显式 workflow_dispatch。
- 修改 `scripts/desktop-release.mts`，不再创建 `desktop-v*` tag 或独立 Release。
- plan 冻结 tag 对应提交、草稿 ID 和 CLI checksums.txt 摘要；发布时再次核对，并保留稳定 tag 的原对象。
- 五个平台安装包完成后，验证本地记录与摘要，再向同一草稿上传七个桌面附件。下载校验全部附件，核对未发生并发状态改变，最终公开并设置 Latest。
- 失败保持草稿；已公开附件不可替换；重试成功版本只验证返回。

## 4. 文档和验证

- 更新 `RELEASING.md`、Desktop README 和任务说明的中英文内容，说明触发、恢复、历史分界、13 附件和 CLI 下载协议兼容性。
- 扩展已有发布规则测试，覆盖历史兼容、新版缺件、错版本文件、重复 / 意外附件、草稿恢复与公开版本幂等。
- 运行 `one mise run release:check`、仓库要求的串行 `check`，校验 workflow 与相关脚本。
- 本次不提交、不推送、不创建 tag、不修改远端 Release、不触发真实流水线。
