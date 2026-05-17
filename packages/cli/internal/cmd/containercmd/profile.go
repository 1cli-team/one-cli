package containercmd

// profile.go is the dispatch surface between containercmd and the
// docker package's per-kind ResolveRegistry. The kind is read off the
// manifest (`workspace.ContainerKindForProject`) with a workspace-
// level fallback chain. Build accepts a nil registry (local-only
// build) when no profile is configured; Push hard-requires one.

import (
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/container"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/docker"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// containerKindForInvocation picks the container kind for one
// containercmd invocation. When the user passed -p / a positional,
// the per-project pin wins; otherwise the workspace-level default is
// used. Empty defaults to "docker".
func containerKindForInvocation(projectRoot, subproject string) string {
	m, err := workspace.ReadManifest(projectRoot)
	if err != nil {
		return "docker"
	}
	return workspace.ContainerKindForProject(m, subproject)
}

// resolveContainerRegistry walks the profile resolution chain for the
// given (kind, subproject) pair and returns the container.Registry
// endpoint that Build / Push need to compose registry-prefixed tags.
// Push callers expect REGISTRY_CREDENTIAL_MISSING when no profile is
// set; Build callers use resolveBuildContainerRegistry which tolerates
// a nil result.
func resolveContainerRegistry(projectRoot, profileFlag, kind, subproject string) (*container.Registry, error) {
	return docker.ResolveRegistry(docker.ResolveRegistryInput{
		ProjectRoot:     projectRoot,
		Kind:            kind,
		ProfileFlag:     profileFlag,
		Subproject:      subproject,
		RequireRegistry: true,
	})
}

// resolveBuildContainerRegistry is the Build-time variant: when no
// profile pin exists at flag-, project-, or workspace-level, it
// returns nil so Build falls back to a local-only `<workload>:<tag>`
// image with no registry prefix and no docker login.
func resolveBuildContainerRegistry(projectRoot, profileFlag, kind, subproject string) (*container.Registry, error) {
	return docker.ResolveRegistry(docker.ResolveRegistryInput{
		ProjectRoot:     projectRoot,
		Kind:            kind,
		ProfileFlag:     profileFlag,
		Subproject:      subproject,
		RequireRegistry: false,
		SkipDefault:     true,
	})
}
