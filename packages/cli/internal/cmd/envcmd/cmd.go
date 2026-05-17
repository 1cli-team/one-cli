// Package envcmd contributes `one env` to the root command via
// cliexts. Verbs switch on the backend selected for the workspace
// (dotenv / infisical) and call the respective backend package
// directly.
//
// Backend coverage:
//   - env/dotenv: Get / List / Set against the file overlay
//     (.env + .env.<env> + .env.local + .env.<env>.local). Pull
//     stays unsupported (dotenv has no remote).
//   - env/infisical: Get / Set / List / Pull against the default env
//     profile (machine-level credentials). Project binding happens
//     at `one create --env-provider infisical` time (auto-bind), or
//     lazily on the first env op when projectId is still empty.
//
// Environment selection (--env) is workspace-scoped: manifest.environments.names
// is the source of truth and manifest.environments.default is the fallback when
// no flag is passed. Names not in the list trip
// ENV_UNKNOWN_ENVIRONMENT for read verbs (get/list/pull) and prompt
// for confirmation on write (set), creating the entry on accept.
//
// Infisical project binding is
// no longer a separate user-facing step. Auto-bind covers create
// time; lazy auto-bind covers post-create profile changes. Tweaking
// environments/default is a manifest edit (rare, advanced).
package envcmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/cliexts"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/prompt"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets/dotenv"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets/infisical"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func init() {
	cliexts.Register("env", buildContributions)
}

func buildContributions() []*cobra.Command {
	parent := &cobra.Command{
		Use: "env",
		Long: `本命令操作工作区当前选定的 env 后端
（manifest.domains.env.kind，dotenv 或 infisical）。

子命令：
  one env get  <KEY>                  读取一个环境变量值
  one env set  <KEY[=VALUE]> [VALUE]  写一个环境变量值
  one env list                        列出所有 KEY
  one env pull                        从远端拉取并写入本地 .env（仅 infisical）
  one env switch <backend>            切换工作区后端（dotenv ↔ infisical），可选数据同步

环境选择：
  manifest.environments.names 是工作区维护的环境名列表（默认 dev / staging / prod）。
  --env 不传则使用 manifest.environments.default。
  对 dotenv 后端而言，每个环境对应 <project>/.env.<env> 覆盖文件，加载顺序为
  .env → .env.<env> → .env.local → .env.<env>.local（后者覆盖前者）。
  对 infisical 后端而言，--env 直接选 Infisical 项目内对应的环境分区。

项目选择（dotenv）：
  -p / --project <selector> 接受两种形式：
    one env set FOO=bar -p web                  # manifest 里 projects[].name
    one env set FOO=bar -p apps/web             # 相对工作区根的路径
  不传 -p 时按 cwd 推断（cd 进项目后直接 set 即可）。

Infisical 项目绑定：
  在 one create --env-provider infisical 时自动完成（前提是已配 env/infisical profile）。
  profile 暂未配置 / 网络不通时，create 仍然成功；首次跑 env 命令会再尝试一次自动绑定。
  自动绑定彻底失败的情况下，错误信息会指向 one configure add env/infisical。

凭据 scope：machine 级（siteUrl + clientId/secret）走 one configure add env/infisical，
不在 manifest 中。`,
	}
	parent.AddCommand(
		newGetCmd(),
		newSetCmd(),
		newListCmd(),
		newPullCmd(),
		newSwitchCmd(),
	)
	i18n.MarkShort(parent, "env.short")
	return []*cobra.Command{parent}
}

// requireEnv resolves the active env backend for the workspace.
// dotenv is the implicit default when manifest.domains.env.kind is empty —
// every workspace has a filesystem, so the file-based backend is
// always usable without configuration. Infisical is only selected
// when manifest.domains.env.kind == "infisical" (set by `one env switch`
// or by picking Infisical at `one create` time).
func requireEnv(projectRoot string) (string, error) {
	m, err := workspace.ReadManifest(projectRoot)
	if err != nil {
		return "", err
	}
	backend := workspace.EnvBackend(m)
	if backend == "" {
		return workspace.EnvBackendDotenv, nil
	}
	return backend, nil
}

