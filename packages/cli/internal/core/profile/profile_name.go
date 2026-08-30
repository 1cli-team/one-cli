package profile

import (
	"fmt"
	"regexp"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

const maxProfileNameLength = 128

var profileNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// ValidateName enforces the single profile-name contract used by persistent
// profile definitions, workspace bindings, and the token cache. Keeping the
// value to one portable path segment prevents a Dashboard-supplied name from
// escaping ~/.config/one/cache when it is later used as a cache filename.
func ValidateName(name string) error {
	if name == "" || name != strings.TrimSpace(name) || len(name) > maxProfileNameLength ||
		!profileNameRE.MatchString(name) {
		return cliErrors.New(
			cliErrors.PROFILE_BACKEND_INVALID,
			fmt.Sprintf(
				"profile 名 %q 不合法；必须匹配 %s，且长度不超过 %d。",
				name,
				profileNameRE.String(),
				maxProfileNameLength,
			),
		).WithContext(map[string]any{
			"field":   "profile name",
			"value":   name,
			"pattern": profileNameRE.String(),
			"max":     maxProfileNameLength,
		})
	}
	return nil
}
