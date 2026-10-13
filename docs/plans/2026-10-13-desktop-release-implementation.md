# One Desktop Release Implementation Plan

**Goal:** Build five desktop installers in GitHub Actions, sign and notarize macOS, and publish a verified desktop Release.

**Architecture:** Reuse a published CLI `vX.Y.Z` tag and its exact source SHA. Publish `desktop-vX.Y.Z` separately, keeping CLI Latest and its six-asset contract intact. macOS sidecars are signed before their hashes enter the manifest, then copied unchanged into Tauri's signed app.

**Tech Stack:** Tauri 2, Go, React, Node TypeScript scripts, mise, GitHub Actions, Apple codesign/notarytool/stapler.

## Implementation

1. Update `scripts/desktop.mts`: allow installer releases without updater keys; sign macOS sidecars before hashing and package those signed files without a second signature. Keep development builds and other platforms unchanged.
2. Add `scripts/desktop-macos.mts`: import a Developer ID Application p12 into a temporary keychain, verify its Team ID, sign sidecars, verify Tauri's app notarization, notarize/staple the DMG, and clean up even after failure.
3. Add `scripts/desktop-release.mts`: validate the published CLI tag and pinned source, collect installers with checksums, create/reuse a desktop draft, download and verify its attachments, then publish without changing CLI Latest. Never replace published assets or move tags.
4. Add `.github/workflows/desktop.yml`: published CLI Release and manual triggers; native macOS arm64/x64, Windows x64, Linux arm64/x64 jobs; read-only builds and a separate contents-write publisher. Use the five standard Apple Secrets only in macOS packaging.
5. Update `mise.toml`, `RELEASING.md`, and `apps/desktop/README.md` with bilingual configuration, root commands, triggers, retry rules, artifact names, and the distinction between Apple credentials and updater keys.
6. Run existing repository checks through One/mise, validate workflow syntax, build a local macOS installer, and verify packaged sidecar hashes. Actual Developer ID signing, notarization, and remote five-platform results require configured GitHub Secrets and a published CLI tag containing these changes; report them separately from local validation.

## 凭据与验收

仓库 Secrets：`APPLE_TEAM_ID`、`APPLE_ID`、`APPLE_PASSWORD`（应用专用密码）、`APPLE_CERTIFICATE_BASE64`（包含私钥的 Developer ID Application p12 的 base64）、`APPLE_CERTIFICATE_PASSWORD`。不把值写入源码、文档或日志。

复用现有质量检查，不新增测试代码。发布前必须收齐五个平台安装包，macOS 必须通过签名、公证、票据和内置 CLI 摘要验证。失败保留草稿；重试仍使用同一 CLI tag 和源码。自动更新 feed 与恢复验收不属于本次安装包流水线。
