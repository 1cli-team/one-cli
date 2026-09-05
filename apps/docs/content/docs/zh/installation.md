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
