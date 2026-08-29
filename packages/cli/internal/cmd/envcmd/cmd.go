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
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/cliexts"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/helpui"
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
		Use:     "env",
		Long:    i18n.T("env.tip"),
		Example: "  one env\n  one env set DATABASE_URL\n  one env list",
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := workspace.ResolveProjectRoot("")
			if err != nil {
				return err
			}
			if !workspace.HasManifest(root) {
				return cliErrors.New(cliErrors.NOT_ONE_PROJECT, i18n.T("env.workspace_required"))
			}
			summary, err := buildEnvSummary(root)
			if err != nil {
				return err
			}
			output.Emit(&summary)
			return nil
		},
	}
	children := []*cobra.Command{newGetCmd(), newSetCmd(), newListCmd(), newPullCmd(), newSwitchCmd()}
	for _, child := range children {
		helpui.MarkAdvanced(child, "profile")
	}
	parent.AddCommand(children...)
	i18n.MarkShort(parent, "env.short")
	i18n.MarkLong(parent, "env.tip")
	return []*cobra.Command{parent}
}

type envSummary struct {
	Schema                string   `json:"schema"`
	Source                string   `json:"source"`
	DefaultEnvironment    string   `json:"default_environment"`
	AvailableEnvironments []string `json:"available_environments"`
	Scope                 string   `json:"scope"`
	Project               string   `json:"project,omitempty"`
	Commands              []string `json:"commands"`
}

func buildEnvSummary(root string) (envSummary, error) {
	m, err := workspace.ReadManifest(root)
	if err != nil {
		return envSummary{}, err
	}
	source := workspace.EnvBackend(m)
	if source == "" {
		source = workspace.EnvBackendDotenv
	}
	defaultEnv := "dev"
	environments := append([]string(nil), workspace.DefaultEnvironments...)
	if m.Environments != nil {
		if strings.TrimSpace(m.Environments.Default) != "" {
			defaultEnv = m.Environments.Default
		}
		if len(m.Environments.Names) > 0 {
			environments = append([]string(nil), m.Environments.Names...)
		}
	}
	scope := "workspace"
	project := ""
	if cwd, cwdErr := os.Getwd(); cwdErr == nil {
		if p, resolveErr := workspace.ResolveProjectFromCWD(root, cwd); resolveErr == nil && p != nil {
			scope = "project"
			project = p.Name
		}
	}
	return envSummary{
		Schema:                "one-cli/env-summary/v1",
		Source:                source,
		DefaultEnvironment:    defaultEnv,
		AvailableEnvironments: environments,
		Scope:                 scope,
		Project:               project,
		Commands:              []string{"one env set <KEY>", "one env list", "one env get <KEY>"},
	}, nil
}

func (s *envSummary) RenderTTY(w io.Writer) {
	if s == nil {
		return
	}
	source := s.Source
	if source == workspace.EnvBackendDotenv {
		source = i18n.T("env.source_dotenv")
	}
	scope := i18n.T("env.scope_workspace")
	if s.Scope == "project" {
		scope = i18n.Tf("env.scope_project", s.Project)
	}
	fmt.Fprintf(w, i18n.T("env.summary_source")+"\n", source)
	fmt.Fprintf(w, i18n.T("env.summary_default")+"\n", s.DefaultEnvironment)
	fmt.Fprintf(w, i18n.T("env.summary_available")+"\n", strings.Join(s.AvailableEnvironments, ", "))
	fmt.Fprintf(w, i18n.T("env.summary_scope")+"\n", scope)
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("env.common_commands"))
	for _, command := range s.Commands {
		fmt.Fprintln(w, "  "+command)
	}
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
	cmd.Flags().StringVarP(&sub, "project", "p", "", i18n.T("env.flag.project"))
	cmd.Flags().StringVar(&env, "env", "", i18n.T("env.flag.environment"))
	cmd.Flags().StringVar(&profileFlag, "profile", "", i18n.T("env.flag.profile"))
	markEnvFlagUsage(cmd, "project", "env", "profile")
	i18n.MarkShort(cmd, "env.get.short")
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
		Long:  i18n.T("env.set.tip"),
		Args:  cobra.RangeArgs(1, 2),
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
			if !setValueProvided(args) {
				if !output.CanPrompt() {
					return cliErrors.New(cliErrors.ENV_SET_VALUE_REQUIRED,
						i18n.T("env.value_required"))
				}
				value, err = prompt.Password(i18n.Tf("env.prompt_value", key), func(v string) error {
					if v == "" {
						return fmt.Errorf("%s", i18n.T("env.value_empty"))
					}
					return nil
				})
				if err != nil {
					return err
				}
			}

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
			subProject, err := resolveSetSubprojectForSet(root, sub, output.CanPrompt())
			if err != nil {
				return err
			}
			targetSelector := resolveSetTargetSelector(sub, subProject)

			recordKey := func() error {
				if subProject != nil {
					return workspace.RecordProjectEnvKey(root, subProject.Name, key)
				}
				if targetSelector == "" {
					return workspace.RecordWorkspaceEnvKey(root, key)
				}
				// Raw paths are intentionally not represented in the manifest:
				// they do not identify a declared project or workspace scope.
				return nil
			}

			switch backend {
			case workspace.EnvBackendDotenv:
				setInput := dotenv.SetInput{
					ProjectRoot:    root,
					SubprojectPath: targetSelector,
					Env:            resolvedEnv,
					Key:            key,
					Value:          value,
					Overwrite:      yes,
				}
				res, err := dotenv.Set(setInput)
				if retry, confirmErr := confirmOverwrite(err, key, yes); confirmErr != nil {
					return confirmErr
				} else if retry {
					setInput.Overwrite = true
					res, err = dotenv.Set(setInput)
				}
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
				folder, err := resolveInfisicalFolderPath(root, cfg, targetSelector)
				if err != nil {
					return err
				}
				setInput := infisical.SetInput{
					Env:       resolvedEnv,
					Path:      folder,
					Key:       key,
					Value:     value,
					Overwrite: yes,
					Cfg:       cfg,
					Creds:     creds,
				}
				res, err := infisical.Set(cmd.Context(), root, setInput)
				if retry, confirmErr := confirmOverwrite(err, key, yes); confirmErr != nil {
					return confirmErr
				} else if retry {
					setInput.Overwrite = true
					res, err = infisical.Set(cmd.Context(), root, setInput)
				}
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
	cmd.Flags().StringVarP(&sub, "project", "p", "", i18n.T("env.flag.project"))
	cmd.Flags().StringVar(&env, "env", "", i18n.T("env.flag.environment"))
	cmd.Flags().StringVar(&profileFlag, "profile", "", i18n.T("env.flag.profile"))
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, i18n.T("env.flag.yes"))
	markEnvFlagUsage(cmd, "project", "env", "profile", "yes")
	i18n.MarkShort(cmd, "env.set.short")
	i18n.MarkLong(cmd, "env.set.tip")
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

