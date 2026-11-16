package secrets

import (
	"regexp"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
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

var envNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-_]*$`)

func ValidateEnvName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !envNamePattern.MatchString(value) {
		return "", cliErrors.New(cliErrors.ENV_INVALID_ENV_NAME, i18n.Tf("env.environment_format", value))
	}
	return value, nil
}
