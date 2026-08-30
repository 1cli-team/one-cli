package githubactions

import (
	pkgci "github.com/torchstellar-team/one-cli/packages/cli/pkg/ci"
	"github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"
)

// workflowInputFor adapts pkg/ci.Input to the toolchain.WorkflowInput
// the existing adapter.RenderWorkflow expects. Kept here so a future
// rework of the WorkflowInput shape stays scoped to GitHub Actions.
func workflowInputFor(in pkgci.Input) toolchain.WorkflowInput {
	pm := in.PackageManager
	if pm == "" && in.Toolchain == toolchain.Node {
		pm = toolchain.PMpnpm
	}
	return toolchain.WorkflowInput{
		ProjectName:      in.ProjectName,
		RelativeDir:      in.RelativeDir,
		WorkflowFilePath: in.WorkflowFilePath,
		PackageManager:   pm,
		Scripts:          in.Scripts,
	}
}
