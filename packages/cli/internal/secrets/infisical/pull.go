package infisical

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets/dotenv"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// PullInput captures `env pull` flags.
type PullInput struct {
	Env     string
	Project string // optional subproject selector (name or relativeDir); empty = pull all
	Force   bool   // overwrite existing .env even if content differs
	DryRun  bool   // do not write any files; just report what would happen

	// Cfg / Creds: see crud.go for rationale. When nil, the pull
	// pipeline resolves them via the cobra layer's profile lookup.
	Cfg   *WorkspaceConfig
	Creds *Credentials
}

// PullResult is the JSON envelope. PerSubproject reports per-project
// status plus the InfisicalPath actually used.
type PullResult struct {
	Schema        string      `json:"schema"`
	Env           string      `json:"env"`
	DryRun        bool        `json:"dry_run"`
	WrittenCount  int         `json:"written_count"`
	SkippedCount  int         `json:"skipped_count"`
	PerSubproject []PullEntry `json:"per_subproject"`
}

// PullEntry is one row of PullResult.PerSubproject.
type PullEntry struct {
	Name          string   `json:"name"`
	RelativeDir   string   `json:"relative_dir"`
	InfisicalPath string   `json:"infisical_path"`
	EnvFilePath   string   `json:"env_file_path"`
	Status        string   `json:"status"` // written / unchanged / skipped / dry-run
	Reason        string   `json:"reason,omitempty"`
	KeysWritten   []string `json:"keys_written,omitempty"`
}

// RenderTTY summarises which subprojects got new .env content and which
// were skipped, with reasons.
func (r *PullResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	mode := "wrote"
	if r.DryRun {
		mode = "would write"
	}
	fmt.Fprintf(w, "Env: %s · %s %d, skipped %d\n", r.Env, mode, r.WrittenCount, r.SkippedCount)
	for _, e := range r.PerSubproject {
		mark := "·"
		switch e.Status {
		case "written", "dry-run":
			mark = "✓"
		case "unchanged":
			mark = "="
		case "skipped":
			mark = "✗"
		}
		line := fmt.Sprintf("  %s %s [%s] → %s", mark, e.RelativeDir, e.Status, e.EnvFilePath)
		if e.Reason != "" {
			line += " — " + e.Reason
		}
		fmt.Fprintln(w, line)
		if len(e.KeysWritten) > 0 {
			fmt.Fprintf(w, "      keys: %s\n", strings.Join(e.KeysWritten, ", "))
		}
	}
}

// Pull is the main pull entry point.
//
// Subproject discovery is driven by one.manifest.json (workspace-level
// source of truth). Each manifest subproject becomes one pull target;
// every key Infisical surfaces along its inheritance chain is written to
// <subproject>/.env. There is no longer a workspace-root target — secrets
// belong to the subprojects they're scoped for.
//
// Path isolation is the only filter: a subproject's path means it can
// only see secrets in that folder + parent folders (when inherits=true).
// `.env.example` is no longer consulted; if you don't want a key in a
// subproject, scope it correctly in Infisical.
func Pull(ctx context.Context, projectRoot string, in PullInput) (*PullResult, error) {
	cfg, creds, err := resolveCfgAndCreds(projectRoot, in.Cfg, in.Creds)
	if err != nil {
		return nil, err
	}
	env, err := SanitizeEnvName(envOrDefault(in.Env, cfg.DefaultEnvOrFallback()))
	if err != nil {
		return nil, err
	}
	client, err := NewClient(ctx, cfg, creds)
	if err != nil {
		return nil, err
	}

	targets, err := buildPullTargets(projectRoot, cfg, in.Project)
	if err != nil {
		return nil, err
	}

	result := &PullResult{
		Schema: "one-cli/env-pull/v1",
		Env:    env,
		DryRun: in.DryRun,
	}

	for _, t := range targets {
		entry, conflict, err := pullOne(client, env, t, in.Force, in.DryRun)
		if err != nil {
			return nil, err
		}
		if conflict {
			return nil, cliErrors.New(cliErrors.ENV_PULL_CONFLICT,
				"已有 .env 与 Infisical 拉取的密钥不一致："+entry.EnvFilePath+"。如需覆盖请加 --force。").
				WithContext(map[string]any{
					"env_file_path":  entry.EnvFilePath,
					"relative_dir":   entry.RelativeDir,
					"infisical_path": entry.InfisicalPath,
				})
		}
		result.PerSubproject = append(result.PerSubproject, entry)
		switch entry.Status {
		case "written":
			result.WrittenCount++
		case "dry-run":
			result.WrittenCount++
		default:
			result.SkippedCount++
		}
	}
	return result, nil
}

// pullTarget is one subproject the pull pipeline should produce a .env for.
type pullTarget struct {
	TargetDir   string // absolute dir where .env lives
	RelativeDir string // displayable relative path
	Name        string // subproject name
	Resolution  PathResolution
}

