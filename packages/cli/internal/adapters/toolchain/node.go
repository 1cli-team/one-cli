// Package adapters bundles One CLI's built-in toolchain adapters. The
// composition root calls RegisterBundled explicitly; importing this package
// has no side effects. Consumers use pkg/toolchain for the public contract.
package adapters

import (
	"github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"
)

// nodeAdapter implements toolchain.Adapter for Node.js subprojects (the
// default).
type nodeAdapter struct{}

func (nodeAdapter) ID() toolchain.Toolchain  { return toolchain.Node }
func (nodeAdapter) UsesPackageManager() bool { return true }

func (nodeAdapter) InstallPlan(in toolchain.PlanInput) toolchain.CommandStep {
	pm := toolchain.PMpnpm
	return toolchain.CommandStep{
		Kind:    "install",
		Command: string(pm),
		Args:    []string{"install"},
	}
}

// PackageManagerForManifest just echoes the value back for Node — every
// Node template has a meaningful package manager. Go's adapter returns "" so
// the manifest omits the field for go subprojects.
func (nodeAdapter) PackageManagerForManifest(pm toolchain.PackageManager) toolchain.PackageManager {
	if pm == "" {
		return toolchain.PMpnpm
	}
	return toolchain.PMpnpm
}
