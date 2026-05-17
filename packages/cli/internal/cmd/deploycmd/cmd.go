// Package deploycmd contributes `one deploy` to the root command via
// cliexts. Verbs iterate per-project deploy targets — each
// subproject's deploy backend is configured in the manifest, so a
// workspace can mix front-end (s3 / vercel) and back-end (kustomize)
// deployments in one command.
//
// Profile support: each verb takes --profile to one-shot override the
// default profile; the cobra layer resolves the machine-level profile
// per subproject and hands the resolved profile to a deploy.Provider
// loaded from the deploy registry.
//
// Per-workspace and per-project profile choices live in
// ~/.config/one/config.json#workspaces. --profile overrides at runtime;
// otherwise resolution falls through to workspace bindings and then the
// machine default profile.
package deploycmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/cliexts"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/deploy"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/kustomize"
	// Blank-import the remaining providers so their init() registers
	// with the deploy package's process-global registry. Adding a new
	// platform = drop a new blank import here.
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/infra/cloudflare"
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/infra/edgeone"
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/infra/s3compat"
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/infra/vercel"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/prompt"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func init() {
	cliexts.Register("deploy", buildContributions)
}

func buildContributions() []*cobra.Command {
	return []*cobra.Command{newDeployCmd()}
}

// deployTarget is one project's deploy job: which backend and the
// workspace-view of the project.
type deployTarget struct {
	Project        workspace.Project
	Backend        string // bare backend id from manifest.projects[i].deploy.target
	Toolchain      string
	TemplateID     string
	PackageManager string
}

// deployTargets enumerates per-project deploy jobs the manifest
// declares. Returns an empty slice (no error) when no subproject has a
// deploy backend configured — caller decides whether that's an error.
func deployTargets(projectRoot string) ([]deployTarget, error) {
	if !workspace.HasManifest(projectRoot) {
		return nil, cliErrors.New(cliErrors.NOT_ONE_PROJECT,
			"未检测到 One CLI 项目，请在工作区根目录执行。")
	}
	m, err := workspace.ReadManifest(projectRoot)
	if err != nil {
		return nil, err
	}
	var out []deployTarget
	for _, sub := range m.Projects {
		sel := workspace.DeployForProject(m, sub.Name)
		if sel.Backend == "" {
			continue
		}
		out = append(out, deployTarget{
			Project: workspace.Project{
				Name:           sub.Name,
				RelativeDir:    sub.RelativeDir,
				TargetDir:      filepath.Join(projectRoot, filepath.FromSlash(sub.RelativeDir)),
				Toolchain:      sub.Toolchain,
				PackageManager: sub.PackageManager,
				TemplateID:     sub.TemplateID,
			},
			Backend:        sel.Backend,
			Toolchain:      sub.Toolchain,
			TemplateID:     sub.TemplateID,
			PackageManager: sub.PackageManager,
		})
	}
	return out, nil
}

// normalizeProjectSelector turns the user-facing -p value (which may be a
// manifest name or a relative path) into a canonical subproject name, so
// downstream filters that key on Name need no further changes. Empty
// selector → empty result (means "all").
func normalizeProjectSelector(projectRoot, selector string) (string, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return "", nil
	}
	sub, err := workspace.ResolveProjectFromSelector(projectRoot, selector)
	if err != nil {
		return "", err
	}
	if sub == nil {
		return selector, nil
	}
	return sub.Name, nil
}

func filterDeployTargets(targets []deployTarget, subproject string) ([]deployTarget, error) {
	subproject = strings.TrimSpace(subproject)
	if subproject == "" {
		return targets, nil
	}
	for _, target := range targets {
		if target.Project.Name == subproject {
			return []deployTarget{target}, nil
		}
	}
	return nil, cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND,
		fmt.Sprintf("没有名为 %s 且声明了 deploy 后端的项目", subproject)).
		WithContext(map[string]any{
			"subproject":               subproject,
			"deploy_enabled_projects":  deployTargetNames(targets),
			"configured_deploy_target": false,
		})
}

func deployTargetNames(targets []deployTarget) []string {
	names := make([]string, 0, len(targets))
	for _, target := range targets {
		names = append(names, target.Project.Name)
	}
	return names
}

