package secrets

import (
	"fmt"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// ResolveEnvName picks the effective environment name for a verb
// invocation against a workspace's manifest. Precedence:
//
//	flag → manifest.environments.default → manifest.environments.names[0] → ""
//
// allowUnknown=true skips the "not in environments list" check (used by
// the set verb, which can implicitly create new environments, and by deploy
// when the user passes --secrets-env for an env that hasn't been declared
// yet). Returns the chosen name plus the declared list (for help text /
// error context).
func ResolveEnvName(projectRoot, flag string, allowUnknown bool) (string, []string, error) {
	m, err := workspace.ReadManifest(projectRoot)
	if err != nil {
		return "", nil, err
	}
	declared := []string{}
	defaultEnv := ""
	if m.Environments != nil {
		declared = append(declared, m.Environments.Names...)
		defaultEnv = strings.TrimSpace(m.Environments.Default)
	}

	chosen := strings.TrimSpace(flag)
	if chosen == "" {
		chosen = defaultEnv
	}
	if chosen == "" && len(declared) > 0 {
		chosen = declared[0]
	}
	if chosen == "" {
		return "", declared, nil
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
		fmt.Sprintf("环境 %q 未在 manifest.environments.names 中（已声明：%s）。",
			chosen, strings.Join(declared, ", "))).
		WithContext(map[string]any{
			"requested":    chosen,
			"environments": declared,
		})
}