func buildPullTargets(projectRoot string, cfg *WorkspaceConfig, projectSelector string) ([]pullTarget, error) {
	m, err := workspace.ReadManifest(projectRoot)
	if err != nil {
		return nil, err
	}

	// Optional subproject filter: name (manifest.projects[].name) or
	// relative path (with tolerated leading ./). The literal "/" filter
	// addresses the workspace-root scope explicitly.
	wantSub := strings.TrimSpace(projectSelector)
	wantPath := ""
	if wantSub != "" {
		wantPath = workspace.ToPosixPath(strings.TrimSuffix(strings.TrimPrefix(wantSub, "./"), "/"))
	}

	out := make([]pullTarget, 0, len(m.Projects)+1)

	// Workspace-root target: pull keys at cfg.RootPath into
	// `<workspace>/.env`. Only included when no -p filter is set, or
	// when the filter explicitly addresses root ("/" or empty path).
	includeRoot := wantSub == "" || wantSub == "/" || wantPath == ""
	if includeRoot && rootHasDeclaredKeys(m) {
		rootRes := ResolveSubprojectPath(cfg, nil, nil)
		out = append(out, pullTarget{
			TargetDir:   projectRoot,
			RelativeDir: ".",
			Name:        "<workspace>",
			Resolution:  rootRes,
		})
	}

	for _, sub := range m.Projects {
		if wantSub != "" && sub.Name != wantSub && sub.RelativeDir != wantPath {
			continue
		}
		targetDir := filepath.Join(projectRoot, filepath.FromSlash(sub.RelativeDir))
		ws := workspace.Project{
			Name:           sub.Name,
			TargetDir:      targetDir,
			RelativeDir:    sub.RelativeDir,
			Toolchain:      sub.Toolchain,
			PackageManager: sub.PackageManager,
			TemplateID:     sub.TemplateID,
		}
		override, err := LoadSubprojectConfig(projectRoot, sub.RelativeDir)
		if err != nil {
			return nil, err
		}
		if override != nil && override.Disabled {
			continue
		}
		res := ResolveSubprojectPath(cfg, &ws, override)
		out = append(out, pullTarget{
			TargetDir:   targetDir,
			RelativeDir: sub.RelativeDir,
			Name:        sub.Name,
			Resolution:  res,
		})
	}

	if len(out) == 0 {
		if wantSub != "" {
			return nil, cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND,
				"找不到名字或路径匹配 "+wantSub+" 的项目。已声明: "+
					strings.Join(workspace.ProjectNames(m), ", "))
		}
		return nil, cliErrors.New(cliErrors.MANIFEST_MISSING_OR_EMPTY,
			"manifest 没有声明任何项目，且 workspace 根也没有 keys 可拉。先 `one add` 新建一个，或 `one env set` 在根级写入 keys。")
	}
	return out, nil
}

// rootHasDeclaredKeys reports whether the workspace-root scope has any
// keys declared in manifest.domains.env.config.keys. Used so pull doesn't
// manufacture a workspace `.env` for a project that has no globals —
// avoids touching files in workspaces that exclusively use per-subproject
// keys.
func rootHasDeclaredKeys(m *workspace.Manifest) bool {
	return len(workspace.WorkspaceEnvKeys(m)) > 0
}

// pullOne is the per-target work: fetch keys via the inheritance chain and
// write the .env. Returns the rendered entry and whether the user must
// --force (conflict detected).
func pullOne(client *Client, env string, t pullTarget, force, dryRun bool) (PullEntry, bool, error) {
	envFilePath := filepath.Join(t.TargetDir, ".env")
	relativeDir := t.RelativeDir
	if relativeDir == "" {
		relativeDir = "."
	}
	entry := PullEntry{
		Name:          t.Name,
		RelativeDir:   relativeDir,
		InfisicalPath: t.Resolution.Path,
		EnvFilePath:   envFilePath,
	}

	merged := map[string]string{}
	for _, p := range t.Resolution.Chain {
		secrets, err := client.ListSecrets(env, p, false)
		if err != nil {
			// Same reasoning as fetch.go: a missing intermediate folder
			// in the inheritance chain is not fatal — that path was only
			// walked in case it carried inheritable values.
			if isFolderNotFound(err) {
				continue
			}
			return entry, false, err
		}
		for _, s := range secrets {
			merged[s.SecretKey] = s.SecretValue
		}
	}

	if len(merged) == 0 {
		entry.Status = "skipped"
		entry.Reason = "no secrets at " + t.Resolution.Path
		return entry, false, nil
	}

	existingRaw, err := os.ReadFile(envFilePath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return entry, false, err
	}
	if err == nil {
		existing := dotenv.Parse(string(existingRaw))
		if dotenv.Equal(existing, merged) {
			entry.Status = "unchanged"
			entry.KeysWritten = sortedKeys(merged)
			return entry, false, nil
		}
		if !force {
			entry.Status = "skipped"
			entry.Reason = "existing .env conflicts; pass --force to overwrite"
			return entry, true, nil
		}
	}

	if dryRun {
		entry.Status = "dry-run"
		entry.KeysWritten = sortedKeys(merged)
		return entry, false, nil
	}

	if err := os.MkdirAll(t.TargetDir, 0o755); err != nil {
		return entry, false, err
	}
	body := dotenv.Serialize(merged)
	if err := os.WriteFile(envFilePath, []byte(body), 0o600); err != nil {
		return entry, false, err
	}
	entry.Status = "written"
	entry.KeysWritten = sortedKeys(merged)
	return entry, false, nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
