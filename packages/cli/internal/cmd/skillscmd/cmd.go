// Package skillscmd contributes `one skills` to the root command via
// cliexts. Today it owns one verb (`one skills install`) — install /
// refresh the bundled skills on the current machine. Replaces the v0.5
// `one setup skills` entry point: same agent detection, same flags,
// same on-disk layout. Splitting out of `setup` lets `one configure`
// own the credential surface end-to-end while skill installation
// stays a separate, idempotent operation.
package skillscmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/cliexts"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/prompt"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/skills"
	"github.com/torchstellar-team/one-cli/packages/cli/pkg/agentskills"
)

func init() {
	cliexts.Register("skills", buildContributions)
}

func buildContributions() []*cobra.Command {
	parent := &cobra.Command{
		Use: "skills",
		Long: `把 bundled one-cli / one-migrate skills 装到本机选定的 coding agent。

子命令：
  one skills install   安装 / 刷新 bundled skills 到本机`,
	}
	parent.AddCommand(newInstallCmd())
	i18n.MarkShort(parent, "skills.short")
	return []*cobra.Command{parent}
}

type installFlags struct {
	agents []string
	yes    bool
}

func newInstallCmd() *cobra.Command {
	flags := &installFlags{}
	cmd := &cobra.Command{
		Use:   "install",
		Short: "安装 / 刷新 one CLI 自带 skills 到本机选定的 agent",
		Long: `安装 / 刷新 one CLI 自带的 one-cli / one-migrate skills 到本机。

会先自动检测所有受支持的 coding agent（Claude Code / Cursor / Codex /
Gemini CLI / GitHub Copilot / OpenCode / Cline 等 50+），然后让你
**勾选**装到哪些（默认只勾 Claude Code；↑/↓ 移动；空格勾选/取消；回车开始安装）。

非交互场景：
  --agent claude-code --agent cursor   # 只装这两个
  --yes                                 # 装到所有检测到的（CI 用）

幂等：跑多次都安全。升级 binary 后跑一次刷新 skills 内容到所有目标。

skills materialize 到 ~/.one/skills-store/one-bundled/<skill-name>/，每个
目标 agent 在其 global skills 目录创 symlink 指向 store。Windows
自动 fallback 到 copy。`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runInstall(flags)
		},
	}
	cmd.Flags().StringSliceVarP(&flags.agents, "agent", "a", nil,
		"目标 agent ID（可重复；不传则交互式多选）")
	cmd.Flags().BoolVarP(&flags.yes, "yes", "y", false,
		"非交互：装到所有检测到的 agent，跳过 prompt")
	return cmd
}

func runInstall(flags *installFlags) error {
	targets, err := resolveTargets(flags)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return cliErrors.New(cliErrors.SKILLS_INSTALL_FAILED,
			"no target agents selected")
	}

	res, err := skills.InstallTo(targets)
	if err != nil {
		return err
	}

	output.Emit(&result{
		Schema:      "one-cli/skills-install/v1",
		Status:      "completed",
		Targets:     summariseTargets(targets),
		InstalledTo: res.InstalledTo,
		SkillCount:  res.SkillCount,
	})
	return nil
}

// resolveTargets implements the three-way decision:
//   - explicit --agent flags → use those (validate against registry)
//   - non-TTY OR --yes → use all detected (with claude-code fallback)
//   - TTY without flags → multi-select prompt with claude-code
//     pre-checked (when detected)
func resolveTargets(flags *installFlags) ([]agentskills.Agent, error) {
	if len(flags.agents) > 0 {
		out := make([]agentskills.Agent, 0, len(flags.agents))
		for _, id := range flags.agents {
			a, ok := agentskills.GetByID(id)
			if !ok {
				return nil, cliErrors.New(cliErrors.SKILLS_INSTALL_FAILED,
					fmt.Sprintf("unknown agent id %q (run `one skills install --help`)", id))
			}
			out = append(out, a)
		}
		return out, nil
	}

	detected := agentskills.Detect()
	if len(detected) == 0 {
		fallback, ok := agentskills.GetByID("claude-code")
		if !ok {
			return nil, cliErrors.New(cliErrors.SKILLS_INSTALL_FAILED,
				"claude-code missing from agent registry (build error)")
		}
		fmt.Fprintln(os.Stderr,
			"未检测到任何已安装的 agent，将装到 claude-code 默认路径。")
		return []agentskills.Agent{fallback}, nil
	}

	if flags.yes || !output.IsTTY() {
		return detected, nil
	}

	options := make([]prompt.Option[string], 0, len(detected))
	defaults := make([]string, 0, 1)
	for _, a := range detected {
		options = append(options, prompt.Option[string]{
			Label:       a.DisplayName,
			Description: a.GlobalPath,
			Value:       a.ID,
		})
		if a.ID == "claude-code" {
			defaults = append(defaults, a.ID)
		}
	}
	picked, err := prompt.MultiSelect(
		"选择要安装到的 agent（↑/↓ 移动，空格勾选/取消，回车开始安装；默认仅 Claude Code）",
		options, defaults)
	if err != nil {
		return nil, err
	}
	out := make([]agentskills.Agent, 0, len(picked))
	for _, id := range picked {
		if a, ok := agentskills.GetByID(id); ok {
			out = append(out, a)
		}
	}
	return out, nil
}

func summariseTargets(agents []agentskills.Agent) []target {
	out := make([]target, 0, len(agents))
	for _, a := range agents {
		out = append(out, target{
			AgentID:     a.ID,
			DisplayName: a.DisplayName,
			GlobalPath:  a.GlobalPath,
		})
	}
	return out
}

type result struct {
	Schema      string   `json:"schema"`
	Status      string   `json:"status"`
	Targets     []target `json:"targets"`
	InstalledTo []string `json:"installed_to"`
	SkillCount  int      `json:"skill_count"`
}

type target struct {
	AgentID     string `json:"agent_id"`
	DisplayName string `json:"display_name"`
	GlobalPath  string `json:"global_path"`
}

func (r *result) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	fmt.Fprintf(w, "✓ Installed bundled skills to %d agent%s\n",
		len(r.Targets), pluralS(len(r.Targets)))
	for _, t := range r.Targets {
		fmt.Fprintf(w, "  · %s → %s\n", t.DisplayName, t.GlobalPath)
	}
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
