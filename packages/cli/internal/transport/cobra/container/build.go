package containercmd

// build.go: `one container build` subcommand. Resolves the target
// subproject + container kind + registry + tag + platform, then
// dispatches the Build verb through the injected container service.

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

func newBuildCmd(deps Dependencies) *cobra.Command {
	var (
		buildVersion, profileFlag, project string
		dryRun                             bool
	)
	cmd := &cobra.Command{
		Use:   "build [subproject]",
		Short: "构建项目的容器镜像",
		Args:  cobra.MaximumNArgs(1),
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
			profileSubproject := sub
			if profileSubproject == "" && len(names) == 1 {
				profileSubproject = names[0]
			}
			kind := containerKindForInvocation(manifest, profileSubproject)
			reg, err := resolveBuildContainerRegistry(deps, root, profileFlag, kind, profileSubproject)
			if err != nil {
				return err
			}
			platform := resolveBuildPlatform(root, manifest)
			buildTag, err := resolveBuildTag(manifest, sub, names, buildVersion)
			if err != nil {
				return err
			}
			res, err := deps.Service.Build(cmd.Context(), kind, container.BuildInput{
				ProjectRoot: root,
				Project:     sub,
				TargetNames: names,
				Tag:         buildTag,
				Platform:    platform,
				DryRun:      dryRun,
				Registry:    reg,
			})
			if err != nil {
				return err
			}
			if dryRun && res != nil {
				for _, e := range res.Built {
					fmt.Fprintln(cmd.OutOrStdout(), strings.Join(e.Argv, " "))
				}
				return nil
			}
			if res != nil {
				output.Emit(res)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&buildVersion, "build-version", "", "非交互/CI 用镜像版本（如 v0.1.0）；TTY 未传且无 Git/package 默认版本时会提示选择")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "只打印 build 命令不实际构建")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "一次性使用指定 container profile（不改 default）")
	cmd.Flags().StringVarP(&project, "project", "p", "", "只构建指定 subproject（manifest 里的 name 或相对路径）")
	helpui.MarkAdvanced(cmd, "profile", "project", "build-version")
	i18n.MarkShort(cmd, "container.build.short")
	return cmd
}