// verbNotSupported returns a structured envelope for verbs the dotenv
// backend doesn't implement.
func verbNotSupported(verb string) error {
	return cliErrors.New(cliErrors.BACKEND_VERB_NOT_SUPPORTED,
		fmt.Sprintf("env/dotenv 后端不支持 `one env %s`（dotenv 没有远端 / 无 schema）。切到 env/infisical 即可使用。", verb)).
		WithContext(map[string]any{
			"domain":  "env",
			"backend": "dotenv",
			"verb":    verb,
		})
}

// resolveInfisical loads the default env profile and converts it into
// the infisical-package types. Returns (nil, nil, nil) when no profile
// is configured (callers fall back to the legacy manifest path inside
// the infisical package).
func resolveInfisical(projectRoot, profileFlag string) (*infisical.WorkspaceConfig, *infisical.Credentials, error) {
	workspaceID := ""
	if m, err := workspace.ReadManifest(projectRoot); err == nil {
		workspaceID = workspace.WorkspaceID(m)
	}
	resolved, err := profile.Resolve(profile.ResolveInput{
		Domain:       profile.DomainEnv,
		Backend:      "infisical",
		FlagOverride: profileFlag,
		WorkspaceID:  workspaceID,
	})
	if err != nil {
		if cliErr, ok := err.(interface{ ErrorCode() string }); ok &&
			cliErr.ErrorCode() == "PROFILE_NONE_CONFIGURED" {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	if resolved.Profile.Infisical == nil {
		return nil, nil, nil
	}
	ip := resolved.Profile.Infisical
	cfg := &infisical.WorkspaceConfig{
		SiteURL: ip.SiteURL,
	}
	var creds *infisical.Credentials
	if ip.Credentials != nil {
		creds = &infisical.Credentials{
			ClientID:     ip.Credentials.ClientID,
			ClientSecret: ip.Credentials.ClientSecret,
		}
	}
	return cfg, creds, nil
}

// ensureInfisicalBound is the lazy auto-bind path. When a workspace
// has domains.env.kind = "infisical" but domains.env.config.projectId is still empty
// (because create-time auto-bind couldn't reach Infisical, e.g. no
// profile yet), the next env op tries to bind once. Success persists
// projectId to the manifest; failure leaves the manifest as-is and
// returns a structured error pointing at profile setup.
func ensureInfisicalBound(ctx context.Context, projectRoot string) error {
	m, err := workspace.ReadManifest(projectRoot)
	if err != nil {
		return err
	}
	cfg, _ := infisical.LoadWorkspaceConfig(projectRoot)
	if cfg == nil || strings.TrimSpace(cfg.ProjectID) != "" {
		_ = m
		return nil
	}
	_, err = infisical.Init(ctx, projectRoot, infisical.InitInput{})
	return err
}

// resolveInfisicalFolderPath turns the `-p` selector + cwd into the
// Infisical folder path that get/set/list should target.
//
// Resolution order:
//
//  1. selector ≠ "" → look up subproject via name / relativeDir,
//     compute its Infisical path (respects per-project env.path
//     override). Falls back to treating the selector as a raw folder
//     path when no subproject matches — lets advanced users address
//     ad-hoc folders directly (`-p /shared`).
//  2. selector == "" + cwd inside a subproject → that subproject's
//     Infisical path.
//  3. selector == "" + cwd outside any subproject → workspace root
//     path (domains.env.config.rootPath, default "/").
//
// Per-subproject env overrides are read from the manifest. The cfg
// arg may be nil — we synthesize a minimal config from the manifest
// in that case.
func resolveInfisicalFolderPath(projectRoot string, cfg *infisical.WorkspaceConfig, selector string) (string, error) {
	if cfg == nil {
		// Build a minimal cfg from the manifest so ResolveSubprojectPath
		// has a rootPath to anchor on.
		if existing, err := infisical.LoadWorkspaceConfig(projectRoot); err == nil && existing != nil {
			cfg = &infisical.WorkspaceConfig{RootPath: existing.RootPath}
		} else {
			cfg = &infisical.WorkspaceConfig{}
		}
	}
	selector = strings.TrimSpace(selector)
	if selector != "" {
		sub, err := workspace.ResolveProjectFromSelector(projectRoot, selector)
		if err != nil {
			return "", err
		}
		if sub != nil {
			override, err := infisical.LoadSubprojectConfig(projectRoot, sub.RelativeDir)
			if err != nil {
				return "", err
			}
			return infisical.ResolveSubprojectPath(cfg, sub, override).Path, nil
		}
		// Unknown selector: if it looks like an absolute folder path,
		// honour it verbatim. Otherwise surface a clear error pointing
		// to declared subproject names.
		if strings.HasPrefix(selector, "/") {
			return infisical.NormalizePath(selector), nil
		}
		m, _ := workspace.ReadManifest(projectRoot)
		return "", cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND,
			"找不到名字或路径匹配 "+selector+" 的项目。已声明: "+
				strings.Join(workspace.ProjectNames(m), ", "))
	}
	// No selector: try cwd → subproject.
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	sub, err := workspace.ResolveProjectFromCWD(projectRoot, cwd)
	if err != nil {
		return "", err
	}
	if sub != nil {
		override, err := infisical.LoadSubprojectConfig(projectRoot, sub.RelativeDir)
		if err != nil {
			return "", err
		}
		return infisical.ResolveSubprojectPath(cfg, sub, override).Path, nil
	}
	// Workspace root.
	return infisical.NormalizePath(cfg.RootPathOrDefault()), nil
}

func newGetCmd() *cobra.Command {
	var sub, env, profileFlag string
	cmd := &cobra.Command{
		Use:   "get <KEY>",
		Short: "读取一个环境变量值",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := workspace.ResolveProjectRoot("")
			if err != nil {
				return err
			}
			backend, err := requireEnv(root)
			if err != nil {
				return err
			}
			resolvedEnv, _, err := secrets.ResolveEnvName(root, env, false)
			if err != nil {
				return err
			}
			switch backend {
			case workspace.EnvBackendDotenv:
				res, err := dotenv.Get(dotenv.GetInput{
					ProjectRoot:    root,
					SubprojectPath: sub,
					Env:            resolvedEnv,
					Key:            args[0],
				})
				if err != nil {
					return err
				}
				output.Emit(res)
				return nil
			case workspace.EnvBackendInfisical:
				if err := ensureInfisicalBound(cmd.Context(), root); err != nil {
					return err
				}
				cfg, creds, err := resolveInfisical(root, profileFlag)
				if err != nil {
					return err
				}
				folder, err := resolveInfisicalFolderPath(root, cfg, sub)
				if err != nil {
					return err
				}
				res, err := infisical.Get(cmd.Context(), root, infisical.GetInput{
					Env:   resolvedEnv,
					Path:  folder,
					Key:   args[0],
					Cfg:   cfg,
					Creds: creds,
				})
				if err != nil {
					return err
				}
				output.Emit(res)
				return nil
			}
			return verbNotSupported("get")
		},
	}
	cmd.Flags().StringVarP(&sub, "project", "p", "", "项目名（manifest.projects[].name）或相对路径；默认从 cwd 推导")
	cmd.Flags().StringVar(&env, "env", "", "环境名（缺省 manifest.environments.default）")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "一次性使用指定 profile（不改 default）")
	return cmd
}

