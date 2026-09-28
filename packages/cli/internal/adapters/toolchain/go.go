package adapters

import (
	"github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"
)

// goAdapter implements toolchain.Adapter for Go subprojects.
type goAdapter struct{}

func (goAdapter) ID() toolchain.Toolchain  { return toolchain.Go }
func (goAdapter) UsesPackageManager() bool { return false }

func (goAdapter) InstallPlan(_ toolchain.PlanInput) toolchain.CommandStep {
	return toolchain.CommandStep{
		Kind:    "install",
		Command: "go",
		Args:    []string{"mod", "tidy"},
	}
}

// PackageManagerForManifest returns empty for Go: the manifest field is
// omitted because Go subprojects have no package manager.
func (goAdapter) PackageManagerForManifest(_ toolchain.PackageManager) toolchain.PackageManager {
	return ""
}
