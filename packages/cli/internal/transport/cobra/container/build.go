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
		buildVersion, profileFlag, project, environment string
		dryRun                                          bool
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
			buildTag, err := resolveBuildTag(manifest, sub, names, buildVersion)
			if err != nil {
				return err
			}
			targets := names
			if sub != "" {
				targets = []string{sub}
			}
			combined := &container.BuildResult{}
			for _, target := range targets {
				platform := resolveBuildPlatform(
					root, manifest, target, environment, deps.detectKubeNodePlatform,
				)
				kind := containerKindForInvocation(manifest, target)
				reg, err := resolveBuildContainerRegistry(
					deps, root, profileFlag, kind, target, environment,
				)
				if err != nil {
					return err
				}
				result, err := deps.Service.Build(cmd.Context(), kind, container.BuildInput{
					ProjectRoot: root,
					Project:     target,
					TargetNames: []string{target},
					Tag:         buildTag,
					Platform:    platform,
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
					combined.Built = append(combined.Built, result.Built...)
				}
			}
			if dryRun {
				for _, e := range combined.Built {
					fmt.Fprintln(cmd.OutOrStdout(), strings.Join(e.Argv, " "))
				}
				return nil
			}
			output.Emit(combined)
			return nil
		},
	}
	cmd.Flags().StringVar(&buildVersion, "build-version", "", "非交互/CI 用镜像版本（如 v0.1.0）；TTY 未传且无 Git/package 默认版本时会提示选择")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "只打印 build 命令不实际构建")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "一次性使用指定 container profile（不改 default）")
	cmd.Flags().StringVarP(&project, "project", "p", "", "只构建指定 subproject（manifest 里的 name 或相对路径）")
	cmd.Flags().StringVar(&environment, "env", "", "使用指定环境的 container profile（如 dev / preview / prod）")
	helpui.MarkAdvanced(cmd, "profile", "project", "build-version", "env")
	i18n.MarkShort(cmd, "container.build.short")
	return cmd
}
