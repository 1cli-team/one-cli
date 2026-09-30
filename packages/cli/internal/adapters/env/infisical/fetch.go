package infisical

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	api "github.com/infisical/go-sdk/packages/api/secrets"
	"github.com/infisical/go-sdk/packages/models"
	"github.com/infisical/go-sdk/packages/util"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
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

// Snapshot lifetime is exactly one call: a later launch always reads fresh
// values. Unique folders are requested concurrently, then merged in chain order.
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
	indices := map[string]int{}
	paths := []string{}
	for _, dir := range dirs {
		resolution, err := resolveRunPath(projectRoot, dir)
		if err != nil {
			return nil, err
		}
		chains[dir] = resolution.Chain
		for _, path := range resolution.Chain {
			if _, exists := indices[path]; !exists {
				indices[path] = len(paths)
				paths = append(paths, path)
			}
		}
	}
	folders, err := fetchFolders(ctx, cfg, creds, env, paths)
	if err != nil {
		return nil, err
	}
	for dir, chain := range chains {
		merged := map[string]string{}
		for _, path := range chain {
			for _, secret := range folders[indices[path]] {
				merged[secret.SecretKey] = secret.SecretValue
			}
		}
		result[dir] = merged
	}
	return result, nil
}

const maxFolderRequests = 6

func fetchFolders(ctx context.Context, cfg *WorkspaceConfig, creds *Credentials, env string, paths []string) ([][]models.Secret, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// The SDK's high-level client does not pass its constructor context to
	// HTTP requests. Reuse its list API and error contract with a cancellable
	// request client so Ctrl-C and a failed sibling stop in-flight reads.
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
	results := make([][]models.Secret, len(paths))
	jobs := make(chan int, len(paths))
	for i := range paths {
		jobs <- i
	}
	close(jobs)
	var wg sync.WaitGroup
	var once sync.Once
	var firstErr error
	for range min(maxFolderRequests, len(paths)) {
		wg.Go(func() {
			for i := range jobs {
				if ctx.Err() != nil {
					return
				}
				response, err := api.CallListSecretsV3(nil, client, api.ListSecretsV3RawRequest{
					ProjectID: cfg.ProjectID, Environment: env, SecretPath: paths[i],
					ExpandSecretReferences: true,
				})
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					err = mapAPIError(err)
					// Missing ancestors are empty, never a reason to hide auth,
					// network, or project-not-found failures.
					if isFolderNotFound(err) {
						continue
					}
					once.Do(func() { firstErr = err; cancel() })
					return
				}
				results[i] = response.Secrets
			}
		})
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return results, nil
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
