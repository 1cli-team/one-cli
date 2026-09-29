package secrets

import (
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// ResolveEnvName selects --env or the fixed dev default. The declared slugs
// come from env.infisical.environments. Set may use an undeclared slug, which
// is registered locally only after the remote write succeeds.
func ResolveEnvName(projectRoot, flag string, allowUnknown bool) (string, []string, error) {
	m, err := workspace.ReadManifest(projectRoot)
	if err != nil {
		return "", nil, err
	}
	declared := workspace.EnvironmentNames(m)
	chosen := strings.TrimSpace(flag)
	if chosen == "" {
		chosen = "dev"
	}
	if allowUnknown {
		return chosen, declared, nil
	}
	for _, e := range declared {
		if e == chosen {
			return chosen, declared, nil
		}
	}
	if len(declared) == 0 {
		return chosen, declared, nil
	}
	return "", declared, cliErrors.New(cliErrors.ENV_UNKNOWN_ENVIRONMENT,
		i18n.Tf("env.environment_unknown",
			chosen, strings.Join(declared, ", "))).
		WithContext(map[string]any{
			"requested":    chosen,
			"environments": declared,
		})
}
