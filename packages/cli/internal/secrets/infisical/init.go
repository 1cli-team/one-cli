package infisical

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// InitInput captures the auto-bind inputs for an Infisical workspace.
//
// All fields are optional. The default flow auto-creates an Infisical
// project named after the workspace (manifest.project.name);
// --project-id short-circuits creation (bind to an existing Infisical
// project), and --project-name overrides the desired name when
// auto-creating.
//
// Scope split (post-profile refactor):
//   - This path writes WORKSPACE-level fields to manifest.domains.env.config
//     and manifest.environments (projectId, projectName, environments,
//     defaultEnv, rootPath).
//   - SiteURL + credentials are MACHINE-level — they come from a
//     profile (`one configure add env/infisical --profile <name>`), not from flags here.
type InitInput struct {
	ProjectID    string
	ProjectName  string
	Environments []string
	DefaultEnv   string
	RootPath     string
	// ProfileName one-shot overrides the default env profile (for the
	// network call that creates / verifies the project). Doesn't change
	// machine default.
	ProfileName string
	// SkipVerify lets `init` write the config without contacting Infisical
	// (useful for offline workflows / generation tooling). Default off:
	// the CLI's value is in catching configuration mistakes early.
	// Note: skipping verify also disables auto-create — the resolved
	// projectId must be supplied explicitly.
	SkipVerify bool
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
	WrittenTo    string   `json:"written_to"`  // absolute path to one.manifest.json
}

// RenderTTY prints the init outcome.
func (r *InitResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	fmt.Fprintln(w, "✓ Secrets configuration written")
	if r.Created {
		fmt.Fprintf(w, "  Project: %s (%s) — created\n", r.ProjectName, r.ProjectID)
	} else if r.ProjectName != "" {
		fmt.Fprintf(w, "  Project: %s (%s)\n", r.ProjectName, r.ProjectID)
	} else {
		fmt.Fprintf(w, "  Project: %s\n", r.ProjectID)
	}
	fmt.Fprintf(w, "  Environments: %s (default: %s)\n",
		strings.Join(r.Environments, ", "), r.DefaultEnv)
	fmt.Fprintf(w, "  Root path: %s\n", r.RootPath)
	fmt.Fprintf(w, "  Auth: %s\n", r.AuthStatus)
	fmt.Fprintf(w, "  Written to: %s\n", r.WrittenTo)
}

// maxCreateProjectRetries caps the suffix-retry loop. Five 4-char hex
// suffixes give a 20-bit search space — collisions on every attempt would
// indicate Infisical-side trouble, not legitimate name competition.
const maxCreateProjectRetries = 5

