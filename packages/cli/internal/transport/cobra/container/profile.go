package containercmd

// profile.go maps container command inputs into the compiled module's registry
// resolver. The kind is read off the
// manifest (`workspace.ContainerKindForProject`) with a workspace-
// level fallback chain. Build accepts a nil registry (local-only
// build) when no profile is configured; Push hard-requires one.

import (
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/container"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	containermodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/container"
)

// containerKindForInvocation picks the container kind for one
// containercmd invocation. When the user passed -p / a positional,
// the per-project pin wins; otherwise the workspace-level default is
// used. Empty defaults to "docker".
func containerKindForInvocation(manifest *workspace.Manifest, subproject string) string {
	return workspace.ContainerKindForProject(manifest, subproject)
}

// resolveContainerRegistry walks the profile resolution chain for the
// given (kind, subproject) pair and returns the container.Registry
// endpoint that Build / Push need to compose registry-prefixed tags.
// Push callers expect REGISTRY_CREDENTIAL_MISSING when no profile is
// set; Build callers use resolveBuildContainerRegistry which tolerates
// a nil result.
func resolveContainerRegistry(deps Dependencies, projectRoot, profileFlag, kind, subproject string) (*container.Registry, error) {
	return deps.Service.ResolveRegistry(containermodule.ResolveRegistryInput{
		ProjectRoot:     projectRoot,
		Backend:         kind,
		Profile:         profileFlag,
		Project:         subproject,
		RequireRegistry: true,
	})
}

// resolveBuildContainerRegistry is the Build-time variant: when no
// profile pin exists at flag-, project-, or workspace-level, it
// returns nil so Build falls back to a local-only `<workload>:<tag>`
// image with no registry prefix and no docker login.
func resolveBuildContainerRegistry(deps Dependencies, projectRoot, profileFlag, kind, subproject string) (*container.Registry, error) {
	return deps.Service.ResolveRegistry(containermodule.ResolveRegistryInput{
		ProjectRoot:     projectRoot,
		Backend:         kind,
		Profile:         profileFlag,
		Project:         subproject,
		RequireRegistry: false,
		SkipDefault:     true,
	})
}
