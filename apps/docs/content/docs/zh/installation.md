---
title: 安装
description: 把 one cli 装到本机 — macOS / Linux 一行装，Windows 手动下载，含升降级与卸载。
---

把 `one` 二进制装到 PATH 上，5 秒钟的事。

**适合读这页的人**：第一次装 / 想升级或降级 / 想换安装位置 / 想卸载。

**读完会**：本机 `one --version` 能跑通，知道升降级语义和环境变量。

## macOS / Linux 一行装

```bash
curl -fsSL https://one.torchstellar.com/install.sh | bash
```

脚本会：

1. 检测 `$os/$arch`（darwin/linux × amd64/arm64）
2. 从 `one.torchstellar.com/dl/latest` 解析最新版本
3. 下载对应 tarball + 校验 SHA256
4. 解压到 `~/.local/bin/one`
5. 提示 PATH 是否需要补全

**审计脚本**：直接浏览器访问 `https://one.torchstellar.com/install.sh`，纯文本可读。

跑完确认：

```bash
one --version
# 0.1.0 (or later)
```

PATH 没配的话脚本会提示，照着做：

```bash
# zsh:
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc

# bash:
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
```

新开 shell 即可。

## Windows / 不想跑脚本

从 [GitHub Releases](https://github.com/torchstellar-team/one-cli/releases/latest) 下载对应平台归档（`darwin/linux/windows × amd64/arm64`），解压把 `one` 放到 PATH。

例（Linux amd64）：

```bash
curl -L -o one.tar.gz \
  https://github.com/torchstellar-team/one-cli/releases/latest/download/one-cli_linux_amd64.tar.gz
tar -xzf one.tar.gz
mv one ~/.local/bin/
one --version
```

Windows 类似——下载 `one-cli_windows_amd64.zip`，解压把 `one.exe` 放到 PATH 上的目录。

## 升级与降级

`install.sh` 会先读已装 `one --version` 再决定怎么处理：

| 现状 | 行为 |
|---|---|
| 没装过 | 直接装 |
| 目标更新 | 自动升级 |
| 目标相同 | **跳过**；要修复损坏的 binary 设 `ONE_FORCE=1` 强制重装 |
| 目标更旧 | **拒绝**降级；确认要降级设 `ONE_FORCE=1` |

也就是说升级根本不需要任何 flag，重跑安装命令就行。降级 / 修复才用 `ONE_FORCE`。

## 装完之后：跑一次 `one skills install`

`install.sh` 只把 `one` 二进制装到 PATH。要让 Claude Code / Cursor / Codex 等 agent 识别 One CLI skills，**还需要手工跑一次**：

```bash
one skills install
```

它会自动检测本机已装的所有受支持 agent，让你勾选装到哪些（默认全选；空格切换；回车确认）。

非交互场景：

```bash
one skills install --yes        # 装到所有检测到的 agent（CI 用）
one skills install --agent claude-code  # 只装到指定 agent
```

升级 binary 后再跑一次 `one skills install` 可以把最新的 skill 内容刷进去，幂等。当前内置入口包括 `one-cli`（新建、追加、依赖、参考）和 `one-migrate`（迁移已有项目）。

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

详见 [安装 Skill 到 Agent](/zh/tutorials/skills-install/)。

## 环境变量参考

`install.sh` 接受这些环境变量：

| 变量 | 默认 | 说明 |
|---|---|---|
| `ONE_VERSION` | （读 `/dl/latest`） | 锁版本，例如 `v0.1.0` |
| `ONE_INSTALL_DIR` | `$HOME/.local/bin` | 安装目录 |
| `ONE_FORCE` | `0` | 设为 `1` 允许降级 / 同版本重装 / 覆盖读不出版本号的二进制 |
| `ONE_BASE_URL` | `https://one.torchstellar.com` | 镜像源覆盖（调试用） |
| `ONE_SKIP_VERIFY` | `0` | 设为 `1` 跳过 SHA256 校验（仅调试） |

例：装一个特定旧版本到自定义目录：

```bash
curl -fsSL https://one.torchstellar.com/install.sh | ONE_VERSION=v0.1.0 ONE_INSTALL_DIR=/opt/bin bash
```

## 卸载

```bash
rm ~/.local/bin/one
```

如果之前跑过 `one skills install` 想把 skills 也清掉：手工删除对应 agent 的 `~/.<agent>/skills/one-cli` / `~/.<agent>/skills/one-migrate` 软链；或整个 `~/.one/skills-store/` 目录。也可以手工 `rm -rf ~/.config/one` 把所有 profile 凭据 + 缓存一并清掉。

## 本地编译版（贡献开发用）

如果你要改 one cli 自己的代码，看 [CONTRIBUTING.md](https://github.com/torchstellar-team/one-cli/blob/master/CONTRIBUTING.md)。一句话：

```bash
git clone https://github.com/torchstellar-team/one-cli
cd one-cli
brew install go go-task     # macOS；Linux 类比
task install-local           # 编译当前分支并 symlink 到 ~/.local/bin/one
hash -r
which one
one --version
```

开发期完整流程见 [CONTRIBUTING.md](https://github.com/torchstellar-team/one-cli/blob/master/CONTRIBUTING.md)；命令面速查见 [命令总览](/zh/docs/cli-overview/)。

## 装完了？

跳到 [快速开始](/zh/docs/quick-start/) 跑通第一个工作区。
