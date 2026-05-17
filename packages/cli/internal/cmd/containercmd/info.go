package containercmd

// info.go: `one container info` subcommand. Dispatches to the workspace-
// default container kind's Provider via container.Get — the Info
// implementation is shared across all four kinds (it only enumerates
// Dockerfile presence), but routing through the registry keeps the
// dispatch shape consistent with build / push.

import (
	"context"

	"github.com/spf13/cobra"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/container"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func newInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "列出工作区里容器构建的相关元数据（无副作用）",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := workspace.ResolveProjectRoot("")
			if err != nil {
				return err
			}
			names, err := containerSubprojects(root)
			if err != nil {
				return err
			}
			kind := containerKindForInvocation(root, "")
			provider, ok := container.Get(kind)
			if !ok {
				return cliErrors.New(cliErrors.CONTAINER_KIND_UNKNOWN,
					"container kind "+kind+" 未注册；检查 `projects[i].domains.container.kind` 值。")
			}
			res, err := provider.Info(context.Background(), container.InfoInput{
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
	return cmd
}
