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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/cliexts"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/cmd/configurecmd"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/helpui"
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
	"github.com/torchstellar-team/one-cli/packages/cli/internal/preset"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/prompt"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/template"
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

type deployDeferredResult struct {
	Schema   string `json:"schema"`
	Project  string `json:"project"`
	Provider string `json:"provider"`
	Recovery string `json:"recovery_command"`
}

func (r *deployDeferredResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	fmt.Fprintln(w, i18n.T("deploy.deferred"))
	fmt.Fprintf(w, i18n.T("deploy.deferred_project")+"\n", r.Project)
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("common.next_steps"))
	fmt.Fprintln(w, "  "+r.Recovery)
}

func manifestProjectTarget(root string, p *workspace.ManifestProject, backend string) deployTarget {
	return deployTarget{
		Project: workspace.Project{
			Name: p.Name, RelativeDir: p.RelativeDir,
			TargetDir: filepath.Join(root, filepath.FromSlash(p.RelativeDir)),
			Toolchain: p.Toolchain, PackageManager: p.PackageManager, TemplateID: p.TemplateID,
		},
		Backend: backend, Toolchain: p.Toolchain, TemplateID: p.TemplateID, PackageManager: p.PackageManager,
	}
}

func findManifestProject(m *workspace.Manifest, name string) *workspace.ManifestProject {
	if m == nil {
		return nil
	}
	for i := range m.Projects {
		if m.Projects[i].Name == name {
			return &m.Projects[i]
		}
	}
	return nil
}

func templateForProject(registry *template.Registry, p *workspace.ManifestProject) *template.Template {
	if registry == nil || p == nil {
		return nil
	}
	for i := range registry.Templates {
		if registry.Templates[i].ID == p.TemplateID {
			return &registry.Templates[i]
		}
	}
	return nil
}

func compatibleProviders(tpl *template.Template) []string {
	if tpl == nil || tpl.Compat == nil {
		return nil
	}
	registered := map[string]bool{}
	for _, id := range deploy.IDs() {
		registered[id] = true
	}
	var result []string
	for _, id := range tpl.Compat["deploy"] {
		if registered[id] {
			result = append(result, id)
		}
	}
	return result
}

func selectProjectForDeployment(m *workspace.Manifest, registry *template.Registry) (*workspace.ManifestProject, error) {
	options := []prompt.Option[string]{}
	for i := range m.Projects {
		p := &m.Projects[i]
		if len(compatibleProviders(templateForProject(registry, p))) == 0 {
			continue
		}
		options = append(options, prompt.Option[string]{Label: p.Name, Value: p.Name})
	}
	if len(options) == 0 {
		return nil, cliErrors.New(cliErrors.BACKEND_NOT_ENABLED, i18n.T("deploy.no_compatible_projects"))
	}
	name, err := prompt.Select(i18n.T("deploy.prompt_project"), options)
	if err != nil {
		return nil, err
	}
	return findManifestProject(m, name), nil
}

func providerCategory(provider string) string {
	switch provider {
	case workspace.DeployBackendKustomize:
		return "kubernetes"
	case workspace.DeployBackendVercel, workspace.DeployBackendCloudflare, workspace.DeployBackendEdgeOne:
		return "platform"
	default:
		return "static"
	}
}

func providerDisplayLabel(provider string) string {
	key := "deploy.provider." + provider
	label := i18n.T(key)
	if label == key {
		return provider
	}
	return label
}

func selectCompatibleProvider(compatible []string) (string, error) {
	categoryOrder := []string{"static", "platform", "kubernetes"}
	grouped := map[string][]string{}
	for _, provider := range compatible {
		category := providerCategory(provider)
		grouped[category] = append(grouped[category], provider)
	}
	categoryOptions := []prompt.Option[string]{}
	for _, category := range categoryOrder {
		if len(grouped[category]) > 0 {
			categoryOptions = append(categoryOptions, prompt.Option[string]{
				Label: i18n.T("deploy.category." + category), Value: category,
			})
		}
	}
	category, err := prompt.Select(i18n.T("deploy.prompt_category"), categoryOptions)
	if err != nil {
		return "", err
	}
	providerOptions := make([]prompt.Option[string], 0, len(grouped[category]))
	for _, provider := range grouped[category] {
		providerOptions = append(providerOptions, prompt.Option[string]{Label: providerDisplayLabel(provider), Value: provider})
	}
	return prompt.Select(i18n.T("deploy.prompt_provider"), providerOptions)
}

