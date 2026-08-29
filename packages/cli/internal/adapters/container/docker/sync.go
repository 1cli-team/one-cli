package docker

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"
)

// ShouldSync reports whether a Sync call would do work for the given
// subproject. Requires a toolchain Adapter and that no Dockerfile is
// already present (we never overwrite).
func ShouldSync(targetDir string, adapter toolchain.Adapter) bool {
	if adapter == nil {
		return false
	}
	_, err := os.Stat(filepath.Join(targetDir, "Dockerfile"))
	return errors.Is(err, fs.ErrNotExist)
}

// Sync renders the Dockerfile via the toolchain adapter and writes it
// into the subproject directory. Caller should gate on ShouldSync to
// avoid clobbering hand-edited Dockerfiles.
func Sync(targetDir string, adapter toolchain.Adapter, pm toolchain.PackageManager, runtime toolchain.RuntimeResolution) error {
	content := adapter.RenderDockerfile(toolchain.DockerfileInput{
		PackageManager: pm,
		Runtime:        runtime,
	})
	path := filepath.Join(targetDir, "Dockerfile")
	return os.WriteFile(path, []byte(content), 0o644)
}