func newSetCmd() *cobra.Command {
	var (
		sub, env, profileFlag string
		yes                   bool
	)
	cmd := &cobra.Command{
		Use:   "set <KEY[=VALUE]> [VALUE]",
		Short: "写一个环境变量值（dotenv 写到 .env / .env.<env>，infisical 写到对应环境）",
		Long: `写一个环境变量值。两种调用形式都接受：

  one env set KEY VALUE        # 两个位置参数
  one env set KEY=VALUE        # 单参数 KEY=VALUE
  one env set KEY              # 等价于 KEY=（空值）

VALUE 含等号时，单参形式按第一个 = 拆分：one env set DSN=postgres://x=y → KEY="DSN", VALUE="postgres://x=y"。`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := workspace.ResolveProjectRoot("")
			if err != nil {
				return err
			}
			backend, err := requireEnv(root)
			if err != nil {
				return err
			}

			key, value := parseSetArgs(args)

			// Cross-backend validation: enforce the POSIX env-var
			// pattern on the key. dotenv would otherwise silently
			// write keys that `source .env` rejects (anything with a
			// `.` or other non-identifier char); Infisical accepts
			// looser keys but `one run` injects them as env vars, so
			// the same constraint applies to keep the data path
			// portable.
			if err := secrets.AssertValidKey(key); err != nil {
				return err
			}

			// set permits unknown env names — that's the implicit-create
			// path. Confirm before adding to the manifest.
			resolvedEnv, declared, err := secrets.ResolveEnvName(root, env, true)
			if err != nil {
				return err
			}
			created := false
			if resolvedEnv != "" && !contains(declared, resolvedEnv) {
				if err := confirmCreateEnv(resolvedEnv, yes); err != nil {
					return err
				}
				added, err := workspace.EnsureEnvironment(root, resolvedEnv)
				if err != nil {
					return err
				}
				created = added
			}

			// Resolve which project (if any) we're writing for. Used
			// both for the backend write itself and to record the KEY
			// in manifest.projects[i].env.keys (so `one env check`
			// can later compare envs for completeness).
			subProject, err := resolveSetSubproject(root, sub)
			if err != nil {
				return err
			}

			recordKey := func() error {
				if subProject != nil {
					return workspace.RecordProjectEnvKey(root, subProject.Name, key)
				}
				return workspace.RecordWorkspaceEnvKey(root, key)
			}

			switch backend {
			case workspace.EnvBackendDotenv:
				subPath := ""
				if subProject != nil {
					subPath = subProject.RelativeDir
				}
				res, err := dotenv.Set(dotenv.SetInput{
					ProjectRoot:    root,
					SubprojectPath: subPath,
					Env:            resolvedEnv,
					Key:            key,
					Value:          value,
					Overwrite:      yes,
				})
				if err != nil {
					return err
				}
				if err := recordKey(); err != nil {
					return err
				}
				output.Emit(envSetEnvelope{
					SetResult:          res,
					CreatedEnvironment: created,
				})
				return nil
			case workspace.EnvBackendInfisical:
				if err := ensureInfisicalBound(cmd.Context(), root); err != nil {
					return err
				}
				cfg, creds, err := resolveInfisical(root, profileFlag)
				if err != nil {
					return err
				}
				folder, err := resolveInfisicalFolderPath(root, cfg, sub)
				if err != nil {
					return err
				}
				res, err := infisical.Set(cmd.Context(), root, infisical.SetInput{
					Env:       resolvedEnv,
					Path:      folder,
					Key:       key,
					Value:     value,
					Overwrite: yes,
					Cfg:       cfg,
					Creds:     creds,
				})
				if err != nil {
					return err
				}
				if err := recordKey(); err != nil {
					return err
				}
				output.Emit(infisicalSetEnvelope{
					SetResult:          res,
					CreatedEnvironment: created,
				})
				return nil
			}
			return verbNotSupported("set")
		},
	}
	cmd.Flags().StringVarP(&sub, "project", "p", "", "项目名（manifest.projects[].name）或相对路径；默认从 cwd 推导")
	cmd.Flags().StringVar(&env, "env", "", "环境名（缺省 manifest.environments.default）")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "一次性使用指定 profile（不改 default）")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "非交互模式：覆盖已存在值 / 自动确认创建新环境")
	return cmd
}

