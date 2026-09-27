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

新建 workspace 的 `one run` 和 `one dev` 使用 mise 管理工具环境。**One 安装包不包含 mise；实际使用时优先采用 PATH 中兼容的 mise，否则复用或从官方 GitHub Release 下载固定版本 2026.9.7。** 下载后校验压缩包和程序 SHA256，再原子安装。支持 macOS、Linux 的 x64 / arm64 和 Windows x64；Linux 使用 musl 资源。[官方二进制分发说明](https://mise.jdx.dev/installing-mise.html)。

每次需要 runtime 时重新检查：系统 mise 被删除、版本过旧或不可执行时，One 转用托管版本；托管程序缺失或损坏时自动恢复。系统版本重新可用后恢复系统优先。显式设置 `ONE_MISE_BINARY` 的路径或版本有误时直接报错，不自动回退。

不需要激活 shell。配置信任遵循 mise 自身规则；设置 `MISE_PARANOID=1` 后需先显式审查并信任配置。旧 workspace 在显式启用前继续沿用已有工具，详见 [`one init mise`](/zh/docs/login/#mise-工作区工具配置)。

| 托管内容 | 默认目录 | 自定义根目录 |
|---|---|---|
| mise 程序 | `~/.local/share/one/runtimes/mise/<version>/<platform>/` | `XDG_DATA_HOME` |
| Node、Go、插件等 | `~/.local/share/one/mise/` | `XDG_DATA_HOME` |
| mise 全局配置 | `~/.config/one/mise/` | `XDG_CONFIG_HOME` |
| 状态、信任记录 | `~/.local/state/one/mise/` | `XDG_STATE_HOME` |
| 可清理缓存 | `~/.cache/one/mise/` | `XDG_CACHE_HOME` |

Windows 同样使用有效用户 Home 下的对应目录。XDG 根必须是绝对路径。托管模式会在子进程中设置 `MISE_DATA_DIR`、`MISE_CONFIG_DIR`、`MISE_STATE_DIR`、`MISE_CACHE_DIR`，覆盖继承的同名变量；外部 mise 保留原目录设置。项目配置文件仍留在项目内。切换到托管模式时，工具可能需要重新安装，配置可能需要重新授权；不复制系统配置或信任记录。

One 仅修改子进程 PATH。托管版本随 One 更新，自动升级和更新提示在该子进程中关闭；自行用 `mise self-update` 替换托管程序后，下次运行会按固定摘要修复。旧版 One 缓存中的同版本程序经校验后可迁移，旧文件保留。缓存与程序、工具、状态分开，清理缓存不会删除工具或信任记录。

**离线使用**：已有可用系统版本、托管程序或可迁移旧缓存时无需下载 mise。全新环境没有这些程序时需要联网；可提前安装兼容 mise 或用 `ONE_MISE_BINARY` 指定已准备的程序。Node、Go、hk 和项目依赖也须提前安装，mise 程序可用并不代表这些工具已可离线使用。

| 变量 | 用途 |
|---|---|
| `ONE_MISE_BINARY` | 显式指定 mise 可执行文件的绝对路径，外部程序最低支持版本为 2026.9.7 |
| `ONE_RUNTIME=builtin` | 临时诊断时使用机器原有工具，跳过 mise |

下载、迁移、写入或校验失败返回 `MISE_INSTALL_FAILED`；检查到 GitHub Releases 的网络/代理及托管目录权限后重试原命令。显式外部文件不存在时返回 `MISE_NOT_FOUND`。帮助、`--dry-run`、创建项目和生成配置不准备 runtime。

需要访问 mise 的原生命令时，使用 `one mise`，无需把托管程序目录加入 PATH：

```bash
one mise --version
one mise doctor
one mise trust .mise/conf.d/one.toml
one mise trust apps/web/.mise/conf.d/one.toml
one mise exec -- pnpm install
```

`trust` 请在审查对应配置后运行；自定义配置同样遵循 mise 的信任规则。安装依赖的例子应在 workspace 根目录执行。`one mise` 原样转发参数、IO 和退出码，不额外注入 One 项目密钥；需要项目密钥时继续使用 `one run`。`one mise --help` 展示 One 的入口说明，不探测或下载 mise。

## Infisical 登录

运行 `one login` 在浏览器中登录，会话保存在系统钥匙串。参见[登录与共享凭据](/zh/docs/login/)。

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

运行 `one logout` 删除系统钥匙串中的当前会话。

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

## Agent skill

运行 `one skills install`，将内置 `one-cli` skill 安装到 coding agent 的用户级 skills 目录。目标选择和支持的 Agent 见 `one skills install --help`，详见 [Skills](./skills)。
