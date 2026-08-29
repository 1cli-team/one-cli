package environment

import (
	"fmt"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

func unsupportedVerb(backend, verb string) error {
	return cliErrors.New(cliErrors.BACKEND_VERB_NOT_SUPPORTED,
		fmt.Sprintf("env/%s 后端不支持 `one env %s`。", backend, verb)).
		WithContext(map[string]any{
			"domain": "env", "backend": backend, "verb": verb,
		})
}
