package infisical

// loader.go registers Infisical as a secrets.Loader so `one run` can
// dispatch through the registry rather than hardcoding "is Infisical
// configured? then use it; else fall back to dotenv". The same pattern
// is followed by every future provider (doppler, vault, ...).

import (
	"context"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets"
)

type runLoader struct{}

func (runLoader) ID() string { return "infisical" }

// Priority is "remote backend": configured provider wins over
// filesystem fallback in --from auto. Future remote providers
// (Doppler / Vault) should also use PriorityRemoteBackend with
// distinct IDs; they're mutually exclusive at the manifest level
// so only one returns true from Available() at a time.
func (runLoader) Priority() secrets.Priority { return secrets.PriorityRemoteBackend }

// Available is the gate for --from auto: Infisical must be both
// configured in the workspace manifest AND have credentials available
// (env vars OR a default env profile). We avoid a network probe here
// — it's a cheap pre-flight, not a healthcheck. If creds are stale,
// the actual Load() call will surface the auth error.
func (runLoader) Available(projectRoot string) bool {
	cfg, err := LoadWorkspaceConfig(projectRoot)
	if err != nil || cfg == nil {
		return false
	}
	// projectId is required even when creds come from a profile —
	// project-level fields stay in the manifest.
	if strings.TrimSpace(cfg.ProjectID) == "" {
		return false
	}
	return runCredsAvailable(projectRoot)
}

// Load delegates to FetchSecretsForSubproject — same code path the
// previous hardcoded `--from remote` branch used.
func (runLoader) Load(ctx context.Context, projectRoot, relativeDir, envName string) (map[string]string, error) {
	return FetchSecretsForSubproject(ctx, projectRoot, relativeDir, envName)
}

func init() {
	secrets.Register(runLoader{})
}