// resolveDeployProfile loads the profile that applies to one
// subproject and returns the resolved profile struct (or nil if none
// configured). Profile storage splits per (domain, backend), so the
// resolver is called with the project's declared backend — same
// profile name in deploy/aliyun-oss, deploy/kustomize, deploy/vercel
// never collides.
func resolveDeployProfile(projectRoot, profileFlag string, target deployTarget) (*profile.Resolved, error) {
	workspaceID := ""
	if m, err := workspace.ReadManifest(projectRoot); err == nil {
		workspaceID = workspace.WorkspaceID(m)
	}
	resolved, err := profile.Resolve(profile.ResolveInput{
		Domain:       profile.DomainDeploy,
		Backend:      target.Backend,
		FlagOverride: profileFlag,
		WorkspaceID:  workspaceID,
		ProjectName:  target.Project.Name,
	})
	if err != nil {
		if cliErr, ok := err.(interface{ ErrorCode() string }); ok &&
			cliErr.ErrorCode() == "PROFILE_NONE_CONFIGURED" {
			return nil, nil
		}
		return nil, err
	}
	return resolved, nil
}

const defaultCloudflareProfileName = "cf-prod"

func ensureInteractiveCloudflareProfile(profileFlag string, target deployTarget, resolved *profile.Resolved) (*profile.Resolved, error) {
	if resolved != nil || target.Backend != workspace.DeployBackendCloudflare {
		return resolved, nil
	}
	if strings.TrimSpace(profileFlag) != "" || !output.CanPrompt() {
		return resolved, nil
	}
	token, err := prompt.Password(
		"Cloudflare API token（需要 Account / Workers Scripts / Edit；使用 D1 时还需要 Account / D1 / Edit）",
		requireNonEmpty,
	)
	if err != nil {
		return nil, err
	}
	accountID, err := prompt.Text(
		"Account ID（可选；多账号 token 必填；可从 Cloudflare Dashboard URL 或右侧 Account ID 复制）",
		"",
		nil,
	)
	if err != nil {
		return nil, err
	}
	cp := &profile.CloudflareProfile{
		AccountID: strings.TrimSpace(accountID),
		Credentials: &profile.CloudflareCredentials{
			APIToken: strings.TrimSpace(token),
		},
	}
	p := profile.Profile{
		Backend:    workspace.DeployBackendCloudflare,
		Cloudflare: cp,
	}
	if _, err := profile.Upsert(profile.DomainDeploy, workspace.DeployBackendCloudflare, defaultCloudflareProfileName, p, true); err != nil {
		return nil, err
	}
	prompt.Step("Cloudflare profile saved → " + defaultCloudflareProfileName)
	return &profile.Resolved{
		Name:       defaultCloudflareProfileName,
		Profile:    p,
		Source:     "prompt",
		CredSource: profile.SourceFile,
	}, nil
}

func requireNonEmpty(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("不能为空")
	}
	return nil
}

func autoBuildBeforeDeploy(ctx context.Context, in deploy.ApplyInput, target deployTarget, m *workspace.Manifest, dryRun bool) ([]string, error) {
	if !shouldAutoBuild(target) {
		return nil, nil
	}
	scripts, err := readPackageScripts(projectDirForTarget(in, target))
	if err != nil {
		return nil, err
	}
	if _, ok := scripts["build"]; !ok {
		return nil, nil
	}
	argv := nodeBuildArgv(packageManagerForTarget(target, m))
	line := strings.Join(argv, " ")
	if dryRun {
		return []string{line}, nil
	}
	return nil, prompt.Spin(fmt.Sprintf("正在构建项目 %s", target.Project.Name), func() error {
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Dir = projectDirForTarget(in, target)
		cmd.Stdout = in.Stdout
		cmd.Stderr = in.Stderr
		cmd.Env = augmentDeployBuildEnv(os.Environ(), in.ProjectRoot, projectDirForTarget(in, target), in.InjectedEnv)
		return cmd.Run()
	})
}

func shouldAutoBuild(target deployTarget) bool {
	if target.Toolchain != "node" {
		return false
	}
	switch target.Backend {
	case workspace.DeployBackendCloudflare, workspace.DeployBackendEdgeOne:
		return true
	}
	return false
}

