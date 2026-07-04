// gen-error-codes renders apps/docs/content/docs/zh/error-codes.md
// from internal/errors.Codes (the source of truth).
//
// Run via Taskfile: `task gen-error-codes`. Diff the result; commit.
//
// The grouping is hardcoded by code-prefix here because the runtime
// `Codes` map carries no group metadata. Adding a new group → extend
// the `groups` slice. Adding a new code that doesn't match any group
// prefix → it falls into "Misc" (visible in output as a heading, so
// the omission is loud).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
)

type group struct {
	title  string
	intro  string
	prefix []string // any of these prefixes claims a code
}

var groups = []group{
	{
		title: "通用 / 生命周期",
		intro: "命令本身的失败、用户取消、内部序列化错误。",
		prefix: []string{
			"ONE_CLI_ERROR", "UNKNOWN_COMMAND",
			"PROMPT_CANCELLED", "OUTPUT_MARSHAL_FAILED",
		},
	},
	{
		title: "工作区 / 项目",
		intro: "工作区识别、命名规则、目标目录冲突等。",
		prefix: []string{
			"NOT_ONE_PROJECT", "NODE_VERSION_UNSUPPORTED",
			"INVALID_NAME", "INVALID_WORKSPACE_ROOTS",
			"PROJECT_NAME_REQUIRED", "EXISTING_TARGET_NOT_EMPTY",
			"TARGET_EXISTS",
		},
	},
	{
		title:  "Manifest",
		intro:  "`one.manifest.json` 的格式 / 缺失 / 内容问题。",
		prefix: []string{"MANIFEST_"},
	},
	{
		title: "模板 / 注册表",
		intro: "模板注册表的拉取、解析、查找。",
		prefix: []string{
			"REGISTRY_", "NO_TEMPLATES", "TEMPLATE_",
			"SUBPROJECT_NAME_REQUIRED",
		},
	},
	{
		title:  "Workspace 后置同步",
		intro:  "manifest 写入后某个 per-domain 后端 sync 失败 / 回滚（由 `create` / `add` 抛出）。",
		prefix: []string{"STATUS_FIX_"},
	},
	{
		title: "插件 / Profile / 部署",
		intro: "插件选择、profile 解析、部署 / CI 产物生成过程中的问题。",
		prefix: []string{
			"PLUGIN_", "PROFILE_",
			"IMAGE_REF_", "CI_", "K8S_", "LOCAL_ORCH_", "RELEASE_FLOW_",
		},
	},
	{
		title:  "Agent 文档 / Skills",
		intro:  "`AGENTS.md` / `CLAUDE.md` / `.one/agents/**` 生成与 bundled skill 安装。",
		prefix: []string{"AI_", "SKILLS_"},
	},
	{
		title:  "Env — 输入校验",
		intro:  "`one env` 命令的入参校验、覆写冲突等（与 Infisical 后端无关）。",
		prefix: []string{"ENV_"},
	},
	{
		title:  "Infisical 后端",
		intro:  "与 Infisical API 交互过程中的认证、权限、网络问题。",
		prefix: []string{"INFISICAL_"},
	},
}

func groupFor(code string) int {
	for i, g := range groups {
		for _, p := range g.prefix {
			if code == p || strings.HasPrefix(code, p) {
				return i
			}
		}
	}
	return -1 // Misc bucket
}

func main() {
	out, err := render()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen-error-codes:", err)
		os.Exit(1)
	}

	// Path resolved relative to the repo root. Taskfile invokes this tool
	// with `dir: packages/cli`, so we walk two levels up before descending
	// into apps/docs/.
	target := filepath.Join("..", "..", "apps", "docs", "content", "docs", "zh", "error-codes.md")
	if err := os.WriteFile(target, []byte(out), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Printf("gen-error-codes: wrote %s (%d codes)\n", target, len(cliErrors.Codes))
}

func render() (string, error) {
	// Bucket codes into groups.
	buckets := make([][]string, len(groups))
	misc := []string{}
	for code := range cliErrors.Codes {
		s := string(code)
		if g := groupFor(s); g >= 0 {
			buckets[g] = append(buckets[g], s)
		} else {
			misc = append(misc, s)
		}
	}
	for i := range buckets {
		sort.Strings(buckets[i])
	}
	sort.Strings(misc)

	var b strings.Builder
	b.WriteString(`---
title: 错误码大全
description: One CLI 所有错误码的 code / context / remediation 参考。本文件由 internal/errors/codes.go 自动生成，请勿手工编辑。
---

import { Callout } from "fumadocs-ui/components/callout";

<Callout type="info">
本页由 ` + "`task gen-error-codes`" + ` 从 ` + "`internal/errors/codes.go`" + ` 自动生成。
要改文案，改源文件后重跑命令；不要手工编辑这个 .md。
</Callout>

## 这是什么

每个 ` + "`one`" + ` 命令出错时都会发出一个**结构化错误信封**：

` + "```json" + `
{
  "schema": "one-cli/error/v1",
  "error": {
    "code": "TEMPLATE_NOT_FOUND",
    "message": "...",
    "context": { "available_templates": ["nestjs-api", "go-api", "..."] },
    "remediation": [
      {
        "action": "use-different-template",
        "hint": "用注册表里的模板",
        "command": "one add nestjs-api --name api"
      }
    ]
  }
}
` + "```" + `

字段含义：

- **` + "`error.code`" + `** —— 稳定、可路由的标识符；agent 按 code 分支，不要按 message 文本分支
- **` + "`error.context`" + `** —— 错误现场的关键数据；常常已经包含恢复需要的信息（例如 ` + "`available_templates`" + ` 已经在错误里，agent 不用再调一次 ` + "`one templates`" + `）
- **` + "`error.remediation`" + `** —— 恢复动作列表，每条带 ` + "`action`" + ` / ` + "`hint`" + ` / 可选 ` + "`command`" + `；agent 挑一条执行后重试

下面按命令域分组列出所有 code。

`)

	for i, g := range groups {
		if len(buckets[i]) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", g.title, g.intro)
		for _, code := range buckets[i] {
			renderCode(&b, code, cliErrors.Codes[cliErrors.Code(code)])
		}
	}
	if len(misc) > 0 {
		b.WriteString("## 未分组\n\n以下错误码未匹配任何分组前缀，请补充 `tools/gen-error-codes/main.go` 的 `groups` 表。\n\n")
		for _, code := range misc {
			renderCode(&b, code, cliErrors.Codes[cliErrors.Code(code)])
		}
	}

	return strings.TrimRight(b.String(), "\n") + "\n", nil
}

func renderCode(b *strings.Builder, code string, def cliErrors.Definition) {
	fmt.Fprintf(b, "### `%s`\n\n", code)
	if def.Summary != "" {
		fmt.Fprintf(b, "%s\n\n", def.Summary)
	}
	if len(def.Remediation) == 0 {
		b.WriteString("> 没有默认 remediation。具体恢复方式请看错误的 `context` 字段。\n\n")
		return
	}
	b.WriteString("**Remediation**:\n\n")
	for _, r := range def.Remediation {
		fmt.Fprintf(b, "- `%s` — %s", r.Action, r.Hint)
		if r.Command != "" {
			fmt.Fprintf(b, "<br />运行：`%s`", r.Command)
		}
		if r.Destructive {
			b.WriteString(" *(destructive)*")
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
}
