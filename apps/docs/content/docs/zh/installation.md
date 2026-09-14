---
title: 安装
description: 把 one cli 装到 macOS、Linux 或 Windows，含一行安装、升降级与卸载。
---

把 `one` 二进制装到 PATH 上，5 秒钟的事。

**适合读这页的人**：第一次装 / 想升级或降级 / 想换安装位置 / 想卸载。

**读完会**：本机 `one --version` 能跑通，知道升降级语义和环境变量。

## macOS / Linux 一行装

```bash
curl -fsSL https://1cli.dev/install.sh | bash
```

脚本会：

1. 检测 `$os/$arch`（darwin/linux × amd64/arm64）
2. 从 GitHub Releases 的 latest redirect 解析最新版本
3. 从对应 release assets 下载 tarball + 校验 SHA256
4. 解压到 `~/.local/bin/one`
5. 提示 PATH 是否需要补全

**审计脚本**：直接浏览器访问 `https://1cli.dev/install.sh`，纯文本可读。

跑完确认：

```bash
one --version
# 0.1.1 (or later)
```

PATH 没配的话脚本会提示，照着做：

```bash
# zsh:
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc

# bash:
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
```

新开 shell 即可。

## Windows PowerShell 一行装

Windows 10/11 x64 在 PowerShell 里运行：

```powershell
irm https://1cli.dev/install.ps1 | iex
```

脚本会下载 `one-cli_windows_amd64.zip`、对照 `checksums.txt` 校验 SHA256、把 `one.exe` 安装到 `%LOCALAPPDATA%\Programs\one\bin`，并把目录加入用户 PATH。第一次安装后新开一个终端即可。

**审计脚本**：浏览器打开 `https://1cli.dev/install.ps1`，纯文本可读。

## 手动下载