// Init writes (or updates) the workspace's Infisical configuration under
// one.manifest.json#domains.env.config plus manifest.environments.
//
// Three branches:
//
//  1. in.ProjectID is set — bind to the named Infisical project. Verify
//     reachable (unless --skip-verify) and persist.
//  2. in.ProjectID empty + a previous auto-bind already wrote a projectId
//     to the manifest — re-verify and idempotently rewrite the config.
//  3. in.ProjectID empty + no prior config — auto-create on Infisical using
//     manifest.project.name (or the override --project-name), retrying with
//     a short random suffix on name collisions, and write the resolved id +
//     name back into manifest.domains.env.config.
//
// The function is idempotent within each branch.
func Init(ctx context.Context, projectRoot string, in InitInput) (*InitResult, error) {
	if !workspace.HasManifest(projectRoot) {
		return nil, cliErrors.New(cliErrors.NOT_ONE_PROJECT,
			"未找到 one.manifest.json。请先在 One workspace 根目录执行。")
	}
	// Inherit existing manifest.environments.names / default when the
	// caller didn't pass --envs / --default-env. This keeps re-runs of
	// auto-bind idempotent against a workspace that customised its environment
	// list (otherwise DefaultEnvironments would clobber whatever was there).
	if len(in.Environments) == 0 || strings.TrimSpace(in.DefaultEnv) == "" {
		if existing, _ := LoadWorkspaceConfig(projectRoot); existing != nil {
			if len(in.Environments) == 0 && len(existing.Environments) > 0 {
				in.Environments = append([]string{}, existing.Environments...)
			}
			if strings.TrimSpace(in.DefaultEnv) == "" && existing.DefaultEnv != "" {
				in.DefaultEnv = existing.DefaultEnv
			}
		}
	}
	in = applyInitDefaults(in)

	cfg := &WorkspaceConfig{
		ProjectID:    strings.TrimSpace(in.ProjectID),
		ProjectName:  strings.TrimSpace(in.ProjectName),
		Environments: dedupeStrings(in.Environments),
		DefaultEnv:   strings.TrimSpace(in.DefaultEnv),
		RootPath:     strings.TrimSpace(in.RootPath),
	}

	// Branch 2 fallback: re-use a previously-stored projectId so re-running
	// `env init` is idempotent and does NOT try to create a duplicate
	// project upstream.
	if cfg.ProjectID == "" {
		if existing, _ := LoadWorkspaceConfig(projectRoot); existing != nil && existing.ProjectID != "" {
			cfg.ProjectID = existing.ProjectID
			if cfg.ProjectName == "" {
				cfg.ProjectName = existing.ProjectName
			}
		}
	}

	authStatus := "skipped"
	created := false

	// Auto-create branch (Branch 3). Triggered only when we still have no
	// projectId AND we're allowed to talk to the network.
	if cfg.ProjectID == "" {
		if in.SkipVerify {
			return nil, cliErrors.New(cliErrors.INFISICAL_NOT_CONFIGURED,
				"--skip-verify 与自动创建项目互斥。请显式传 --project-id，或去掉 --skip-verify。")
		}
		desiredName, err := resolveProjectName(projectRoot, cfg.ProjectName)
		if err != nil {
			return nil, err
		}
		profileName, creds, siteURL, err := loadInitCreds(projectRoot, in.ProfileName)
		if err != nil {
			return nil, err
		}
		cfg.SiteURL = siteURL
		cfg.ProfileName = profileName
		client, err := NewClient(ctx, cfg, creds)
		if err != nil {
			return nil, err
		}
		id, resolvedName, err := createWithRetry(client, desiredName)
		if err != nil {
			return nil, err
		}
		cfg.ProjectID = id
		cfg.ProjectName = resolvedName
		authStatus = "created"
		created = true

		// Back-fill the manifest's workspace identity. New scaffolds set
		// project at create time; older workspaces (or those that lost the
		// field) get it written here so subsequent `env init` runs and any
		// future identity-aware command can rely on it.
		if err := ensureManifestProject(projectRoot, resolvedName); err != nil {
			return nil, err
		}
	} else if !in.SkipVerify {
		// Branch 1 / Branch-2-rewrite: validate the explicit / cached id.
		profileName, creds, siteURL, err := loadInitCreds(projectRoot, in.ProfileName)
		if err != nil {
			return nil, err
		}
		cfg.SiteURL = siteURL
		cfg.ProfileName = profileName
		client, err := NewClient(ctx, cfg, creds)
		if err != nil {
			return nil, err
		}
		if err := client.VerifyProjectExists(cfg.DefaultEnvOrFallback()); err != nil {
			return nil, err
		}
		authStatus = "verified"
	}

	cfg.RootPath = cfg.RootPathOrDefault()
	configJSON, err := EncodeManifestConfig(cfg)
	if err != nil {
		return nil, err
	}
	if err := workspace.InitWorkspaceEnv(projectRoot, workspace.EnvInit{
		Kind:             workspace.EnvBackendInfisical,
		ConfigJSON:       configJSON,
		EnvironmentNames: cfg.Environments,
		DefaultEnv:       cfg.DefaultEnvOrFallback(),
	}); err != nil {
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

// loadInitCreds is a thin alias around requireProfileCreds, kept so the
// two call sites in Init read locally rather than spelling out the
// shared helper name. Profile-only — env vars retired.
func loadInitCreds(projectRoot, profileFlag string) (string, *Credentials, string, error) {
	return requireProfileCreds(projectRoot, profileFlag)
}

// resolveProjectName picks the Infisical project name when env init is
// auto-creating. Precedence: explicit override → manifest.project.name →
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
		"无法推断 Infisical 项目名（manifest.project.name / package.json#name 都为空）。请显式传 --project-name 或 --project-id。")
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
	candidate := baseName
	for attempt := 0; attempt < maxCreateProjectRetries; attempt++ {
		id, resolved, err := client.CreateProject(candidate)
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
		fmt.Sprintf("Infisical 项目名 %q 反复冲突，已重试 %d 次。请显式传 --project-name 指定一个唯一名字。",
			baseName, maxCreateProjectRetries))
}

func applyInitDefaults(in InitInput) InitInput {
	if len(in.Environments) == 0 {
		in.Environments = append([]string{}, DefaultEnvironments...)
	}
	if strings.TrimSpace(in.DefaultEnv) == "" {
		in.DefaultEnv = in.Environments[0]
	}
	if strings.TrimSpace(in.RootPath) == "" {
		in.RootPath = "/"
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