func configureFirstDeployment(cmd *cobra.Command, root string, m *workspace.Manifest, p *workspace.ManifestProject, registry *template.Registry, providerFlag, profileFlag string) (deployTarget, error) {
	tpl := templateForProject(registry, p)
	compatible := compatibleProviders(tpl)
	if len(compatible) == 0 {
		return deployTarget{}, cliErrors.New(cliErrors.BACKEND_NOT_ENABLED,
			i18n.Tf("deploy.project_not_deployable", p.Name))
	}
	providerID := strings.TrimPrefix(strings.TrimSpace(providerFlag), "deploy/")
	if providerID == "" {
		if !output.CanPrompt() {
			return deployTarget{}, cliErrors.New(cliErrors.BACKEND_NOT_ENABLED,
				i18n.Tf("deploy.provider_required", p.Name)).
				WithRemediation(output.Remediation{Action: "choose-deployment-target", Command: "one deploy " + p.Name + " --provider <target> --profile <connection>"})
		}
		var err error
		providerID, err = selectCompatibleProvider(compatible)
		if err != nil {
			return deployTarget{}, err
		}
	}
	allowed := false
	for _, id := range compatible {
		if id == providerID {
			allowed = true
			break
		}
	}
	if !allowed {
		return deployTarget{}, cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			i18n.Tf("deploy.provider_incompatible", providerID, p.Name, strings.Join(compatible, ", ")))
	}
	target := manifestProjectTarget(root, p, providerID)
	resolved, err := resolveDeployProfile(root, profileFlag, target)
	if err != nil {
		return deployTarget{}, err
	}
	if resolved == nil {
		if !output.CanPrompt() {
			return deployTarget{}, cliErrors.New(cliErrors.PROFILE_NONE_CONFIGURED,
				i18n.Tf("deploy.connection_required", providerID)).
				WithRemediation(output.Remediation{
					Action: "configure-connection", Command: "one configure add deploy/" + providerID + " --profile <connection>",
				})
		}
		choice, err := prompt.Select(i18n.Tf("deploy.prompt_missing_connection", providerID), []prompt.Option[string]{
			{Label: i18n.T("deploy.configure_now"), Value: "now"},
			{Label: i18n.T("deploy.configure_later"), Value: "later"},
		})
		if err != nil {
			return deployTarget{}, err
		}
		if choice == "later" {
			recovery := "one deploy " + p.Name
			output.Emit(&deployDeferredResult{
				Schema: "one-cli/deploy-deferred/v1", Project: p.Name, Provider: providerID, Recovery: recovery,
			})
			return deployTarget{}, cliErrors.New(cliErrors.PROMPT_CANCELLED, i18n.T("deploy.deferred")).WithExit0()
		}
		if err := configurecmd.ConfigureService(cmd, profile.DomainDeploy, providerID); err != nil {
			return deployTarget{}, err
		}
		resolved, err = resolveDeployProfile(root, profileFlag, target)
		if err != nil {
			return deployTarget{}, err
		}
		if resolved == nil {
			return deployTarget{}, cliErrors.New(cliErrors.PROFILE_NONE_CONFIGURED,
				i18n.Tf("deploy.connection_required", providerID))
		}
	}
	// Only now, after the user has chosen a usable local connection, mutate
	// workspace deployment state and generate its artifacts.
	if err := preset.ConfigureProjectDeployment(cmd.Context(), root, tpl, p.Name, providerID); err != nil {
		return deployTarget{}, err
	}
	_ = m
	return target, nil
}

