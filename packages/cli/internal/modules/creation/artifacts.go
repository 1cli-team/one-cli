package creation

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/container/docker"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/deploy/cloudflare"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/deploy/edgeone"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/deploy/kustomize"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/deploy/vercel"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/dotenv"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"
)

// syncProjectOptions is the fully resolved local-artifact input produced by
// materializeProject. It is private because no other workflow may assemble backend
// sync order independently.
type syncProjectOptions struct {
	ProjectRoot    string
	TargetDir      string
	ProjectName    string
	TemplateID     string
	Toolchain      toolchain.Toolchain
	PackageManager toolchain.PackageManager
	Selected       map[string]string
}

// syncProject materialises compiled-in project artifacts in dependency order:
// container, dev command, deploy configuration, then environment safety rules.
func syncProject(opts syncProjectOptions) error {
	tc := opts.Toolchain
	if tc == "" {
		tc = toolchain.Node
	}
	pm := opts.PackageManager
	if pm == "" && tc == toolchain.Node {
		pm = toolchain.PMpnpm
	}

	adapter := toolchain.Get(tc)
	scripts, err := loadProjectScripts(opts.TargetDir)
	if err != nil {
		return err
	}
	runtime := adapter.ResolveRuntime(toolchain.PlanInput{
		Scripts:        scripts,
		PackageManager: pm,
		TemplateID:     opts.TemplateID,
	})

	relDir, err := filepath.Rel(opts.ProjectRoot, opts.TargetDir)
	if err != nil {
		return err
	}
	relDir = filepath.ToSlash(relDir)
	workloadName := workspace.ResolveWorkloadName(opts.ProjectName, opts.TargetDir)

	if id := opts.Selected["container"]; id != "" && profile.IsContainerKind(backendName(id)) {
		if docker.ShouldSync(opts.TargetDir, adapter) {
			if err := docker.Sync(opts.TargetDir, adapter, pm, runtime); err != nil {
				return err
			}
		}
	}

	if command := workspace.ResolveScaffoldDevCommand(scripts, string(tc), opts.TargetDir); command != "" {
		if err := workspace.UpdateProjectDev(opts.ProjectRoot, relDir, command); err != nil {
			return err
		}
	}

	if id := opts.Selected["deploy"]; id != "" {
		backend := backendName(id)
		switch {
		case backend == "kustomize":
			if err := kustomize.Sync(opts.ProjectRoot, workloadName, runtime.ContainerPort); err != nil {
				return err
			}
		case workspace.IsS3CompatibleDeploy(backend):
			// S3-compatible backends have no sync-time artifact.
		case backend == "vercel":
			if vercel.ShouldSync(opts.TargetDir) {
				if err := vercel.Sync(opts.TargetDir, opts.TemplateID); err != nil {
					return err
				}
			}
		case backend == "cloudflare":
			if cloudflare.ShouldSync(opts.TargetDir) {
				if err := cloudflare.Sync(opts.TargetDir, opts.TemplateID, workloadName); err != nil {
					return err
				}
			}
		case backend == "edgeone":
			if edgeone.ShouldSync(opts.TargetDir) {
				if err := edgeone.Sync(opts.TargetDir, opts.TemplateID, workloadName); err != nil {
					return err
				}
			}
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