func resolveSetSubprojectForSet(projectRoot, selector string, interactive bool) (*workspace.Project, error) {
	p, err := resolveSetSubproject(projectRoot, selector)
	if err != nil || p != nil || strings.TrimSpace(selector) != "" || !interactive {
		return p, err
	}
	m, err := workspace.ReadManifest(projectRoot)
	if err != nil || len(m.Projects) == 0 {
		return nil, err
	}
	options := []prompt.Option[string]{{Label: i18n.T("env.scope_workspace_shared"), Value: ""}}
	for _, project := range m.Projects {
		options = append(options, prompt.Option[string]{
			Label: i18n.Tf("env.scope_project_option", project.Name),
			Value: project.Name,
		})
	}
	chosen, err := prompt.Select(i18n.T("env.prompt_scope"), options)
	if err != nil || chosen == "" {
		return nil, err
	}
	return workspace.ResolveProjectFromSelector(projectRoot, chosen)
}

// resolveSetTargetSelector normalizes the backend target after project-scope
// resolution. A declared or interactively selected project always uses its
// manifest relativeDir; an unmatched explicit selector remains a raw backend
// path; an empty selector with no project is workspace scope.
func resolveSetTargetSelector(selector string, project *workspace.Project) string {
	if project != nil {
		return project.RelativeDir
	}
	return strings.TrimSpace(selector)
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

func setValueProvided(args []string) bool {
	return len(args) >= 2 || (len(args) == 1 && strings.IndexByte(args[0], '=') > 0)
}

func confirmOverwrite(setErr error, key string, yes bool) (bool, error) {
	if setErr == nil {
		return false, nil
	}
	coded, ok := setErr.(interface{ ErrorCode() string })
	if !ok || coded.ErrorCode() != string(cliErrors.ENV_SET_OVERWRITE_REQUIRED) {
		return false, setErr
	}
	if yes || !output.CanPrompt() {
		return false, setErr
	}
	overwrite, err := prompt.Confirm(i18n.Tf("env.prompt_overwrite", key), false,
		i18n.T("common.overwrite"), i18n.T("common.cancel"))
	if err != nil {
		return false, err
	}
	if !overwrite {
		return false, cliErrors.New(cliErrors.PROMPT_CANCELLED, i18n.T("common.cancelled")).WithExit0()
	}
	return true, nil
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
	if yes || !output.CanPrompt() {
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
	cmd.Flags().StringVarP(&sub, "project", "p", "", i18n.T("env.flag.project"))
	cmd.Flags().StringVar(&env, "env", "", i18n.T("env.flag.environment"))
	cmd.Flags().StringVar(&profileFlag, "profile", "", i18n.T("env.flag.profile"))
	markEnvFlagUsage(cmd, "project", "env", "profile")
	i18n.MarkShort(cmd, "env.list.short")
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
			if err := prompt.Spin(i18n.T("env.pull.running"), func() error {
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
	cmd.Flags().StringVar(&env, "env", "", i18n.T("env.flag.environment"))
	cmd.Flags().StringVarP(&project, "project", "p", "", i18n.T("env.flag.pull_project"))
	cmd.Flags().BoolVar(&force, "force", false, i18n.T("env.flag.force"))
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, i18n.T("env.flag.dry_run"))
	cmd.Flags().StringVar(&profileFlag, "profile", "", i18n.T("env.flag.profile"))
	markEnvFlagUsage(cmd, "env", "project", "force", "dry-run", "profile")
	i18n.MarkShort(cmd, "env.pull.short")
	return cmd
}

func markEnvFlagUsage(cmd *cobra.Command, names ...string) {
	keys := map[string]string{
		"project": "env.flag.project", "env": "env.flag.environment",
		"profile": "env.flag.profile", "yes": "env.flag.yes",
		"force": "env.flag.force", "dry-run": "env.flag.dry_run",
	}
	if cmd.Name() == "pull" {
		keys["project"] = "env.flag.pull_project"
	}
	for _, name := range names {
		i18n.MarkFlagUsage(cmd, name, keys[name])
	}
}
