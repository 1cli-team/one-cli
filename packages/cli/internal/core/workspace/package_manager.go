package workspace

import (
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"os"
	"path/filepath"
	"strings"
)

// ResolvePackageManager validates both project and workspace declarations.
// One manages pnpm projects; it never converts other package managers implicitly.
func ResolvePackageManager(root, fallback string) (string, error) {
	validate := func(value string) error {
		name, _, _ := strings.Cut(strings.TrimSpace(value), "@")
		if name != "" && name != "pnpm" {
			return i18n.Errorf("workspace.package_manager_unsupported", name)
		}
		return nil
	}
	if err := validate(fallback); err != nil {
		return "", err
	}
	pkg, err := ReadPackageJSON(root)
	if err != nil {
		return "", err
	}
	if pkg != nil {
		if err := validate(pkg.PackageManager); err != nil {
			return "", err
		}
	}
	if (pkg == nil || pkg.PackageManager == "") && fallback == "" {
		for _, name := range []string{"package-lock.json", "npm-shrinkwrap.json", "yarn.lock", "bun.lock", "bun.lockb"} {
			if _, err := os.Stat(filepath.Join(root, name)); err == nil {
				return "", i18n.Errorf("workspace.package_manager_unsupported", name)
			} else if !os.IsNotExist(err) {
				return "", err
			}
		}
	}
	return "pnpm", nil
}