// resolveSetSubproject figures out which manifest subproject a set
// invocation targets. Returns nil (no error) when the user is writing
// at workspace-root scope — i.e. selector empty + cwd not inside any
// declared subproject. The returned subproject's Name is used as the
// key for manifest.projects[i].env.keys bookkeeping.
//
// When selector is non-empty but doesn't match any project (e.g.
// `-p shared` with no such name in manifest), returns nil + nil error
// so set can still proceed against a raw path / Infisical folder; the
// keys-tracking step simply skips that case.
func resolveSetSubproject(projectRoot, selector string) (*workspace.Project, error) {
	if strings.TrimSpace(selector) != "" {
		return workspace.ResolveProjectFromSelector(projectRoot, selector)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return workspace.ResolveProjectFromCWD(projectRoot, cwd)
}

// parseSetArgs accepts both `KEY VALUE` and `KEY=VALUE` invocations and
// returns (key, value). When the single-arg form contains `=`, the first
// `=` is the split point — values may legitimately contain `=` (URLs,
// JWTs, etc.), so only the leading separator is special. When two args
// are passed, the second always wins (we don't second-guess intent).
func parseSetArgs(args []string) (string, string) {
	if len(args) == 2 {
		return args[0], args[1]
	}
	first := args[0]
	if idx := strings.IndexByte(first, '='); idx > 0 {
		return first[:idx], first[idx+1:]
	}
	return first, ""
}

// envSetEnvelope wraps the dotenv set result with a flag indicating
// whether this call appended a new entry to manifest.environments.names.
type envSetEnvelope struct {
	*dotenv.SetResult
	CreatedEnvironment bool `json:"created_environment,omitempty"`
}

// infisicalSetEnvelope mirrors envSetEnvelope for the infisical
// backend so the JSON shape stays consistent across backends.
type infisicalSetEnvelope struct {
	*infisical.SetResult
	CreatedEnvironment bool `json:"created_environment,omitempty"`
}

func confirmCreateEnv(name string, yes bool) error {
	if yes || !output.IsTTY() {
		return nil
	}
	ok, err := prompt.Confirm(
		fmt.Sprintf("环境 %q 不在 manifest.environments.names 中。要创建并继续吗？", name),
		false, "", "")
	if err != nil {
		return err
	}
	if !ok {
		return cliErrors.New(cliErrors.PROMPT_CANCELLED, "已取消创建新环境。").
			WithExit0()
	}
	return nil
}

func contains(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

func newListCmd() *cobra.Command {
	var sub, env, profileFlag string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "列出所有 KEY",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := workspace.ResolveProjectRoot("")
			if err != nil {
				return err
			}
			backend, err := requireEnv(root)
			if err != nil {
				return err
			}
			resolvedEnv, _, err := secrets.ResolveEnvName(root, env, false)
			if err != nil {
				return err
			}
			switch backend {
			case workspace.EnvBackendDotenv:
				res, err := dotenv.List(dotenv.ListInput{
					ProjectRoot:    root,
					SubprojectPath: sub,
					Env:            resolvedEnv,
				})
				if err != nil {
					return err
				}
				output.Emit(res)
				return nil
			case workspace.EnvBackendInfisical:
				if err := ensureInfisicalBound(cmd.Context(), root); err != nil {
					return err
				}
				cfg, creds, err := resolveInfisical(root, profileFlag)
				if err != nil {
					return err
				}
				folder, err := resolveInfisicalFolderPath(root, cfg, sub)
				if err != nil {
					return err
				}
				res, err := infisical.List(cmd.Context(), root, infisical.ListInput{
					Env:   resolvedEnv,
					Path:  folder,
					Cfg:   cfg,
					Creds: creds,
				})
				if err != nil {
					return err
				}
				output.Emit(res)
				return nil
			}
			return verbNotSupported("list")
		},
	}
	cmd.Flags().StringVarP(&sub, "project", "p", "", "项目名（manifest.projects[].name）或相对路径；默认从 cwd 推导")
	cmd.Flags().StringVar(&env, "env", "", "环境名（缺省 manifest.environments.default）")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "一次性使用指定 profile（不改 default）")
	return cmd
}