从 [GitHub Releases](https://github.com/1cli-team/one-cli/releases/latest) 下载对应平台归档，解压后把 `one`（Windows 为 `one.exe`）放到 PATH。Windows 当前发布 x64；macOS / Linux 发布 x64 和 arm64。

例（Linux amd64）：

```bash
curl -L -o one.tar.gz \
  https://github.com/1cli-team/one-cli/releases/latest/download/one-cli_linux_amd64.tar.gz
tar -xzf one.tar.gz
mv one ~/.local/bin/
one --version
```

Windows 归档名是 `one-cli_windows_amd64.zip`。

## 升级与降级

`install.sh` 和 `install.ps1` 都会先读已装 `one --version` 再决定怎么处理：

| 现状 | 行为 |
|---|---|
| 没装过 | 直接装 |
| 目标更新 | 自动升级 |
| 目标相同 | **跳过**；要修复损坏的 binary 设 `ONE_FORCE=1` 强制重装 |
| 目标更旧 | **拒绝**降级；确认要降级设 `ONE_FORCE=1` |

也就是说升级根本不需要任何 flag，重跑安装命令就行。降级 / 修复才用 `ONE_FORCE`。

## One 自动管理 mise

新建 workspace 的 `one run` 和 `one dev` 使用 mise 管理工具环境。**发布的 `one` 文件已内置当前平台的 mise 2026.9.7，无需单独安装，首次使用也无需下载 mise。** One 校验内置压缩包，解压并校验可执行文件后放入自己的缓存；后续运行复用缓存。macOS、Linux 的 x64 / arm64 和 Windows x64 发布文件分别携带对应平台资源。[官方二进制分发说明](https://mise.jdx.dev/installing-mise.html)。

不需要激活 shell。配置信任遵循 mise 自身规则；默认模式下执行命令可能自动信任当前配置，设置 `MISE_PARANOID=1` 后需先显式审查并信任配置；Node、Go 等工具下载由 mise 处理，应用依赖仍由包管理器安装，尚未安装的工具和依赖仍可能需要联网。旧 workspace 在显式启用前继续沿用已有工具，详见 [`one configure mise`](/zh/docs/configure/#mise-工作区工具配置)。

缓存位于 `$XDG_CACHE_HOME/one/runtimes/mise/<version>/<platform>/`，未设置时使用 `~/.cache/one/runtimes/mise/`。One 默认使用内置固定版本，仅在子进程 PATH 中加入缓存目录。缓存损坏会从内置资源重新解压，无需联网。内置 mise 随 One 升级；One 会关闭该子进程的 mise 自动升级和更新提示。内置资源会增加 One 发布文件体积；运行时需要可写、可执行的缓存目录。

| 变量 | 用途 |
|---|---|
| `ONE_MISE_BINARY` | 显式使用其他 mise 可执行文件的绝对路径，最低支持版本为 2026.9.7 |
| `ONE_RUNTIME=builtin` | 临时诊断时使用机器原有工具，跳过 mise |

解压、写入或校验失败返回 `MISE_INSTALL_FAILED`；不会执行不完整的文件。修复缓存权限后重试原命令，内置资源损坏时重新安装 One。显式指定的 mise 文件不存在时返回 `MISE_NOT_FOUND`。`--dry-run`、创建项目和生成配置不会解压 runtime。

需要访问 mise 的原生命令时，使用 `one mise`，无需把缓存目录加入 PATH：

```bash
one mise --version
one mise doctor
one mise trust .mise/conf.d/one.toml
one mise trust apps/web/.mise/conf.d/one.toml
one mise exec -- pnpm install
```

`trust` 请在审查对应配置后运行；自定义配置同样遵循 mise 的信任规则。安装依赖的例子应在 workspace 根目录执行。`one mise` 原样转发参数、IO 和退出码，不额外注入 One 项目密钥；需要项目密钥时继续使用 `one run`。`one mise --help` 展示 One 的入口说明，不触发解压。

## 配置 Provider 凭据

Provider 凭据用顶层 `one configure add <domain>/<backend> --profile <name>` 配（一次配全工作区都能用）。当前支持这些 pair：

| pair | 什么时候用 |
|---|---|
| `env/infisical` | Infisical 机器身份，跨工作区共享 |
| `deploy/aliyun-oss` | 阿里云 OSS，S3 协议对象存储 |
| `deploy/tencent-cos` | 腾讯云 COS，S3 协议对象存储 |
| `deploy/aws-s3` | AWS S3 |
| `deploy/minio` | 自部署 MinIO |
| `deploy/rustfs` | 自部署 RustFS |
| `deploy/r2` | Cloudflare R2 |
| `deploy/kustomize` | Kubernetes kubeconfig + context |
| `deploy/vercel` | Vercel API token |
| `deploy/cloudflare` | Cloudflare API token |
| `deploy/edgeone` | Tencent EdgeOne Pages API token |
| `container/docker` | 通用 Docker registry 登录信息 |
| `container/dockerhub` | Docker Hub 登录信息 |
| `container/ghcr` | GitHub Container Registry 登录信息 |
| `container/acr` | 阿里云 ACR 登录信息 |

`env/dotenv` 不需要远端凭据；它直接读写项目本地 `.env`。S3 兼容 deploy 后端共用同一组 profile 字段，但 backend ID 是显式拆开的（`deploy/aws-s3`、`deploy/aliyun-oss`、`deploy/r2` 等）。

常用配置例子：

```bash
one configure add env/infisical --profile work         # Infisical 凭据
one configure add deploy/aws-s3 --profile web-prod     # AWS S3 endpoint + ak/sk
one configure add deploy/kustomize --profile prod-k8s  # kubeconfig context
one configure add container/ghcr --profile ghcr        # GHCR username + PAT
```

## 环境变量参考

两个安装器都接受下列环境变量；PowerShell 从 `$env:变量名` 读取，默认安装目录是 `%LOCALAPPDATA%\Programs\one\bin`。

| 变量 | 默认 | 说明 |
|---|---|---|
| `ONE_VERSION` | （解析 GitHub latest release） | 锁版本，例如 `v0.1.1` |
| `ONE_INSTALL_DIR` | `$HOME/.local/bin`；Windows：`%LOCALAPPDATA%\Programs\one\bin` | 安装目录 |
| `ONE_FORCE` | `0` | 设为 `1` 允许降级 / 同版本重装 / 覆盖读不出版本号的二进制 |
| `ONE_REPO_URL` | `https://github.com/1cli-team/one-cli` | GitHub repo URL 覆盖（调试用） |
| `ONE_RELEASE_BASE_URL` | `$ONE_REPO_URL/releases/download` | release assets 下载源覆盖 |
| `ONE_LATEST_URL` | `$ONE_REPO_URL/releases/latest` | latest release 解析地址覆盖 |
| `ONE_SKIP_VERIFY` | `0` | 设为 `1` 跳过 SHA256 校验（仅调试） |
| `ONE_NO_PATH_UPDATE` | `0` | 设为 `1` 安装但不修改用户 PATH |

例：装一个特定旧版本到自定义目录：

```bash
curl -fsSL https://1cli.dev/install.sh | ONE_VERSION=v0.1.0 ONE_INSTALL_DIR=/opt/bin bash
```

## 卸载

PowerShell：

```powershell
Remove-Item "$env:LOCALAPPDATA\Programs\one\bin\one.exe"
```

macOS / Linux：

```bash
rm ~/.local/bin/one
```

如需清理本地 profile 凭据和缓存，可删除 `~/.config/one`。

## 本地编译版（贡献开发用）

如果你要改 one cli 自己的代码，看 [CONTRIBUTING.md](https://github.com/1cli-team/one-cli/blob/master/CONTRIBUTING.md)。一句话：

```bash
git clone https://github.com/1cli-team/one-cli
cd one-cli
brew install go go-task     # macOS；Linux 类比
task install                 # 打包 Dashboard + CLI，再创建当前平台的本地启动器
hash -r
which one
one --version
```

Windows 会创建 `~/.local/bin/one.exe`；如果系统不允许创建文件符号链接，
则自动退回 `one.cmd` 转发器。旧版生成的无扩展名 `one` 符号链接会被安全迁移。

开发期完整流程见 [CONTRIBUTING.md](https://github.com/1cli-team/one-cli/blob/master/CONTRIBUTING.md)；命令面速查见 [命令总览](/zh/docs/cli-overview/)。

## 装完了？

跳到 [快速开始](/zh/docs/quick-start/) 跑通第一个工作区。
