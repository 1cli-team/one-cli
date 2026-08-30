package containercmd

// push.go: `one container push` subcommand. Same dispatch shape as
// build but requires a configured registry profile (push has nowhere
// to send the image without one).

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/container"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/helpui"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

func newPushCmd(deps Dependencies) *cobra.Command {
	var (
		buildVersion, profileFlag, project, environment string
		dryRun                                          bool
	)
	cmd := &cobra.Command{
		Use:   "push",
		Short: "推送项目镜像到 registry",
		Long: `把 ` + "`one container build`" + ` 产物推到 registry。

需要先 ` + "`one configure add container/<kind> --profile <name>`" + ` 配置 registry profile —
push 必须知道 registry host 才能拼出完整的镜像 tag。

profile 解析顺序：
  --profile <name>                          # 一次性覆盖
  → Dashboard 为 Project + --env 环境选择的 profile
  → Dashboard 为 Workspace + --env 环境选择的 profile
  → 旧版 Workspace/Project profile 绑定
  → ~/.config/one/config.json#container/<kind>.default`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			activeWorkspace, err := execution.ResolveWorkspace(cmd.Context())
			if err != nil {
				return err
			}
			root := activeWorkspace.Root()
			manifest := activeWorkspace.Manifest()
			selector, err := containerProjectSelector(project, args)
			if err != nil {
				return err
			}
			sub := normalizeContainerProject(activeWorkspace, selector)
			names, err := containerSubprojects(manifest)
			if err != nil {
				return err
			}
			if sub != "" {
				enabled := false
				for _, n := range names {
					if n == sub {
						enabled = true
						break
					}
				}
				if !enabled && len(names) > 0 {
					return cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND,
						fmt.Sprintf("没有名为 %s 且启用了容器构建的项目", sub))
				}
			}
			targets := names
			if sub != "" {
				targets = []string{sub}
			}
			combined := &container.PushResult{}
			for _, target := range targets {
				kind := containerKindForInvocation(manifest, target)
				reg, err := resolveContainerRegistry(
					deps, root, profileFlag, kind, target, environment,
				)
				if err != nil {
					return err
				}
				result, err := deps.Service.Push(cmd.Context(), kind, container.PushInput{
					ProjectRoot: root,
					Project:     target,
					TargetNames: []string{target},
					Tag:         buildVersion,
					DryRun:      dryRun,
					Registry:    reg,
				})
				if err != nil {
					return err
				}
				if result != nil {
					if combined.Schema == "" {
						combined.Schema = result.Schema
					}
					combined.Pushed = append(combined.Pushed, result.Pushed...)
				}
			}
			if dryRun {
				for _, e := range combined.Pushed {
					if e.Retagged && e.SourceImage != "" {
						fmt.Fprintln(cmd.OutOrStdout(), "docker tag "+e.SourceImage+" "+e.Image)
					}
					fmt.Fprintln(cmd.OutOrStdout(), strings.Join(e.Argv, " "))
				}
				return nil
			}
			output.Emit(combined)
			return nil
		},
	}
	cmd.Flags().StringVar(&buildVersion, "build-version", "", "镜像版本（如 v0.1.0；未传时推送 manifest 记录的 build 产物）")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "只打印 push 命令不实际推送")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "一次性使用指定 container profile（不改 default）")
	cmd.Flags().StringVarP(&project, "project", "p", "", "只推送指定 subproject 的镜像（manifest 里的 name 或相对路径）")
	cmd.Flags().StringVar(&environment, "env", "", "使用指定环境的 container profile（如 dev / preview / prod）")
	helpui.MarkAdvanced(cmd, "profile", "project", "build-version", "env")
	i18n.MarkShort(cmd, "container.push.short")
	return cmd
}