func newPullCmd() *cobra.Command {
	var (
		env, project, profileFlag string
		force, dryRun             bool
	)
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "从远端拉取环境变量写入本地 .env（仅 infisical）",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := workspace.ResolveProjectRoot("")
			if err != nil {
				return err
			}
			backend, err := requireEnv(root)
			if err != nil {
				return err
			}
			if backend != workspace.EnvBackendInfisical {
				return verbNotSupported("pull")
			}
			if err := ensureInfisicalBound(cmd.Context(), root); err != nil {
				return err
			}
			resolvedEnv, _, err := secrets.ResolveEnvName(root, env, false)
			if err != nil {
				return err
			}
			cfg, creds, err := resolveInfisical(root, profileFlag)
			if err != nil {
				return err
			}
			var res *infisical.PullResult
			if err := prompt.Spin("正在从远端拉取环境变量并写入 .env", func() error {
				r, err := infisical.Pull(cmd.Context(), root, infisical.PullInput{
					Env:     resolvedEnv,
					Project: project,
					Force:   force,
					DryRun:  dryRun,
					Cfg:     cfg,
					Creds:   creds,
				})
				if err != nil {
					return err
				}
				res = r
				return nil
			}); err != nil {
				return err
			}
			if res != nil {
				output.Emit(res)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&env, "env", "", "环境名（缺省 manifest.environments.default）")
	cmd.Flags().StringVarP(&project, "project", "p", "", "限定一个项目（名字或相对路径；缺省拉所有）")
	cmd.Flags().BoolVar(&force, "force", false, "覆盖已存在且内容不一致的 .env 文件")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "只汇报会写哪些 key，不实际落盘")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "一次性使用指定 profile（不改 default）")
	return cmd
}
