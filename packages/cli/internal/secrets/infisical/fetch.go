package infisical

import (
	"context"
	"path/filepath"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// FetchSecretsForSubproject pulls every secret a subproject can see from
// Infisical (along its inheritance chain) and returns the merged map. No
// disk IO. Used by `one run` so users get live secrets each invocation
// without needing to keep a `.env` on disk in sync.
//
// relativeDir uses the same convention as one.manifest.json subprojects
// (e.g. "services/api" — POSIX, no leading slash). When empty the
// workspace-root path is used.
//
// Errors propagate raw so callers can branch:
//   - INFISICAL_NOT_CONFIGURED — workspace's domains.env.config.projectId is unset
//   - INFISICAL_AUTH_MISSING   — no default env profile / profile has no creds
//   - INFISICAL_AUTH_FAILED / INFISICAL_API_ERROR — network / API-level
//
// Credentials + siteUrl come exclusively from the resolved env profile.
// Env vars are no longer read.
func FetchSecretsForSubproject(ctx context.Context, projectRoot, relativeDir, envName string) (map[string]string, error) {
	cfg, err := RequireWorkspaceConfig(projectRoot)
	if err != nil {
		return nil, err
	}
	profileName, creds, siteURL, err := requireProfileCreds(projectRoot, "")
	if err != nil {
		return nil, err
	}
	cfg.SiteURL = siteURL
	cfg.ProfileName = profileName
	env, err := SanitizeEnvName(envOrDefault(envName, cfg.DefaultEnvOrFallback()))
	if err != nil {
		return nil, err
	}
	client, err := NewClient(ctx, cfg, creds)
	if err != nil {
		return nil, err
	}

	resolution, err := resolveRunPath(cfg, projectRoot, relativeDir)
	if err != nil {
		return nil, err
	}

	merged := map[string]string{}
	for _, p := range resolution.Chain {
		secrets, err := client.ListSecrets(env, p, false)
		if err != nil {
			// A missing folder along the inheritance chain is normal: the
			// chain is `[/, /apps, /apps/web]` for a workspace whose
			// secrets only live under /apps/web. The intermediate `/apps`
			// folder doesn't have to exist on Infisical — that path was
			// only there in case it had shared values to inherit. Treat
			// FolderNotFound as "no secrets at this level" and continue.
			// Other errors (auth / network / project-not-found) still bail.
			if isFolderNotFound(err) {
				continue
			}
			return nil, err
		}
		for _, s := range secrets {
			merged[s.SecretKey] = s.SecretValue
		}
	}
	return merged, nil
}

// isFolderNotFound reports whether err is the structured
// INFISICAL_FOLDER_NOT_FOUND envelope (the only "soft" error class in
// the chain walk).
func isFolderNotFound(err error) bool {
	if err == nil {
		return false
	}
	type coded interface{ ErrorCode() string }
	if c, ok := err.(coded); ok {
		return c.ErrorCode() == "INFISICAL_FOLDER_NOT_FOUND"
	}
	return false
}

// resolveRunPath builds the PathResolution for one subproject without
// touching the manifest's full subproject list. Mirrors what
// pull.buildPullTargets does for a single target, but takes the relativeDir
// directly so callers don't have to pre-walk the manifest.
func resolveRunPath(cfg *WorkspaceConfig, projectRoot, relativeDir string) (PathResolution, error) {
	if relativeDir == "" {
		return ResolveSubprojectPath(cfg, nil, nil), nil
	}
	rel := workspace.ToPosixPath(relativeDir)
	sub := &workspace.Project{
		Name:        filepath.Base(rel),
		RelativeDir: rel,
		TargetDir:   filepath.Join(projectRoot, filepath.FromSlash(rel)),
	}
	override, err := LoadSubprojectConfig(projectRoot, rel)
	if err != nil {
		return PathResolution{}, err
	}
	return ResolveSubprojectPath(cfg, sub, override), nil
}
