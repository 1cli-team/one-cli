// Package deploycmd contributes `one deploy` to the explicit CLI composition
// root. Verbs iterate per-project deploy targets — each
// subproject's deploy backend is configured in the manifest, so a
// workspace can mix front-end (s3 / vercel) and back-end (kustomize)
// deployments in one command.
//
// Profile support: each verb takes --profile to one-shot override the
// default profile. The application deployment workflow resolves profiles,
// injects environment values, runs pre-deploy builds, and dispatches providers;
// Cobra supplies interactive fallbacks and renders progress/results.
//
// Per-workspace and per-project profile choices live in
// ~/.config/one/config.json#workspaces. --profile overrides at runtime;
// otherwise resolution falls through to workspace bindings and then the
// machine default profile.
package deploycmd

import (
	"github.com/spf13/cobra"

	configureapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/configure"
	deploymentapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/deployment"
	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	creationmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/creation"
)

type Dependencies struct {
	Catalog    *catalog.Catalog
	Profiles   *configureapp.ProfileService
	Creation   *creationmodule.Service
	NewService func(buildVersion string) *deploymentapp.Service
}

func Commands(deps Dependencies) []*cobra.Command {
	return []*cobra.Command{newDeployCmd(deps)}
}
