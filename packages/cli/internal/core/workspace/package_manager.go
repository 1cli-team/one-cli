package workspace

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// ResolvePackageManager shares one choice between dependency preparation and
// operation execution: workspace declaration, project declaration, lockfile,
// then the default used by newly created workspaces.
func ResolvePackageManager(root, fallback string) (string, error) {
	manager := strings.TrimSpace(fallback)
	pkg, err := ReadPackageJSON(root)
	if err != nil {
		return "", err
	}
	if pkg != nil && strings.TrimSpace(pkg.PackageManager) != "" {
		manager = strings.TrimSpace(pkg.PackageManager)
	}
	manager, _, _ = strings.Cut(manager, "@")
	if manager == "" {
		for _, item := range []struct{ file, manager string }{{"pnpm-lock.yaml", "pnpm"}, {"bun.lock", "bun"}, {"bun.lockb", "bun"}, {"yarn.lock", "yarn"}, {"package-lock.json", "npm"}} {
			if _, err := os.Stat(filepath.Join(root, item.file)); err == nil {
				manager = item.manager
				break
			} else if !os.IsNotExist(err) {
				return "", err
			}
		}
	}
	if manager == "" {
		manager = "pnpm"
	}
	switch manager {
	case "pnpm", "npm", "yarn", "bun":
		return manager, nil
	default:
		return "", i18n.Errorf("workspace.package_manager_unsupported", manager)
	}
}
