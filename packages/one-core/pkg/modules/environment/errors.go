package environment

import (
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func unsupportedVerb(backend, verb string) error {
	return cliErrors.New(cliErrors.BACKEND_VERB_NOT_SUPPORTED,
		i18n.Tf("env.backend_operation_unsupported", backend, verb)).
		WithContext(map[string]any{
			"domain": "env", "backend": backend, "verb": verb,
		})
}
