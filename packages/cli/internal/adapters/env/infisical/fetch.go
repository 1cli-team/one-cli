package infisical

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	api "github.com/infisical/go-sdk/packages/api/secrets"
	"github.com/infisical/go-sdk/packages/models"
	"github.com/infisical/go-sdk/packages/util"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// FetchSecretsForSubproject pulls every secret a subproject can see from
// Infisical (along its inheritance chain) and returns the merged map. No
// disk IO. Used by `one run` so users get live secrets each invocation
// without needing to keep a `.env` on disk in sync.
//
// relativeDir uses the same convention as one.manifest.toml subprojects
// (e.g. "services/api" — POSIX, no leading slash). When empty the
// workspace-root path is used.
//
// Errors propagate raw so callers can branch:
//   - INFISICAL_NOT_CONFIGURED — workspace's env.infisical.projectId is unset
//   - INFISICAL_AUTH_MISSING   — no active browser session
//   - INFISICAL_AUTH_FAILED / INFISICAL_API_ERROR — network / API-level
//
// Credentials + siteUrl come exclusively from the active browser session.
// Env vars are no longer read.
func FetchSecretsForSubproject(ctx context.Context, projectRoot, relativeDir, envName string) (map[string]string, error) {
	projects, err := fetchSecretsForProjects(ctx, projectRoot, []string{relativeDir}, envName)
	if err != nil {
		return nil, err
	}
	return projects[relativeDir], nil
}

// Infisical's recursive list endpoint supports at most 20 directory levels.
const maxSnapshotDepth = 20

// Snapshot lifetime is exactly one call: a later launch always reads fresh
// values. One recursive read supplies all projects, then each project receives
// only the folders in its inheritance chain, merged from root to leaf.
func fetchSecretsForProjects(ctx context.Context, projectRoot string, dirs []string, envName string) (map[string]map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := make(map[string]map[string]string, len(dirs))
	if len(dirs) == 0 {
		return result, nil
	}
	cfg, err := RequireWorkspaceConfig(projectRoot)
	if err != nil {
		return nil, err
	}
	env, err := SanitizeEnvName(envOrDefault(envName, cfg.DefaultEnvOrFallback()))
	if err != nil {
		return nil, err
	}
	creds, siteURL, err := sessionCredentials()
	if err != nil {
		return nil, err
	}
	if cfg.SiteURL != "" && cfg.SiteURL != siteURL {
		return nil, i18n.Errorf("infisical.binding_instance_mismatch")
	}
	cfg.SiteURL = siteURL
	chains := make(map[string][]string, len(dirs))
	paths := map[string]bool{}
	for _, dir := range dirs {
		resolution, err := resolveRunPath(projectRoot, dir)
		if err != nil {
			return nil, err
		}
		if len(resolution.Chain)-1 > maxSnapshotDepth {
			return nil, cliErrors.New(cliErrors.INFISICAL_API_ERROR,
				i18n.Tf("infisical.snapshot_depth_exceeded", dir, maxSnapshotDepth))
		}
		chains[dir] = resolution.Chain
		for _, path := range resolution.Chain {
			paths[path] = true
		}
	}
	snapshot, err := fetchSnapshot(ctx, cfg, creds, env)
	if err != nil {
		return nil, err
	}
	folders := make(map[string][]models.Secret, len(paths))
	for _, secret := range snapshot {
		// A recursive response must identify each secret's folder. Treating a
		// missing path as root would distribute project-only values to siblings.
		if !strings.HasPrefix(secret.SecretPath, "/") {
			return nil, cliErrors.New(cliErrors.INFISICAL_API_ERROR,
				i18n.T("infisical.snapshot_path_missing"))
		}
		path := NormalizePath(secret.SecretPath)
		if paths[path] {
			folders[path] = append(folders[path], secret)
		}
	}
	for dir, chain := range chains {
		merged := map[string]string{}
		for _, path := range chain {
			for _, secret := range folders[path] {
				merged[secret.SecretKey] = secret.SecretValue
			}
		}
		result[dir] = merged
	}
	return result, nil
}

func fetchSnapshot(ctx context.Context, cfg *WorkspaceConfig, creds *Credentials, env string) ([]models.Secret, error) {
	// The SDK's high-level client does not pass its constructor context to
	// HTTP requests. Reuse its list API and error contract with a cancellable
	// request client so Ctrl-C stops the snapshot read.
	client := resty.New().
		SetBaseURL(util.AppendAPIEndpoint(cfg.SiteURLOrDefault())).
		SetHeader("User-Agent", "one-cli/"+clientVersion).
		SetAuthToken(creds.AccessToken).
		SetTimeout(30 * time.Second).
		SetRedirectPolicy(resty.NoRedirectPolicy()).
		OnBeforeRequest(func(_ *resty.Client, request *resty.Request) error {
			request.SetContext(ctx)
			return ctx.Err()
		})
	defer client.GetClient().CloseIdleConnections()
	response, err := api.CallListSecretsV3(nil, client, api.ListSecretsV3RawRequest{
		ProjectID: cfg.ProjectID, Environment: env, SecretPath: "/",
		ExpandSecretReferences: true, Recursive: true,
	})
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, mapAPIError(err)
	}
	return response.Secrets, nil
}

// resolveRunPath derives the fixed folder inheritance chain for a task directory.
func resolveRunPath(projectRoot, relativeDir string) (PathResolution, error) {
	if relativeDir == "" {
		return ResolveSubprojectPath(nil), nil
	}
	rel := workspace.ToPosixPath(relativeDir)
	sub := &workspace.Project{
		Name:        filepath.Base(rel),
		RelativeDir: rel,
		TargetDir:   filepath.Join(projectRoot, filepath.FromSlash(rel)),
	}
	return ResolveSubprojectPath(sub), nil
}
