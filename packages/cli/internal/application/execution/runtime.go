package execution

import (
	"os"
	"path/filepath"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

// RuntimeKind keeps normal command syntax unchanged. Legacy workspaces retain
// their original execution path until they have explicitly generated mise config.
// ONE_RUNTIME is an escape hatch for diagnostics and compatibility tests.
func RuntimeKind(root string) (string, error) {
	if override := os.Getenv("ONE_RUNTIME"); override != "" {
		if err := runtimeport.Validate(override); err != nil {
			return "", err
		}
		return override, nil
	}
	_, err := os.Stat(filepath.Join(root, workspace.MiseConfigFilename))
	if os.IsNotExist(err) {
		return runtimeport.Builtin, nil
	}
	if err != nil {
		return "", err
	}
	return runtimeport.Mise, nil
}
