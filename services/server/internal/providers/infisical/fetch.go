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

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

// Infisical's recursive list endpoint supports at most 20 directory levels.
const maxSnapshotDepth = 20

// Snapshot lifetime is exactly one call: a later launch always reads fresh
// values. One recursive read supplies all projects, then each project receives
// only the folders in its inheritance chain, merged from root to leaf.
func FetchConfigured(ctx context.Context, cfg *WorkspaceConfig, dirs []string, envName string) (map[string]map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := make(map[string]map[string]string, len(dirs))
	if len(dirs) == 0 {
		return result, nil
	}
	env, err := SanitizeEnvName(envOrDefault(envName, cfg.DefaultEnvOrFallback()))
	if err != nil {
		return nil, err
	}
	creds, siteURL, err := sessionCredentials(ctx)
	if err != nil {
		return nil, err
	}
	if cfg.SiteURL != "" && cfg.SiteURL != siteURL {
		return nil, i18n.ForContext(ctx).Errorf("infisical.binding_instance_mismatch")
	}
	cfg.SiteURL = siteURL
	chains := make(map[string][]string, len(dirs))
	paths := map[string]bool{}
	for _, dir := range dirs {
		resolution, err := resolveRunPath("", dir)
		if err != nil {
			return nil, err
		}
		if len(resolution.Chain)-1 > maxSnapshotDepth {
			return nil, cliErrors.New(cliErrors.INFISICAL_API_ERROR, i18n.ForContext(ctx).
				Tf("infisical.snapshot_depth_exceeded", dir, maxSnapshotDepth))
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
			return nil, cliErrors.New(cliErrors.INFISICAL_API_ERROR, i18n.ForContext(ctx).
				T("infisical.snapshot_path_missing"))
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
