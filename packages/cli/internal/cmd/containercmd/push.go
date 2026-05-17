package containercmd

// push.go: `one container push` subcommand. Same dispatch shape as
// build but requires a configured registry profile (push has nowhere
// to send the image without one).

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/container"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func newPushCmd() *cobra.Command {
	var (
		buildVersion, profileFlag, project string
		dryRun                             bool
	)
	cmd := &cobra.Command{
		Use:   "push",
		Short: "推送项目镜像到 registry",
		Long: `把 ` + "`one container build`" + ` 产物推到 registry。

需要先 ` + "`one configure add container/<kind> --profile <name>`" + ` 配置 registry profile —
push 必须知道 registry host 才能拼出完整的镜像 tag。

profile 解析顺序：
  --profile <name>                          # 一次性覆盖
  → config.json#workspaces[workspaceId].projects[subproject].profiles[container/kind]
  → config.json#workspaces[workspaceId].profiles[container/kind]
  → ~/.config/one/config.json#container/<kind>.default`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := workspace.ResolveProjectRoot("")
			if err != nil {
				return err
			}
			selector, err := containerProjectSelector(project, args)
			if err != nil {
				return err
			}
			sub, err := normalizeContainerProject(root, selector)
			if err != nil {
				return err
			}
			names, err := containerSubprojects(root)
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
			kind := containerKindForInvocation(root, sub)
			provider, ok := container.Get(kind)
			if !ok {
				return cliErrors.New(cliErrors.CONTAINER_KIND_UNKNOWN,
					"container kind "+kind+" 未注册；检查 `projects[i].domains.container.kind` 值。")
			}
			reg, err := resolveContainerRegistry(root, profileFlag, kind, sub)
			if err != nil {
				return err
			}
			res, err := provider.Push(context.Background(), container.PushInput{
				ProjectRoot: root,
				Project:     sub,
				TargetNames: names,
				Tag:         buildVersion,
				DryRun:      dryRun,
				Registry:    reg,
			})
			if err != nil {
				return err
			}
			if dryRun && res != nil {
				for _, e := range res.Pushed {
					if e.Retagged && e.SourceImage != "" {
						fmt.Fprintln(cmd.OutOrStdout(), "docker tag "+e.SourceImage+" "+e.Image)
					}
					fmt.Fprintln(cmd.OutOrStdout(), strings.Join(e.Argv, " "))
				}
				return nil
			}
			if res != nil {
				for _, e := range res.Pushed {
					if err := workspace.SetProjectContainerImage(root, e.Project, e.Image); err != nil {
						return err
					}
				}
			}
			if res != nil {
				output.Emit(res)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&buildVersion, "build-version", "", "镜像版本（如 v0.1.0；未传时推送 manifest 记录的 build 产物）")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "只打印 push 命令不实际推送")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "一次性使用指定 container profile（不改 default）")
	cmd.Flags().StringVarP(&project, "project", "p", "", "只推送指定 subproject 的镜像（manifest 里的 name 或相对路径）")
	return cmd
}
