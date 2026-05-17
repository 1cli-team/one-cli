// Package infra orchestrates post-add side-effects on workspace-level
// infra files: subproject Dockerfile, k8s deployment workloads,
// Procfile.dev entries, and .gitignore lines for env/dotenv. Each
// backend exposes a package-level Sync function that this package
// dispatches to via inline switch on the bare backend name (e.g.
// "docker" / "kustomize" / "aliyun-oss" / "cloudflare" / ...).
package infra

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/pkg/toolchain"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/cloudflare"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/docker"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/edgeone"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/internalcommon"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/kustomize"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/vercel"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets/dotenv"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets/infisical"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// Options bundles every input the infra sync needs.
type Options struct {
	ProjectRoot    string
	TargetDir      string
	ProjectName    string
	TemplateID     string
	Toolchain      toolchain.Toolchain
	PackageManager toolchain.PackageManager
	// Selected names which backend to run per polymorphic domain, keyed
	// by domain (string form), e.g. {"container": "container/docker",
	// "deploy": "deploy/kustomize", "env": "env/infisical"}. Empty
	// domain entries are skipped. The current manifest omits ci / dev from this map; the
	// dispatcher synchronises Procfile.dev unconditionally and the
	// caller invokes ci.Sync separately.
	Selected map[string]string
	// Env identifies which environment is being targeted (dev / test /
	// prod / ""). Empty = env-agnostic, the current default.
	Env string
}

// SyncSubproject runs each selected backend's Sync against opts. Order
// is fixed (container → dev → deploy → ci → env) so downstream backends
// see artefacts produced upstream.
func SyncSubproject(opts Options) error {
	tc := opts.Toolchain
	if tc == "" {
		tc = toolchain.Node
	}
	pm := opts.PackageManager
	if pm == "" && tc == toolchain.Node {
		pm = toolchain.PMpnpm
	}

	adapter := toolchain.Get(tc)
	scripts, err := loadScripts(opts.TargetDir)
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
	workloadName := internalcommon.ResolveWorkloadName(opts.ProjectName, opts.TargetDir)

	// Container: write Dockerfile if selected and not already present.
	if id := opts.Selected["container"]; id != "" {
		if profile.IsContainerKind(backend(id)) {
			if docker.ShouldSync(opts.TargetDir, adapter) {
				if err := docker.Sync(opts.TargetDir, adapter, pm, runtime); err != nil {
					return err
				}
			}
		}
	}

	// Dev: derive the dev command from the just-scaffolded package.json
	// (or toolchain default) and persist it to the manifest at
	// projects[].domains.dev.command. `one dev` reads the manifest at
	// startup — no Procfile.dev artefact is produced.
	if cmd := workspace.ResolveDevCommand(scripts, string(tc)); cmd != "" {
		if err := workspace.UpdateProjectDev(opts.ProjectRoot, relDir, cmd); err != nil {
			return err
		}
	}

	// Deploy: scaffold backend-specific config where the platform CLI
	// benefits from a non-interactive first run. The S3-compatible
	// backends have no on-disk artefact, deploy/vercel writes
	// vercel.json, and deploy/cloudflare writes wrangler.toml.
	if id := opts.Selected["deploy"]; id != "" {
		kind := backend(id)
		switch {
		case kind == "kustomize":
			if err := kustomize.Sync(opts.ProjectRoot, workloadName, runtime.ContainerPort); err != nil {
				return err
			}
		case workspace.IsS3CompatibleDeploy(kind):
			// no-op; S3-compatible deploy backends have no sync-time artefact
		case kind == "vercel":
			if vercel.ShouldSync(opts.TargetDir) {
				if err := vercel.Sync(opts.TargetDir, opts.TemplateID); err != nil {
					return err
				}
			}
		case kind == "cloudflare":
			if cloudflare.ShouldSync(opts.TargetDir) {
				if err := cloudflare.Sync(opts.TargetDir, opts.TemplateID, workloadName); err != nil {
					return err
				}
			}
		case kind == "edgeone":
			if edgeone.ShouldSync(opts.TargetDir) {
				if err := edgeone.Sync(opts.TargetDir, opts.TemplateID, workloadName); err != nil {
					return err
				}
			}
		}
	}

	// Env: workspace-scoped — runs once per workspace, not per subproject.
	// Caller (sync.AllSubprojects) usually invokes this separately, but
	// being idempotent we can run it again here when scoped via opts.
	if id := opts.Selected["env"]; id != "" {
		switch backend(id) {
		case "dotenv":
			if err := dotenv.Sync(opts.ProjectRoot); err != nil {
				return err
			}
		case "infisical":
			if err := infisical.Sync(); err != nil {
				return err
			}
		}
	}

	return nil
}

// backend extracts the bare backend name from a namespaced id
// ("container/docker" → "docker"). Inputs without a slash pass through
// unchanged.
func backend(id string) string {
	idx := strings.IndexByte(id, '/')
	if idx < 0 || idx == len(id)-1 {
		return id
	}
	return id[idx+1:]
}

// ResolveWorkloadName re-exports the kebab-cased workload name resolver
// for the few external callers that need it (sync package, status checks).
func ResolveWorkloadName(projectName, targetDir string) string {
	return internalcommon.ResolveWorkloadName(projectName, targetDir)
}

// loadScripts parses package.json#scripts and returns the keys+values.
// Empty for Go subprojects (no package.json).
func loadScripts(targetDir string) (map[string]string, error) {
	pkgPath := filepath.Join(targetDir, "package.json")
	raw, err := os.ReadFile(pkgPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	type pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	var p pkg
	if err := json.Unmarshal(raw, &p); err != nil {
		return map[string]string{}, nil
	}
	if p.Scripts == nil {
		p.Scripts = map[string]string{}
	}
	return p.Scripts, nil
}
