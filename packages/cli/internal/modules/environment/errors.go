package environment

import (
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

func unsupportedVerb(backend, verb string) error {
	return cliErrors.New(cliErrors.BACKEND_VERB_NOT_SUPPORTED,
		i18n.Tf("env.backend_operation_unsupported", backend, verb)).
		WithContext(map[string]any{
			"domain": "env", "backend": backend, "verb": verb,
		})
}
