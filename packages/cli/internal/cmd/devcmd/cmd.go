// Package devcmd contributes `one dev` to the root command via cliexts.
// Today there is one dev runner (Procfile-based local processes); this
// command calls into internal/localorch/process directly.
package devcmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/cliexts"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/i18n"
	processorch "github.com/torchstellar-team/one-cli/packages/cli/internal/localorch/process"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func init() {
	cliexts.Register("dev", buildContributions)
}

func buildContributions() []*cobra.Command {
	return []*cobra.Command{newDevCmd()}
}

// requireDev is retained for v4 backwards compatibility but is now a
// no-op: current workspaces always have the local dev runner enabled — the
// Procfile.dev synchronisation runs unconditionally during `one create`
// and `one add`, so any project root with a valid manifest is dev-ready.
func requireDev(projectRoot string) error {
	if _, err := workspace.ReadManifest(projectRoot); err != nil {
		return err
	}
	return nil
}

// _ keeps the cliErrors import even when requireDev no longer uses it,
// in case future hardening reintroduces an explicit gate.
var _ = cliErrors.BACKEND_NOT_ENABLED

func newDevCmd() *cobra.Command {
	var (
		project string
		dryRun  bool
	)
	cmd := &cobra.Command{
		Use: "dev",
		Long: `本命令把工作区里所有声明了 dev 命令的项目并行启动。命令读自
one.manifest.json 的 projects[].domains.dev.command，由 one add 在脚手架时
自动写入。内置 supervisor 处理子进程，无需安装任何第三方工具。`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := workspace.ResolveProjectRoot("")
			if err != nil {
				return err
			}
			if err := requireDev(root); err != nil {
				return err
			}
			processName, err := resolveProcessSelector(root, project)
			if err != nil {
				return err
			}
			res, err := processorch.Start(context.Background(), processorch.StartInput{
				ProjectRoot: root,
				DryRun:      dryRun,
				Process:     processName,
			})
			if err != nil {
				return err
			}
			if dryRun && res != nil {
				fmt.Fprintln(cmd.OutOrStdout(), strings.Join(res.Argv, " "))
				return nil
			}
			if res != nil {
				output.Emit(res)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "只打印将执行的命令")
	cmd.Flags().StringVarP(&project, "project", "p", "", "只起指定 project 的 dev 进程（manifest 里的 name 或相对路径）")
	i18n.MarkShort(cmd, "dev.short")
	return cmd
}

// resolveProcessSelector turns the user-facing -p value into a manifest
// project name (which equals the Procfile.dev entry name). Empty input
// yields empty output, meaning "all processes".
func resolveProcessSelector(projectRoot, selector string) (string, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return "", nil
	}
	sub, err := workspace.ResolveProjectFromSelector(projectRoot, selector)
	if err != nil {
		return "", err
	}
	if sub == nil {
		m, _ := workspace.ReadManifest(projectRoot)
		return "", cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND,
			fmt.Sprintf("没有名为 %s 的 project", selector)).
			WithContext(map[string]any{
				"selector":           selector,
				"available_projects": workspace.ProjectNames(m),
			})
	}
	return sub.Name, nil
}
