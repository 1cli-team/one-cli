package infisical

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

// InitInput captures the auto-bind inputs for an Infisical workspace.
//
// All fields are optional. The default flow auto-creates an Infisical
// project named after the workspace (manifest.workspace.name);
// ProjectID binds to an existing project; ProjectName overrides the desired
// display name when creating one. Names are never used to find an existing ID.
//
// Authentication uses the single browser session stored in the system keyring.
// Only project metadata is persisted in the workspace manifest.
type InitInput struct {
	ProjectID    string
	ProjectName  string
	Environments []string
	// SkipVerify lets `init` write the config without contacting Infisical
	// (useful for offline workflows / generation tooling). Default off:
	// the CLI's value is in catching configuration mistakes early.
	// Note: skipping verify also disables auto-create — the resolved
	// projectId must be supplied explicitly.
	SkipVerify bool
	// Session pins all remote work to the caller's snapshot. When omitted, Init
	// captures the current session once before contacting Infisical.
	Session *session.Session
	// BeforeWrite rejects stale Dashboard requests after remote work and before
	// publishing any local changes. The caller holds the manifest lock.
	BeforeWrite func() error
}

// InitResult is the JSON payload emitted by auto-bind. Mirrors the
// one-cli/env-init/v1 schema rev for the Infisical era.
type InitResult struct {
	Schema       string   `json:"schema"`
	ProjectID    string   `json:"project_id"`
	ProjectName  string   `json:"project_name,omitempty"`
	Environments []string `json:"environments"`
	DefaultEnv   string   `json:"default_env"`
	RootPath     string   `json:"root_path"`
	AuthStatus   string   `json:"auth_status"` // "verified" / "skipped" / "created"
	Created      bool     `json:"created"`     // true when env init created the Infisical project
	WrittenTo    string   `json:"written_to"`  // absolute path to one.manifest.toml
}

// maxCreateProjectRetries allows one attempt with the original name followed
// by at most four attempts with independently generated 4-character hex suffixes.
const maxCreateProjectRetries = 5

// Init writes (or updates) the workspace's Infisical configuration under
// one.manifest.toml under [env.infisical].
//
// Three branches:
//
//  1. in.ProjectID is set — bind to the named Infisical project. Verify
//     reachable (unless --skip-verify) and persist.
//  2. in.ProjectID empty + a previous auto-bind already wrote a projectId
//     to the manifest — re-verify and idempotently rewrite the config.
//  3. in.ProjectID empty + no prior config — auto-create on Infisical using
//     manifest.workspace.name (or the ProjectName override), retrying with
//     a short random suffix on name collisions, and write the resolved id +
//     name back into manifest.env.
//
// The function is idempotent within each branch.
func Init(ctx context.Context, projectRoot string, in InitInput) (*InitResult, error) {
	if !workspace.HasManifest(projectRoot) {
		return nil, cliErrors.New(cliErrors.NOT_ONE_PROJECT,
			i18n.T("workspace.manifest_required"))
	}
	manifest, err := workspace.ReadManifest(projectRoot)
	if err != nil {
		return nil, err
	}
	if len(in.Environments) == 0 {
		in.Environments = workspace.EnvironmentNames(manifest)
	}
	if manifest.Env != nil && strings.TrimSpace(in.ProjectID) == "" {
		in.ProjectID = manifest.Env.ProjectID
	}
	in = applyInitDefaults(in)
	cfg := &WorkspaceConfig{
		ProjectID:    strings.TrimSpace(in.ProjectID),
		ProjectName:  strings.TrimSpace(in.ProjectName),
		Environments: dedupeStrings(in.Environments),
	}

	authStatus := "skipped"
	created := false
	if cfg.ProjectID == "" && in.SkipVerify {
		return nil, cliErrors.New(cliErrors.INFISICAL_NOT_CONFIGURED,
			i18n.T("infisical.init.verify_conflict"))
	}
	current := in.Session
	var client *Client
	if !in.SkipVerify {
		if current == nil {
			current, err = session.Require()
			if err != nil {
				return nil, err
			}
		}
		cfg.SiteURL = current.SiteURL
		client, err = NewClient(ctx, cfg, &Credentials{AccessToken: current.Token})
		if err != nil {
			return nil, err
		}
	}

	// Auto-create branch (Branch 3). Triggered only when we still have no
	// projectId AND we're allowed to talk to the network.
	if cfg.ProjectID == "" {
		desiredName, err := resolveProjectName(projectRoot, cfg.ProjectName)
		if err != nil {
			return nil, err
		}
		id, resolvedName, err := createWithRetryContext(ctx, client, desiredName)
		if err != nil {
			return nil, err
		}
		cfg.ProjectID = id
		cfg.ProjectName = resolvedName
		authStatus = "created"
		created = true
	} else if !in.SkipVerify {
		// Branch 1 / Branch-2-rewrite: validate the explicit / cached id.
		if err := client.VerifyProjectExists(cfg.DefaultEnvOrFallback()); err != nil {
			return nil, err
		}
		authStatus = "verified"
	}
	publish := func() error {
		if in.BeforeWrite != nil {
			if err := in.BeforeWrite(); err != nil {
				return err
			}
		}
		// Back-fill older workspace identities only after both the revision and
		// session have been validated, under the same local commit guard.
		if created {
			if err := ensureManifestProject(projectRoot, cfg.ProjectName); err != nil {
				return err
			}
		}
		configJSON, err := EncodeManifestConfig(cfg)
		if err != nil {
			return err
		}
		return workspace.InitWorkspaceEnv(projectRoot, workspace.EnvInit{
			Kind:             workspace.EnvBackendInfisical,
			ConfigJSON:       configJSON,
			EnvironmentNames: cfg.Environments,
		})
	}
	if in.SkipVerify {
		err = publish()
	} else {
		err = session.WithUnchanged(current, publish)
	}
	if err != nil {
		if created {
			return nil, bindingWriteError(projectRoot, cfg, err)
		}
		return nil, err
	}

	return &InitResult{
		Schema:       "one-cli/env-init/v1",
		ProjectID:    cfg.ProjectID,
		ProjectName:  cfg.ProjectName,
		Environments: cfg.Environments,
		DefaultEnv:   cfg.DefaultEnvOrFallback(),
		RootPath:     cfg.RootPathOrDefault(),
		AuthStatus:   authStatus,
		Created:      created,
		WrittenTo:    workspace.ManifestPath(projectRoot),
	}, nil
}

