package cicmd

import (
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

func resolveSelector(args []string, legacy string) (string, error) {
	positional := ""
	if len(args) > 0 {
		positional = strings.TrimSpace(args[0])
	}
	legacy = strings.TrimSpace(legacy)
	if positional != "" && legacy != "" && positional != legacy {
		return "", cliErrors.New(
			cliErrors.ONE_CLI_ERROR,
			i18n.T("ci.error.project_conflict"),
		).WithContext(map[string]any{
			"positional_project": positional, "flag_project": legacy,
		})
	}
	if positional != "" {
		return positional, nil
	}
	return legacy, nil
}
