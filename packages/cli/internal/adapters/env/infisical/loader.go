package infisical

// loader.go adapts Infisical to secrets.Loader so an explicit composition
// root can include it without package-initialization side effects.

import (
	"context"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

type runLoader struct{}

func (runLoader) ID() string { return "infisical" }

func (runLoader) Load(ctx context.Context, projectRoot, relativeDir, envName string) (map[string]string, error) {
	return FetchSecretsForSubproject(ctx, projectRoot, relativeDir, envName)
}

// Loader constructs the Infisical adapter for an explicit composition root.
func Loader() secrets.Loader { return runLoader{} }