// resolveProjectName picks the Infisical project name when env init is
// auto-creating. Precedence: explicit override → manifest.workspace.name →
// package.json#name → workspace folder basename. The first non-empty value
// wins; if all are empty we surface a clear error.
func resolveProjectName(projectRoot, override string) (string, error) {
	if v := strings.TrimSpace(override); v != "" {
		return v, nil
	}
	m, err := workspace.ReadManifest(projectRoot)
	if err == nil && m != nil && m.Workspace != nil && strings.TrimSpace(m.Workspace.Name) != "" {
		return strings.TrimSpace(m.Workspace.Name), nil
	}
	if name := readPackageJSONName(projectRoot); name != "" {
		return name, nil
	}
	if base := strings.TrimSpace(filepath.Base(projectRoot)); base != "" && base != "." && base != "/" {
		return base, nil
	}
	return "", cliErrors.New(cliErrors.INFISICAL_NOT_CONFIGURED,
		i18n.T("infisical.init.name_required"))
}

func readPackageJSONName(projectRoot string) string {
	raw, err := os.ReadFile(filepath.Join(projectRoot, "package.json"))
	if err != nil {
		return ""
	}
	var doc struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}
	return strings.TrimSpace(doc.Name)
}

// ensureManifestProject back-fills the workspace identity block when older
// scaffolds left it empty. If the block already has both id and name, it
// is left untouched.
func ensureManifestProject(projectRoot, fallbackName string) error {
	m, err := workspace.ReadManifest(projectRoot)
	if err != nil {
		// Don't escalate: a malformed manifest is a separate concern from
		// env init. If we can't read it, skip back-fill.
		return nil
	}
	if m != nil && m.Workspace != nil && strings.TrimSpace(m.Workspace.ID) != "" && strings.TrimSpace(m.Workspace.Name) != "" {
		return nil
	}
	id := ""
	name := strings.TrimSpace(fallbackName)
	if m != nil && m.Workspace != nil {
		id = strings.TrimSpace(m.Workspace.ID)
		if existing := strings.TrimSpace(m.Workspace.Name); existing != "" {
			name = existing
		}
	}
	if id == "" {
		id = workspace.GenerateProjectID(name)
	}
	return workspace.SetManifestWorkspaceIdentity(projectRoot, id, name)
}

// createWithRetry calls Infisical's create-project, appending a 4-char hex
// suffix to the desired name on collisions. The first attempt uses the bare
// name so the common case (unused name) lands cleanly without decoration.
func createWithRetry(client *Client, baseName string) (string, string, error) {
	return createWithRetryContext(context.Background(), client, baseName)
}

func createWithRetryContext(ctx context.Context, client *Client, baseName string) (string, string, error) {
	candidate := baseName
	for attempt := 0; attempt < maxCreateProjectRetries; attempt++ {
		id, resolved, err := client.CreateProjectContext(ctx, candidate)
		if err == nil {
			return id, resolved, nil
		}
		var typed *output.Error
		if errors.As(err, &typed) && typed.Code == string(cliErrors.INFISICAL_PROJECT_NAME_TAKEN) {
			candidate = baseName + "-" + randomSuffix(4)
			continue
		}
		return "", "", err
	}
	return "", "", cliErrors.New(cliErrors.INFISICAL_PROJECT_NAME_TAKEN,
		i18n.Tf("infisical.init.name_conflicts",
			baseName, maxCreateProjectRetries))
}

func applyInitDefaults(in InitInput) InitInput {
	if len(in.Environments) == 0 {
		in.Environments = append([]string{}, DefaultEnvironments...)
	}
	return in
}

// randomSuffix returns a hex string of n chars using crypto/rand. Falls back
// to all-zero on the (extremely unlikely) read error so the retry loop still
// makes forward progress.
func randomSuffix(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return strings.Repeat("0", n)
	}
	return hex.EncodeToString(buf)[:n]
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func bindingWriteError(root string, cfg *WorkspaceConfig, err error) error {
	code := cliErrors.ONE_CLI_ERROR
	var original *output.Error
	if errors.As(err, &original) {
		switch original.Code {
		case string(cliErrors.SERVE_MANIFEST_CONFLICT), string(cliErrors.INFISICAL_AUTH_MISSING), string(cliErrors.INFISICAL_AUTH_FAILED):
			code = cliErrors.Code(original.Code)
		}
	}
	return cliErrors.New(code,
		i18n.Errorf("infisical.init.binding_write_failed", cfg.ProjectName, cfg.ProjectID, workspace.ManifestPath(root), err).Error()).
		WithContext(map[string]any{"project_id": cfg.ProjectID, "project_name": cfg.ProjectName, "partial_state": "project_created_binding_unsaved"}).WithCause(err)
}
