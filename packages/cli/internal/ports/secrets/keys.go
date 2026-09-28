package secrets

import (
	"regexp"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// secretKeyRE restricts names to portable environment-variable identifiers.
// Infisical values are injected into child processes using these names.
var secretKeyRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// AssertValidKey rejects secret names that don't match the POSIX env-var
// pattern. Returns ENV_INVALID_KEY on bad input; the message includes a
// concrete example so users can fix their input without consulting docs.
func AssertValidKey(s string) error {
	if !secretKeyRE.MatchString(s) {
		return cliErrors.New(cliErrors.ENV_INVALID_KEY,
			i18n.Tf("env.key_naming", s))
	}
	return nil
}
