package bootstrap

import (
	miseruntime "github.com/torchstellar-team/one-cli/packages/one-core/internal/adapters/runtime/mise"
	catalog "github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/backend"
	creationmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/creation"
	skillsmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/skills"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/prompt"
	runtimeport "github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/runtime"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/secrets"
)

// Dependencies contains local execution capabilities only. Management clients
// are assembled by the CLI and injected through narrow execution ports.
type Dependencies struct {
	Runtime  runtimeport.Provider
	Catalog  *catalog.Catalog
	Creation *creationmodule.Service
	Loaders  *secrets.Registry
}

func Compose(observe creationmodule.WorkspaceObserver, loaders ...secrets.Loader) Dependencies {
	creation, err := creationmodule.NewService(observe)
	if err != nil {
		panic(err)
	}
	creation.Runtime = miseruntime.Provider{}
	creation.Skills = skillsmodule.Installer{Run: (skillsmodule.Runner{Runtime: creation.Runtime}).Run, Progress: prompt.ReportProgress}
	return Dependencies{Runtime: miseruntime.Provider{}, Catalog: catalog.Builtin(), Creation: creation, Loaders: secrets.MustRegistry(loaders...)}
}