func newDeployCmd() *cobra.Command {
	var (
		profileFlag, providerFlag, buildVersion, project string
		envProvider                                      string
		envFlag                                          string
		dryRun                                           bool
	)
	cmd := &cobra.Command{
		Use:     "deploy [project]",
		Long:    i18n.T("deploy.tip"),
		Example: "  one deploy\n  one deploy web\n  one deploy web --provider vercel --profile work",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			positional := ""
			if len(args) > 0 {
				positional = args[0]
			}
			if positional != "" && project != "" && positional != project {
				return cliErrors.New(cliErrors.ONE_CLI_ERROR,
					i18n.T("deploy.selector_conflict"))
			}
			if positional != "" {
				project = positional
			}
			root, err := workspace.ResolveProjectRoot("")
			if err != nil {
				return err
			}
			if !workspace.HasManifest(root) {
				return cliErrors.New(cliErrors.NOT_ONE_PROJECT, i18n.T("deploy.workspace_required"))
			}
			m, err := workspace.ReadManifest(root)
			if err != nil {
				return err
			}
			registry, err := template.Fetch(cmd.Context(), "")
			if err != nil {
				return err
			}

			configuredTargets, err := deployTargets(root)
			if err != nil {
				return err
			}
			selector, err := normalizeProjectSelector(root, project)
			if err != nil {
				return err
			}
			var targets []deployTarget
			if selector == "" && strings.TrimSpace(providerFlag) == "" && len(configuredTargets) > 0 {
				targets = configuredTargets
			} else {
				var selectedProject *workspace.ManifestProject
				if selector != "" {
					sub, resolveErr := workspace.ResolveProjectFromSelector(root, selector)
					if resolveErr != nil {
						return resolveErr
					}
					if sub != nil {
						selectedProject = findManifestProject(m, sub.Name)
					}
					if selectedProject == nil {
						return cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND,
							i18n.Tf("deploy.project_not_found", selector)).
							WithContext(map[string]any{"selector": selector, "available_projects": workspace.ProjectNames(m)})
					}
				} else if !output.CanPrompt() {
					if len(m.Projects) == 1 && strings.TrimSpace(providerFlag) != "" {
						selectedProject = &m.Projects[0]
					} else {
						return cliErrors.New(cliErrors.BACKEND_NOT_ENABLED,
							i18n.T("deploy.project_required")).
							WithRemediation(output.Remediation{Action: "choose-project", Command: "one deploy <project> --provider <target> --profile <connection>"})
					}
				} else {
					selectedProject, err = selectProjectForDeployment(m, registry)
					if err != nil {
						return err
					}
				}

				existingBackend := workspace.DeployForProject(m, selectedProject.Name).Backend
				if existingBackend != "" && strings.TrimSpace(providerFlag) == "" {
					targets = []deployTarget{manifestProjectTarget(root, selectedProject, existingBackend)}
				} else {
					target, configureErr := configureFirstDeployment(cmd, root, m, selectedProject, registry, providerFlag, profileFlag)
					if configureErr != nil {
						return configureErr
					}
					targets = []deployTarget{target}
					m, err = workspace.ReadManifest(root)
					if err != nil {
						return err
					}
				}
			}
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
						i18n.T("deploy.starting")+"\n", t.Project.Name, providerDisplayLabel(t.Backend))
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
				injection, err := deploy.LoadInjectionEnv(cmd.Context(), input, deploy.LoadInjectionOptions{
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
				buildLines, err := autoBuildBeforeDeploy(cmd.Context(), input, t, m, dryRun)
				if err != nil {
					return err
				}
				res, err := p.Apply(cmd.Context(), input)
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
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, i18n.T("deploy.flag.dry_run"))
	cmd.Flags().StringVar(&profileFlag, "profile", "", i18n.T("deploy.flag.profile"))
	cmd.Flags().StringVar(&providerFlag, "provider", "", i18n.T("deploy.flag.provider"))
	cmd.Flags().StringVar(&buildVersion, "build-version", "", i18n.T("deploy.flag.build_version"))
	cmd.Flags().StringVarP(&project, "project", "p", "", i18n.T("deploy.flag.project"))
	cmd.Flags().StringVar(&envProvider, "env-provider", "", i18n.T("deploy.flag.env_provider"))
	cmd.Flags().StringVar(&envFlag, "env", "", i18n.T("deploy.flag.env"))
	for name, key := range map[string]string{
		"dry-run": "deploy.flag.dry_run", "profile": "deploy.flag.profile",
		"provider": "deploy.flag.provider", "build-version": "deploy.flag.build_version",
		"project": "deploy.flag.project", "env-provider": "deploy.flag.env_provider",
		"env": "deploy.flag.env",
	} {
		i18n.MarkFlagUsage(cmd, name, key)
	}
	helpui.MarkAdvanced(cmd, "profile", "provider", "project", "build-version", "env-provider")
	i18n.MarkShort(cmd, "deploy.short")
	i18n.MarkLong(cmd, "deploy.tip")
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
