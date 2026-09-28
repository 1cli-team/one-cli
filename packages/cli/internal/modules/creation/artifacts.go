package creation

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/dotenv"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"
)

// syncProjectOptions is the fully resolved local-artifact input produced by
// materializeProject. It is private because no other workflow may assemble backend
// sync order independently.
type syncProjectOptions struct {
	ProjectRoot    string
	TargetDir      string
	Toolchain      toolchain.Toolchain
	PackageManager toolchain.PackageManager
	Selected       map[string]string
}

// syncProject writes the development command and environment safety rules.
func syncProject(opts syncProjectOptions) error {
	tc := opts.Toolchain
	if tc == "" {
		tc = toolchain.Node
	}

	scripts, err := loadProjectScripts(opts.TargetDir)
	if err != nil {
		return err
	}
	relDir, err := filepath.Rel(opts.ProjectRoot, opts.TargetDir)
	if err != nil {
		return err
	}
	relDir = filepath.ToSlash(relDir)
	if command := workspace.ResolveScaffoldDevCommand(scripts, string(tc), opts.TargetDir); command != "" {
		if err := workspace.UpdateProjectDev(opts.ProjectRoot, relDir, command); err != nil {
			return err
		}
	}

	if id := opts.Selected["env"]; id != "" {
		switch backendName(id) {
		case workspace.EnvBackendDotenv, workspace.EnvBackendInfisical:
			if err := dotenv.Sync(opts.ProjectRoot); err != nil {
				return err
			}
		}
	}

	return nil
}

func backendName(id string) string {
	index := strings.IndexByte(id, '/')
	if index < 0 || index == len(id)-1 {
		return id
	}
	return id[index+1:]
}

func loadProjectScripts(targetDir string) (map[string]string, error) {
	raw, err := os.ReadFile(filepath.Join(targetDir, "package.json"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	var value struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return map[string]string{}, nil
	}
	if value.Scripts == nil {
		value.Scripts = map[string]string{}
	}
	return value.Scripts, nil
}
