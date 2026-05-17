package envcmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/prompt"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets/dotenv"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets/infisical"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// SwitchResult is the JSON envelope for `one env switch`. Mirrors the
// shape of other env subcommands; `synced` / `conflicts` are zero when
// --no-sync skipped the data migration (dotenv → infisical) or when
// switching to dotenv (no migration ever performed).
type SwitchResult struct {
	Schema       string `json:"schema"`
	From         string `json:"from"`
	To           string `json:"to"`
	ManifestPath string `json:"manifest_path"`
	Synced       int    `json:"synced,omitempty"`
	Conflicts    int    `json:"conflicts,omitempty"`
	SkippedSync  bool   `json:"skipped_sync,omitempty"`
}

// RenderTTY prints a human-readable summary.
func (r *SwitchResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	fmt.Fprintf(w, "✓ env backend: %s → %s\n", r.From, r.To)
	if r.SkippedSync {
		fmt.Fprintf(w, "  manifest 已切，未执行数据同步\n")
		return
	}
	if r.Synced > 0 || r.Conflicts > 0 {
		fmt.Fprintf(w, "  同步: %d 个 key 推送到 %s\n", r.Synced, r.To)
		if r.Conflicts > 0 {
			fmt.Fprintf(w, "  冲突: %d 个 key 未推送（加 --overwrite 重跑）\n", r.Conflicts)
		}
	}
}

func newSwitchCmd() *cobra.Command {
	var (
		yes       bool
		noSync    bool
		overwrite bool
		dryRun    bool
	)
	cmd := &cobra.Command{
		Use:   "switch <backend>",
		Short: "切换工作区的 env 后端 (dotenv / infisical)",
		Long: `切换工作区当前的 env 后端，可选地把本地 dotenv 数据一并同步到 Infisical。

合法 <backend>: dotenv | infisical

切到 infisical:
  1. 需要本机已有 default env/infisical profile
     （没有时报 INFISICAL_AUTH_MISSING，提示先跑 one configure add env/infisical）
  2. 默认交互式问你是否要把本地 .env 数据同步到 Infisical
     （--yes 跳过问询直接同步；--no-sync 仅切 manifest 不同步）
  3. Infisical 已存在的同名 key 默认报 ENV_MIGRATE_CONFLICT；加 --overwrite 覆盖

切到 dotenv:
  - 只改 manifest，不删 Infisical 数据（安全）
  - 切回后若需要把 Infisical 已有数据拉到本地，先跑 one env pull

示例:
  one env switch infisical                  # 交互式
  one env switch infisical -y               # 非交互：直接同步
  one env switch infisical --no-sync        # 仅切 manifest 不同步
  one env switch infisical --overwrite -y   # 覆盖冲突 key
  one env switch infisical --dry-run        # 只打印计划，不写
  one env switch dotenv                     # 切回 dotenv（只改 manifest）`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := runSwitch(cmd.Context(), args[0], switchFlags{
				yes:       yes,
				noSync:    noSync,
				overwrite: overwrite,
				dryRun:    dryRun,
			})
			if err != nil {
				return err
			}
			output.Emit(res)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "非交互：跳过同步确认，默认执行同步")
	cmd.Flags().BoolVar(&noSync, "no-sync", false, "只切 manifest 后端，不做数据同步")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "目标 backend 已有同名 key 时覆盖（默认报错）")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "只打印计划，不实际写")
	return cmd
}

type switchFlags struct {
	yes       bool
	noSync    bool
	overwrite bool
	dryRun    bool
}

