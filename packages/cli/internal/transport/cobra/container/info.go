package containercmd

// info.go: `one container info` subcommand. Dispatches to the workspace-
// default container backend through the compiled container module. Info is
// shared across all four OCI registry kinds.

import (
	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/container"
	containermodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/container"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

func newInfoCmd(service *containermodule.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "列出工作区里容器构建的相关元数据（无副作用）",
		RunE: func(cmd *cobra.Command, _ []string) error {
			activeWorkspace, err := execution.ResolveWorkspace(cmd.Context())
			if err != nil {
				return err
			}
			root := activeWorkspace.Root()
			names, err := containerSubprojects(activeWorkspace.Manifest())
			if err != nil {
				return err
			}
			kind := containerKindForInvocation(activeWorkspace.Manifest(), "")
			res, err := service.Info(cmd.Context(), kind, container.InfoInput{
				ProjectRoot: root,
				TargetNames: names,
			})
			if err != nil {
				return err
			}
			if res != nil {
				output.Emit(res)
			}
			return nil
		},
	}
	i18n.MarkShort(cmd, "container.info.short")
	return cmd
}