func projectDirForTarget(in deploy.ApplyInput, target deployTarget) string {
	if in.Project.TargetDir != "" {
		return in.Project.TargetDir
	}
	if target.Project.TargetDir != "" {
		return target.Project.TargetDir
	}
	return filepath.Join(in.ProjectRoot, filepath.FromSlash(target.Project.RelativeDir))
}

func readPackageScripts(projectDir string) (map[string]string, error) {
	raw, err := os.ReadFile(filepath.Join(projectDir, "package.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return nil, err
	}
	return pkg.Scripts, nil
}

func packageManagerForTarget(target deployTarget, _ *workspace.Manifest) string {
	pm := strings.TrimSpace(target.PackageManager)
	if pm == "" {
		// The current manifest dropped the workspace-level packageManager field — projects
		// either declare their own (target.PackageManager) or fall back
		// to the canonical pnpm default.
		pm = "pnpm"
	}
	return pm
}

func nodeBuildArgv(pm string) []string {
	switch strings.TrimSpace(pm) {
	case "npm":
		return []string{"npm", "run", "build"}
	case "yarn":
		return []string{"yarn", "build"}
	default:
		return []string{"pnpm", "run", "build"}
	}
}

func augmentDeployBuildEnv(parent []string, projectRoot, projectDir string, injected map[string]string) []string {
	base := secrets.MergeIntoEnviron(parent, injected, true)
	binPaths := []string{
		filepath.Join(projectDir, "node_modules", ".bin"),
		filepath.Join(projectRoot, "node_modules", ".bin"),
	}
	sep := string(os.PathListSeparator)
	out := make([]string, 0, len(base)+1)
	replaced := false
	for _, kv := range base {
		if !replaced && strings.HasPrefix(kv, "PATH=") {
			existing := strings.TrimPrefix(kv, "PATH=")
			parts := append([]string{}, binPaths...)
			if existing != "" {
				parts = append(parts, existing)
			}
			out = append(out, "PATH="+strings.Join(parts, sep))
			replaced = true
			continue
		}
		out = append(out, kv)
	}
	if !replaced {
		out = append(out, "PATH="+strings.Join(binPaths, sep))
	}
	return out
}

func newDeployCmd() *cobra.Command {
	var (
		profileFlag, buildVersion, project string
		envProvider                        string
		envFlag                            string
		dryRun                             bool
	)
	cmd := &cobra.Command{
		Use: "deploy",
		Long: `本命令逐个 project 派发到其声明的 deploy 后端。混合工作区
（前端走 aws-s3 / aliyun-oss / vercel、后端走 kustomize）在一次 ` + "`one deploy`" + ` 内完成各类部署。

profile 解析每个 project 各自走一遍：
  --profile <name>                          # 一次性覆盖（所有 project 都用这个）
  → config.json#workspaces[workspaceId].projects[project].profiles[deploy/backend]
  → config.json#workspaces[workspaceId].profiles[deploy/backend]
  → ~/.config/one/config.json#deploy/<backend>.default # machine default

env 目标环境（vercel / cloudflare / edgeone / kustomize 后端）按以下顺序解析：
  --env <name>                              # 一次性覆盖（所有 project 都用这个）
  → projects[i].domains.deploy.config.env   # project 自己的 pin（manifest）
  → "prod"                                  # 默认生产部署
kustomize 把 env 名映射到 overlay 路径 kustomize/overlays/<env>，覆盖
workspace 级别的 manifest.domains.deploy.config.kustomizationPath。

machine-level endpoint / 凭据用顶层 ` + "`one configure <domain>/<backend>`" + ` 管理，
对应：one configure deploy/aliyun-oss ... / one configure deploy/aws-s3 ... /
one configure deploy/kustomize ... / one configure deploy/vercel ... 等。

可用 backend：` + strings.Join(deploy.IDs(), " / ") + `（按字典序，由已注册的 provider 决定）。`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := workspace.ResolveProjectRoot("")
			if err != nil {
				return err
			}
			targets, err := deployTargets(root)
			if err != nil {
				return err
			}
			selector, err := normalizeProjectSelector(root, project)
			if err != nil {
				return err
			}
			targets, err = filterDeployTargets(targets, selector)
			if err != nil {
				return err
			}
			if len(targets) == 0 {
				return cliErrors.New(cliErrors.BACKEND_NOT_ENABLED,
					"工作区没有任何 subproject 声明 deploy 后端。在 projects[i] 中配置 deploy 后再试。")
			}
			m, _ := workspace.ReadManifest(root)
			if err := applyEnvOverride(m, envFlag); err != nil {
				return err
			}
			if err := validateProjectEnvs(m); err != nil {
				return err
			}
			// kustomize provider reads --build-version from a package-level
			// hand-off slot. Other providers ignore it.
			kustomize.ProviderTag = buildVersion
			defer func() { kustomize.ProviderTag = "" }()

			for _, t := range targets {
				if output.IsTTY() {
					fmt.Fprintf(cmd.OutOrStderr(),
						"→ deploying project %q via %s\n", t.Project.Name, t.Backend)
				}
				p, ok := deploy.Get(t.Backend)
				if !ok {
					return cliErrors.New(cliErrors.BACKEND_NOT_ENABLED,
						fmt.Sprintf("project %q 声明了未知 deploy 后端 %q（已注册：%v）",
							t.Project.Name, t.Backend, deploy.IDs()))
				}
				resolved, err := resolveDeployProfile(root, profileFlag, t)
				if err != nil {
					return err
				}
				resolved, err = ensureInteractiveCloudflareProfile(profileFlag, t, resolved)
				if err != nil {
					return err
				}
				input := deploy.ApplyInput{
					ProjectRoot: root,
					Project:     t.Project,
					Toolchain:   t.Toolchain,
					Manifest:    m,
					Resolved:    resolved,
					DryRun:      dryRun,
					Stdout:      cmd.OutOrStdout(),
					Stderr:      cmd.ErrOrStderr(),
				}
				providerID, err := resolveDeployEnvProvider(m, envProvider)
				if err != nil {
					return err
				}
				injection, err := deploy.LoadInjectionEnv(context.Background(), input, deploy.LoadInjectionOptions{
					LoaderID: providerID,
					EnvName:  envFlag,
				})
				if err != nil {
					return err
				}
				if injection != nil {
					input.InjectedEnv = injection.Vars
					input.InjectedEnvSource = injection.Source
				}
				buildLines, err := autoBuildBeforeDeploy(context.Background(), input, t, m, dryRun)
				if err != nil {
					return err
				}
				res, err := p.Apply(context.Background(), input)
				if err != nil {
					return err
				}
				if res == nil {
					continue
				}
				if dryRun {
					if injection != nil {
						envLabel := injection.EnvName
						if envLabel == "" {
							envLabel = "(default)"
						}
						_, _ = fmt.Fprintf(cmd.OutOrStdout(),
							"# injected env (source: %s, env=%s): %s\n",
							injection.Source, envLabel, strings.Join(injection.Keys, ", "))
					}
					lines := res.CommandLines
					if len(lines) == 0 {
						lines = []string{strings.Join(res.Argv, " ")}
					}
					lines = append(buildLines, lines...)
					for _, line := range lines {
						_, _ = cmd.OutOrStdout().Write([]byte(line + "\n"))
					}
					continue
				}
				output.Emit(res)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "只打印将执行的命令 / 计划")
	cmd.Flags().StringVar(&profileFlag, "profile", "", "一次性使用指定 profile（覆盖所有 project）")
	cmd.Flags().StringVar(&buildVersion, "build-version", "", "非交互/CI 用镜像版本（如 v0.1.0）；TTY 未传时会提示选择（仅 kustomize 后端有效）")
	cmd.Flags().StringVarP(&project, "project", "p", "", "只部署指定 subproject（manifest 里的 name 或相对路径）")
	cmd.Flags().StringVar(&envProvider, "env-provider", "", "env provider: dotenv | infisical（默认取 workspace manifest 中已选的值）")
	cmd.Flags().StringVar(&envFlag, "env", "", "deploy 目标环境名 + 环境变量环境名（必须在 manifest.environments.names；空=按 manifest 走）")
	i18n.MarkShort(cmd, "deploy.short")
	return cmd
}

// resolveDeployEnvProvider returns the env-provider id ("dotenv" | "infisical")
// to use for this deploy invocation. Flag wins; otherwise read the
// workspace's recorded provider (set at `one create --env-provider` time);
// fall back to dotenv if the manifest doesn't pin a provider yet.
func resolveDeployEnvProvider(m *workspace.Manifest, flag string) (string, error) {
	id := strings.ToLower(strings.TrimSpace(flag))
	if id == "" {
		id = workspace.EnvBackend(m)
	}
	if id == "" {
		id = workspace.EnvBackendDotenv
	}
	if id != workspace.EnvBackendDotenv && id != workspace.EnvBackendInfisical {
		return "", cliErrors.New(cliErrors.BACKEND_ID_UNKNOWN,
			"--env-provider 取值非法："+id+"（合法值: dotenv | infisical）")
	}
	return id, nil
}

// applyEnvOverride sets each project's per-provider deploy env to the
// flag value when --env was passed. The override is in-memory only; we
// never write back to one.manifest.json. Validates against
// manifest.environments.names via validateEnvAgainstDeclared so a typo at
// the CLI surfaces ENV_UNKNOWN_ENVIRONMENT before any provider runs.
func applyEnvOverride(m *workspace.Manifest, envFlag string) error {
	envFlag = strings.TrimSpace(envFlag)
	if envFlag == "" || m == nil {
		return nil
	}
	if err := validateEnvAgainstDeclared(m, envFlag); err != nil {
		return err
	}
	for i := range m.Projects {
		if m.Projects[i].Domains == nil || m.Projects[i].Domains.Deploy == nil {
			continue
		}
		dep := m.Projects[i].Domains.Deploy
		if err := setDeployEnv(dep, envFlag); err != nil {
			return err
		}
	}
	return nil
}

// validateProjectEnvs reports any per-project deploy env that is set
// but not present in manifest.environments.names. A workspace without an
// environments declaration is treated as "anything goes" — this matches
// the existing dotenv-only workspace flow that doesn't require declaring
// envs up front.
func validateProjectEnvs(m *workspace.Manifest) error {
	if m == nil {
		return nil
	}
	for _, p := range m.Projects {
		if p.Domains == nil || p.Domains.Deploy == nil {
			continue
		}
		envName, err := readDeployEnv(p.Domains.Deploy)
		if err != nil {
			return err
		}
		if envName == "" {
			continue
		}
		if err := validateEnvAgainstDeclared(m, envName); err != nil {
			return err
		}
	}
	return nil
}

// deployConfigEnvShape is the partial view used by applyEnvOverride /
// validateProjectEnvs to read or update only the `env` field of a
// per-project deploy backend's config blob, regardless of the kind. Every
// per-deploy-kind config (Vercel, Cloudflare, EdgeOne, Kustomize, S3) shares
// this field name; the merge-and-rewrite avoids touching kind-specific
// fields.
func readDeployEnv(dep *workspace.ProjectDeployBackend) (string, error) {
	if dep == nil || len(dep.Config) == 0 {
		return "", nil
	}
	cfg := struct {
		Env string `json:"env,omitempty"`
	}{}
	if err := json.Unmarshal(dep.Config, &cfg); err != nil {
		return "", err
	}
	return strings.TrimSpace(cfg.Env), nil
}

func setDeployEnv(dep *workspace.ProjectDeployBackend, env string) error {
	if dep == nil {
		return nil
	}
	cfg := map[string]json.RawMessage{}
	if len(dep.Config) > 0 {
		if err := json.Unmarshal(dep.Config, &cfg); err != nil {
			return err
		}
	}
	if env == "" {
		delete(cfg, "env")
	} else {
		raw, err := json.Marshal(env)
		if err != nil {
			return err
		}
		cfg["env"] = raw
	}
	if len(cfg) == 0 {
		dep.Config = nil
		return nil
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	dep.Config = raw
	return nil
}

func validateEnvAgainstDeclared(m *workspace.Manifest, env string) error {
	env = strings.TrimSpace(env)
	if env == "" {
		return nil
	}
	var declared []string
	if m.Environments != nil {
		declared = m.Environments.Names
	}
	if len(declared) == 0 {
		return nil
	}
	for _, e := range declared {
		if e == env {
			return nil
		}
	}
	return cliErrors.New(cliErrors.ENV_UNKNOWN_ENVIRONMENT,
		fmt.Sprintf("环境 %q 未在 manifest.environments.names 中（已声明：%s）。",
			env, strings.Join(declared, ", "))).
		WithContext(map[string]any{
			"requested":    env,
			"environments": declared,
		})
}