func runSwitch(ctx context.Context, target string, fl switchFlags) (*SwitchResult, error) {
	target = strings.TrimSpace(target)
	if target != workspace.EnvBackendDotenv && target != workspace.EnvBackendInfisical {
		return nil, cliErrors.New(cliErrors.ENV_BACKEND_INVALID,
			fmt.Sprintf("不支持的 backend %q；合法值: dotenv / infisical", target))
	}

	root, err := workspace.ResolveProjectRoot("")
	if err != nil {
		return nil, err
	}
	m, err := workspace.ReadManifest(root)
	if err != nil {
		return nil, err
	}
	current := workspace.EnvBackend(m)
	if current == "" {
		current = workspace.EnvBackendDotenv
	}
	if current == target {
		return nil, cliErrors.New(cliErrors.ENV_BACKEND_UNCHANGED,
			fmt.Sprintf("工作区已经是 %s 后端，无需切换。", target)).
			WithContext(map[string]any{"backend": target})
	}

	manifestPath := filepath.Join(root, "one.manifest.json")

	// dotenv ← infisical: just flip the manifest. Don't touch Infisical
	// data; user can `one env pull` separately to grab any keys that
	// only live remote.
	if target == workspace.EnvBackendDotenv {
		if fl.dryRun {
			return &SwitchResult{
				Schema: "one-cli/env-switch/v1", From: current, To: target,
				ManifestPath: manifestPath, SkippedSync: true,
			}, nil
		}
		if err := writeEnvBackend(m, root, target); err != nil {
			return nil, err
		}
		return &SwitchResult{
			Schema: "one-cli/env-switch/v1", From: current, To: target,
			ManifestPath: manifestPath, SkippedSync: true,
		}, nil
	}

	// dotenv → infisical. Verify profile exists before scanning anything
	// so a misconfigured user fails fast.
	cfg, creds, err := resolveInfisical(root, "")
	if err != nil {
		return nil, err
	}

	// Plan: collect (project, env, key, value) tuples from local .env files.
	tuples, err := collectDotenvTuples(root, m)
	if err != nil {
		return nil, err
	}

	// Sync decision:
	//   --no-sync             → never sync, just flip manifest
	//   --yes                 → sync without asking (zero tuples = nothing to do, still flip manifest)
	//   else interactive ask  → default Yes if there are tuples
	doSync := !fl.noSync
	if doSync && !fl.yes && len(tuples) > 0 {
		ok, err := prompt.Confirm(
			fmt.Sprintf("发现 %d 个本地 .env 条目要推到 Infisical，继续？", len(tuples)),
			true,
			"是，同步并切换", "否，只切 manifest",
		)
		if err != nil {
			return nil, err
		}
		doSync = ok
	}

	if fl.dryRun {
		return &SwitchResult{
			Schema: "one-cli/env-switch/v1", From: current, To: target,
			ManifestPath: manifestPath, Synced: len(tuples),
			SkippedSync: !doSync,
		}, nil
	}

	// Important ordering: if we're syncing, push data BEFORE we flip the
	// manifest. If sync fails midway we leave the workspace on dotenv —
	// the user can retry without state confusion. With --no-sync we skip
	// straight to the flip.
	synced := 0
	conflicts := 0
	if doSync && len(tuples) > 0 {
		// Ensure Infisical project is bound (lazy auto-bind).
		if _, err := infisical.Init(ctx, root, infisical.InitInput{}); err != nil {
			return nil, err
		}
		for _, t := range tuples {
			_, err := infisical.Set(ctx, root, infisical.SetInput{
				Env:       t.env,
				Path:      t.path,
				Key:       t.key,
				Value:     t.value,
				Overwrite: fl.overwrite,
				Cfg:       cfg,
				Creds:     creds,
			})
			if err != nil {
				var cErr *output.Error
				if errors.As(err, &cErr) && cErr.Code == string(cliErrors.ENV_SET_OVERWRITE_REQUIRED) {
					conflicts++
					continue
				}
				// Hard fail. Manifest hasn't flipped yet, so workspace stays on
				// dotenv. User can fix and re-run.
				return nil, err
			}
			synced++
		}
		if conflicts > 0 {
			return nil, cliErrors.New(cliErrors.ENV_MIGRATE_CONFLICT,
				fmt.Sprintf("%d 个 key 在 Infisical 已存在且值不同；加 --overwrite 重跑以覆盖。", conflicts)).
				WithContext(map[string]any{
					"backend": target, "conflicts": conflicts, "synced": synced,
				})
		}
	}

	if err := writeEnvBackend(m, root, target); err != nil {
		return nil, err
	}

	return &SwitchResult{
		Schema: "one-cli/env-switch/v1", From: current, To: target,
		ManifestPath: manifestPath, Synced: synced,
		SkippedSync: !doSync,
	}, nil
}

// writeEnvBackend mutates m.Domains.Env.Kind and persists. The manifest
// keeps any existing Profile / Config (Infisical projectId etc.) so
// switching back and forth is non-destructive.
func writeEnvBackend(m *workspace.Manifest, root, target string) error {
	if m.Domains == nil {
		m.Domains = &workspace.WorkspaceDomains{}
	}
	if m.Domains.Env == nil {
		m.Domains.Env = &workspace.BackendRef{}
	}
	m.Domains.Env.Kind = target
	return workspace.WriteManifest(root, m)
}

// dotenvTuple is one (project, env, key, value) entry collected from
// local .env files. `path` is the Infisical folder path the value will
// land at (NormalizePath-ed before Set).
type dotenvTuple struct {
	project string
	env     string
	path    string
	key     string
	value   string
}

// collectDotenvTuples scans every project's .env / .env.<env> /
// .env.local / .env.<env>.local files (in load order; later overrides
// earlier) and returns the resolved kv set for every (project, env)
// combination listed in manifest.environments.names.
//
// .env.local and .env.<env>.local are intentionally consulted — they
// usually hold dev secrets that the user does want to migrate.
func collectDotenvTuples(root string, m *workspace.Manifest) ([]dotenvTuple, error) {
	var envs []string
	if m.Environments != nil {
		envs = append(envs, m.Environments.Names...)
	}
	if len(envs) == 0 {
		envs = []string{"dev"}
	}
	var out []dotenvTuple
	for _, p := range m.Projects {
		relDir := strings.TrimSpace(p.RelativeDir)
		projectDir := filepath.Join(root, relDir)
		if _, err := os.Stat(projectDir); err != nil {
			continue
		}
		path := "/" + strings.Trim(relDir, "/")
		if relDir == "" {
			path = "/"
		}
		for _, env := range envs {
			merged := map[string]string{}
			for _, file := range overlayChain(projectDir, env) {
				b, err := os.ReadFile(file)
				if err != nil {
					if os.IsNotExist(err) {
						continue
					}
					return nil, fmt.Errorf("read %s: %w", file, err)
				}
				for k, v := range dotenv.Parse(string(b)) {
					merged[k] = v
				}
			}
			keys := make([]string, 0, len(merged))
			for k := range merged {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				out = append(out, dotenvTuple{
					project: p.Name, env: env, path: path,
					key: k, value: merged[k],
				})
			}
		}
	}
	return out, nil
}

// overlayChain mirrors dotenv.overlayChain (which is unexported). Order
// matters: later files win. .env.local and .env.<env>.local are
// developer overrides usually carrying real values worth migrating.
func overlayChain(subDir, env string) []string {
	return []string{
		filepath.Join(subDir, ".env"),
		filepath.Join(subDir, ".env."+env),
		filepath.Join(subDir, ".env.local"),
		filepath.Join(subDir, ".env."+env+".local"),
	}
}
