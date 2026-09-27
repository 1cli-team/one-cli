// Package adapters bundles One CLI's built-in toolchain adapters. The
// composition root calls RegisterBundled explicitly; importing this package
// has no side effects. Consumers use pkg/toolchain for the public contract.
package adapters

import (
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"
)

// nodeAdapter implements toolchain.Adapter for Node.js subprojects (the
// default).
type nodeAdapter struct{}

func (nodeAdapter) ID() toolchain.Toolchain  { return toolchain.Node }
func (nodeAdapter) UsesPackageManager() bool { return true }

func (nodeAdapter) InstallPlan(in toolchain.PlanInput) toolchain.CommandStep {
	pm := resolvePackageManager(in.PackageManager)
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
	return pm
}

func (nodeAdapter) RenderWorkflow(in toolchain.WorkflowInput) string {
	pm := resolvePackageManager(in.PackageManager)
	lockfile := resolveLockfileByPM(pm)
	installCmd := resolveNodeInstallCommand(pm, true)
	ciCmds := resolveNodeCiCommands(in.Scripts, pm)

	lines := workflowHeader(in.ProjectName, in.RelativeDir, in.WorkflowFilePath)

	if pm == toolchain.PMpnpm {
		lines = append(lines,
			"      - uses: pnpm/action-setup@v6",
			"        with:",
			"          version: 12.3.4",
		)
	}

	lines = append(lines,
		"      - uses: actions/setup-node@v7",
		"        with:",
		"          node-version: 24",
		"          cache: "+string(pm),
		"          cache-dependency-path: ./"+in.RelativeDir+"/"+lockfile,
		"      - name: Install dependencies",
		"        run: "+installCmd,
	)
	for _, cmd := range ciCmds {
		lines = append(lines,
			"      - name: Run "+cmd,
			"        run: "+cmd,
		)
	}
	return strings.Join(lines, "\n") + "\n"
}
