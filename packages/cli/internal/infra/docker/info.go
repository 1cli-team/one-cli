package docker

// info.go: enumerates each project's Dockerfile presence + resolved
// workload name. Pure read-only. Shared `containerOverride` /
// `targetNameSet` helpers live here too because every other verb
// (build / push) consumes them.

import (
	"os"
	"path/filepath"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/container"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/internalcommon"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// Info enumerates each project's Dockerfile presence and resolved
// workload name. Read-only — no side effects.
func Info(in container.InfoInput) (*container.InfoResult, error) {
	m, err := workspace.ReadManifest(in.ProjectRoot)
	if err != nil {
		return nil, err
	}
	targets := targetNameSet(in.TargetNames)
	subs := make([]container.ProjectInfo, 0, len(m.Projects))
	for _, s := range m.Projects {
		if len(targets) > 0 {
			if _, ok := targets[s.Name]; !ok {
				continue
			}
		}
		dockerfile := filepath.Join(in.ProjectRoot, filepath.FromSlash(s.RelativeDir), "Dockerfile")
		_, statErr := os.Stat(dockerfile)
		info := container.ProjectInfo{
			Name:         s.Name,
			RelativeDir:  s.RelativeDir,
			Backend:      backendName,
			HasArtifact:  statErr == nil,
			ArtifactPath: dockerfile,
			WorkloadName: internalcommon.ResolveWorkloadName(s.Name, filepath.Join(in.ProjectRoot, filepath.FromSlash(s.RelativeDir))),
		}
		if c := containerOverride(s); c != nil {
			info.ImageOverride = c.Image
		}
		subs = append(subs, info)
	}
	return &container.InfoResult{
		Schema:           SchemaInfo,
		Workspace:        in.ProjectRoot,
		ContainerBackend: backendName,
		Projects:         subs,
	}, nil
}

// containerOverride returns the current per-project container override, or
// nil when the project has none. Local helper so call sites read like
// the old `s.Container` access.
func containerOverride(s workspace.ManifestProject) *workspace.ProjectContainerOverride {
	if s.Domains == nil {
		return nil
	}
	return s.Domains.Container
}

func targetNameSet(names []string) map[string]struct{} {
	if len(names) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(names))
	for _, name := range names {
		out[name] = struct{}{}
	}
	return out
}
