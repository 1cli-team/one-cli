// Package createcmd contributes `one create` to the root command via
// cliexts. Scaffolds a new workspace (one.manifest.json + folder skeleton),
// applies the default backend selection (env/dotenv | ci/github-actions |
// dev/process; optionally swaps env to infisical), and installs the
// bundled skill to the user's agent paths.
package createcmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/cliexts"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/preset"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/prompt"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/scaffold"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets/dotenv"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets/infisical"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/skills"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func init() {
	cliexts.Register("create", buildContributions)
}

func buildContributions() []*cobra.Command {
	return []*cobra.Command{newCreateCmd()}
}

// workspaceDefaultEnables are the default backend ids stamped into
// the manifest when scaffolding a new workspace. ci and dev have a
// single canonical implementation; env defaults to dotenv (lowest-
// friction; the interactive prompt may swap to env/infisical).
var workspaceDefaultEnables = []string{
	"env/dotenv",
	"ci/github-actions",
	"dev/process",
}

// canonicalDomainOrder is the canonical iteration order for emitting
// the list of enabled backends in the create envelope. Mirrors the
// legacy ordering.
var canonicalDomainOrder = []string{"container", "dev", "deploy", "ci", "env"}

type createFlags struct {
	name         string
	yes          bool
	envProvider  string
	preset       string
	projectNames string
}

func newCreateCmd() *cobra.Command {
	flags := &createFlags{}
	cmd := &cobra.Command{
		Use: "create [dir]",
		Long: `创建工作区。

位置参数 [dir] 是 **目标目录**（不是项目名）。可以是相对路径或绝对路径。
默认 项目名 = basename(dir)；如需不同名字用 --name 显式指定。

示例：
  one create my-app                    # 创建 ./my-app/，项目名 my-app
  one create services/billing          # 创建 ./services/billing/，名 billing
  one create /tmp/sandbox --name demo  # 创建 /tmp/sandbox/，名 demo
  one create .                          # 在当前目录创建，名 = basename(cwd)

工作区默认配置（自动应用）：
  env       - dotenv（.env 系列文件读写，零配置）
  ci        - GitHub Actions 工作流
  dev       - Procfile.dev（overmind / hivemind / foreman / honcho 可读）

需要 Infisical 托管环境变量？两种方式：
  - create 时显式选：one create my-app --env-provider infisical
  - 后期再切换：    one env switch infisical

deploy / container 由模板自动决定，不在 create 时落盘：
  go-api / nestjs-api / nextjs-app                       → Dockerfile + kustomize
  react-spa / astro-site / starlight-docs                → aws-s3 (静态部署)
  expo-mobile / ts-library / electron-app                → 不参与 deploy / container

会自动安装 bundled skill 到本机 agent。`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := ""
			if len(args) > 0 {
				dir = args[0]
			}
			return runCreate(cmd, dir, flags)
		},
	}
	cmd.Flags().StringVarP(&flags.name, "name", "n", "", "项目名称（默认 basename(dir)）")
	cmd.Flags().BoolVarP(&flags.yes, "yes", "y", false, "非交互模式：使用全部默认值")
	cmd.Flags().StringVar(&flags.envProvider, "env-provider", "",
		"env provider: dotenv | infisical（默认 dotenv；要 Infisical 也可后期 one env switch infisical）")
	cmd.Flags().StringVar(&flags.preset, "preset", "",
		"preset id（如 `1.bnek.fnav.ei`）：一次性 scaffold 工作区 + 项目 + deploy + env。隐含 -y，必须传 [dir]。")
	cmd.Flags().StringVar(&flags.projectNames, "project-names", "",
		"--preset 模式下按展开顺序指定子项目名，逗号分隔（如 `api,web,shared`）。")
	i18n.MarkShort(cmd, "create.short")
	return cmd
}

func runCreate(cmd *cobra.Command, rawDir string, flags *createFlags) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// --preset implies fully non-interactive: the whole point is a
	// reproducible scaffold from a single string. Force -y, parse the
	// preset id up-front, and short-circuit to runCreateWithPreset
	// which fans out into preset.Apply after the workspace skeleton
	// lands. Pre-flight (parse + registry resolve) runs BEFORE any
	// filesystem mutation so PRESET_INVALID never leaves a half-baked
	// dir behind.
	if flags.preset != "" {
		flags.yes = true
		return runCreateWithPreset(cmd, cwd, rawDir, flags)
	}

	interactive := !flags.yes && output.IsTTY()

	// validateDir is the unified target-directory validator: same logic
	// the post-form code path runs (existence + emptiness + nesting),
	// surfaced inside the huh prompt so the user sees the conflict
	// before they finish filling in the form. Without this, you fill in
	// dir + name + skill picks and only THEN learn the directory was
	// already a workspace — bad UX.
	validateDir := func(v string) error {
		v = strings.TrimSpace(v)
		if v == "" {
			return errors.New("请输入目标目录")
		}
		abs := resolveTargetPath(cwd, v)
		if pathExists(abs) {
			empty, err := scaffold.IsDirectoryEmpty(abs)
			if err != nil {
				return err
			}
			if !empty {
				return fmt.Errorf("目标目录 %s 已存在且非空", v)
			}
		}
		if conflict := findEnclosingWorkspace(abs); conflict != "" {
			return fmt.Errorf("拒绝在已存在的工作区里创建：%s 已是 one workspace；请用 one add", conflict)
		}
		return nil
	}
	// Form-mode validation allows empty (we'll fall back to basename(dir));
	// but if the user types something, it must parse as a valid name.
	validateNameOptional := func(v string) error {
		v = strings.TrimSpace(v)
		if v == "" {
			return nil
		}
		if !workspace.IsValidProjectName(v) {
			return errors.New("名称只能包含字母数字、下划线、连字符，且不能以连字符开头")
		}
		return nil
	}

	// Pre-flight: if the user hasn't told us a directory yet AND cwd is
	// itself inside a workspace, refuse before opening any prompt. The
	// per-field validator above also catches this once they type a path,
	// but failing earlier is friendlier when the answer is "you're in
	// the wrong place to begin with".
	if rawDir == "" && interactive {
		if conflict := findEnclosingWorkspace(cwd); conflict != "" {
			return cliErrors.New(cliErrors.WORKSPACE_NESTED_FORBIDDEN,
				fmt.Sprintf("当前目录在已存在的工作区里：%s。请 cd 到工作区外再 one create，或用 one add 在现有工作区里加项目。", conflict)).
				WithContext(map[string]any{
					"cwd":                 cwd,
					"enclosing_workspace": conflict,
				})
		}
	}

	// Fast path: when both dir and name need prompting, render them in a
	// single huh form so the user can shift+tab back to revise dir before
	// committing.
	if rawDir == "" && flags.name == "" && interactive {
		var dirInput, nameInput string
		if err := prompt.NewForm().
			Text(&dirInput, "目标目录（输入 . 表示使用当前目录）", "./my-app", validateDir).
			Text(&nameInput, "项目名称（留空则用目录的基础名）", "", validateNameOptional).
			Run(); err != nil {
			return err
		}
		rawDir = strings.TrimSpace(dirInput)
		flags.name = strings.TrimSpace(nameInput)
	} else if rawDir == "" {
		if !interactive {
			return cliErrors.New(cliErrors.PROJECT_NAME_REQUIRED,
				"非交互模式下必须提供 [dir] 位置参数（使用 `.` 表示当前目录）。").
				WithContext(map[string]any{"interactive": false})
		}
		got, err := prompt.Text("目标目录（输入 . 表示使用当前目录）", "./my-app", validateDir)
		if err != nil {
			return err
		}
		rawDir = strings.TrimSpace(got)
	}

	useCurrentDir := rawDir == "." || rawDir == "./"
	targetDir := resolveTargetPath(cwd, rawDir)

	// Resolve project name. Default = basename(targetDir).
	projectName := strings.TrimSpace(flags.name)
	if projectName == "" {
		projectName = filepath.Base(targetDir)
	}
	if !workspace.IsValidProjectName(projectName) {
		return cliErrors.New(cliErrors.INVALID_NAME,
			fmt.Sprintf("项目名称格式不合法: %q（来自 --name 或 basename(dir)）", projectName))
	}

	displayPath := relativeOrAbs(cwd, targetDir, useCurrentDir)

	dirExists := pathExists(targetDir)
	empty := true
	if dirExists {
		var err error
		empty, err = scaffold.IsDirectoryEmpty(targetDir)
		if err != nil {
			return err
		}
	}
	createdFromScratch := false
	if !dirExists {
		createdFromScratch = true
	}

	if dirExists && !empty {
		return cliErrors.New(cliErrors.EXISTING_TARGET_NOT_EMPTY,
			fmt.Sprintf("目标目录 %s 已存在且非空。请删除目录后重试，或换一个目标位置。", displayPath)).
			WithContext(map[string]any{
				"target_path":  targetDir,
				"display_path": displayPath,
			})
	}

	// Refuse to create a workspace inside (or AS) an existing workspace.
	// Two manifests in the same tree confuse subproject discovery and
	// produce silently inconsistent behaviour for env / add. Users
	// who actually want to add to an existing workspace should use `one add`.
	if conflict := findEnclosingWorkspace(targetDir); conflict != "" {
		return cliErrors.New(cliErrors.WORKSPACE_NESTED_FORBIDDEN,
			fmt.Sprintf("拒绝在已存在的工作区里创建新工作区：%s 已经是一个 one workspace。要在现有工作区里加项目，请用 one add。", conflict)).
			WithContext(map[string]any{
				"target_path":         targetDir,
				"enclosing_workspace": conflict,
				"display_path":        displayPath,
			})
	}

	// Backend selection: --env-provider flag wins; otherwise interactive
	// prompt asks dotenv vs infisical.
	enables, err := resolveCreateEnables(flags.envProvider, interactive)
	if err != nil {
		return err
	}

	options := scaffold.Options{
		ProjectName:    projectName,
		PackageManager: "pnpm",
		// Docker/K8s shells are no longer written by `create`. Backend
		// selections written into one.manifest.json below; per-backend sync
		// runs the next time `one add` runs.
		Docker: false,
		K8s:    false,
	}

	if err := prompt.Spin("正在生成工作区骨架", func() error {
		return scaffold.Generate(targetDir, options)
	}); err != nil {
		// Roll back fresh-create-from-scratch directory on failure, so a
		// half-written workspace doesn't confuse the next attempt.
		if createdFromScratch && !useCurrentDir {
			_ = os.RemoveAll(targetDir)
		}
		return err
	}
	prompt.Step(fmt.Sprintf("骨架生成完成 → %s", displayPath))

	// Apply backend selections to the manifest. Domains that need a
	// project (container / deploy) defer their first Sync to `one add`;
	// workspace domains (dev / ci / env) can run their Sync immediately when
	// needed so workspace-level setup (e.g. .gitignore tweaks) lands at
	// create time.
	if len(enables) > 0 {
		if err := workspace.ApplyBackendSelection(targetDir, enables); err != nil {
			return cliErrors.New(cliErrors.BACKEND_ID_UNKNOWN, err.Error()).
				WithContext(map[string]any{"enabled_backends": enables})
		}
		// Run the Sync side-effect for the selected env backend. The
		// .gitignore patterns (.env / .env.* / !.env.example) are
		// useful regardless of backend — Infisical pulls still write
		// local .env files — so dotenv.Sync runs unconditionally.
		// Infisical's Sync stays a no-op; first-time project setup is
		// handled by the best-effort auto-bind below.
		if err := dotenv.Sync(targetDir); err != nil {
			return cliErrors.New(cliErrors.STATUS_FIX_FAILED,
				fmt.Sprintf("env/dotenv 同步失败: %v", err))
		}
		for _, id := range enables {
			if id == "env/"+workspace.EnvBackendInfisical {
				if err := infisical.Sync(); err != nil {
					return cliErrors.New(cliErrors.STATUS_FIX_FAILED,
						fmt.Sprintf("env/infisical 同步失败: %v", err))
				}
				// Best-effort auto-bind: when the user picked Infisical
				// AND a working env/infisical profile is configured,
				// reach out and bind / create the project so the user
				// doesn't have to bind the project manually. Any
				// failure (no profile, network down, auth rejected) is
				// surfaced as a soft warning — backend stays stamped,
				// projectId stays empty, and the first env command will
				// retry lazy auto-bind with a clearer error message.
				_ = prompt.Spin("正在绑定 Infisical 项目", func() error {
					_, err := infisical.Init(cmd.Context(), targetDir, infisical.InitInput{
						ProjectName: projectName,
					})
					if err != nil {
						prompt.Step(fmt.Sprintf("Infisical 自动绑定未完成（%v）；首次运行 `one env set/get/list/pull` 时会再尝试一次", err))
					}
					return nil
				})
			}
		}
	}

	// `git init` is best-effort: missing-git is a warning, not a hard fail.
	// We don't surface it in the JSON envelope (TS doesn't either).
	_ = scaffold.InitGitRepo(targetDir)

	var skillsResult skillsPayload
	_ = prompt.Spin("正在安装 skill 到本机 agent", func() error {
		skillsResult = installSkills()
		return nil
	})
	if skillsResult.Status == "failed" {
		// Match TS behaviour: workspace creation succeeded, skills install
		// failed. Set non-zero exit code but emit the result envelope so
		// agents see what happened.
		cmd.SetContext(cmd.Context()) // no-op; reserved for future ctx tagging
	}

	// v2 envelope: replaces the v1 `enabled_backends []string` with
	// per-domain semantic fields. `secrets_backend` names the env
	// backend ("dotenv" / "infisical"); `ci_enabled` / `dev_enabled` are
	// booleans. Container / deploy are template-driven and live on the
	// subproject record, not in this envelope.
	secretsBackend := ""
	ciEnabled := false
	devEnabled := false
	for _, id := range enables {
		switch {
		case strings.HasPrefix(id, "env/"):
			secretsBackend = strings.TrimPrefix(id, "env/")
		case strings.HasPrefix(id, "ci/"):
			ciEnabled = true
		case strings.HasPrefix(id, "dev/"):
			devEnabled = true
		}
	}
	payload := createResult{
		Schema:         "one-cli/create/v2",
		ProjectName:    projectName,
		CreatedPath:    targetDir,
		CreatedInPlace: useCurrentDir,
		PackageManager: options.PackageManager,
		SecretsBackend: secretsBackend,
		CIEnabled:      ciEnabled,
		DevEnabled:     devEnabled,
		Skills:         skillsResult,
	}
	output.Emit(&payload)

	if skillsResult.Status == "failed" {
		// Non-fatal but surfaces non-zero exit, mirroring TS.
		os.Exit(1)
	}
	return nil
}

type createResult struct {
	Schema         string        `json:"schema"`
	ProjectName    string        `json:"project_name"`
	CreatedPath    string        `json:"created_path"`
	CreatedInPlace bool          `json:"created_in_place"`
	PackageManager string        `json:"package_manager"`
	SecretsBackend string        `json:"secrets_backend,omitempty"`
	CIEnabled      bool          `json:"ci_enabled"`
	DevEnabled     bool          `json:"dev_enabled"`
	Skills         skillsPayload `json:"skills"`
}

// RenderTTY prints a friendly create-success summary.
func (r *createResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	fmt.Fprintf(w, "✓ Workspace created: %s\n", r.ProjectName)
	fmt.Fprintf(w, "  Path: %s\n", r.CreatedPath)
	fmt.Fprintf(w, "  Package manager: %s\n", r.PackageManager)
	if r.SecretsBackend != "" {
		fmt.Fprintf(w, "  Env backend: %s\n", r.SecretsBackend)
	}
	if r.CIEnabled {
		fmt.Fprintln(w, "  CI: enabled")
	}
	if r.DevEnabled {
		fmt.Fprintln(w, "  Dev runner: enabled")
	}
	switch r.Skills.Status {
	case "completed":
		fmt.Fprintln(w, "  Skills: installed")
	case "failed":
		if r.Skills.Error != nil {
			fmt.Fprintf(w, "  Skills: ✗ %s (%s)\n", r.Skills.Error.Message, r.Skills.Error.Code)
		} else {
			fmt.Fprintln(w, "  Skills: ✗ install failed")
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Next steps:")
	fmt.Fprintf(w, "  cd %s\n", r.CreatedPath)
	fmt.Fprintln(w, "  one add <template> --name <subproject-name>")
}

type skillsPayload struct {
	Status      string       `json:"status"`
	InstalledTo []string     `json:"installed_to,omitempty"`
	SkillCount  int          `json:"skill_count,omitempty"`
	Reason      string       `json:"reason,omitempty"`
	Error       *skillsError `json:"error,omitempty"`
}

type skillsError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func installSkills() skillsPayload {
	res, err := skills.Install()
	if err != nil {
		var cliErr *output.Error
		code := string(cliErrors.SKILLS_INSTALL_FAILED)
		message := err.Error()
		if errors.As(err, &cliErr) {
			code = cliErr.Code
		}
		return skillsPayload{
			Status: "failed",
			Error:  &skillsError{Code: code, Message: message},
		}
	}
	return skillsPayload{
		Status:      "completed",
		InstalledTo: res.InstalledTo,
		SkillCount:  res.SkillCount,
	}
}

func relativeOrAbs(cwd, targetDir string, useCurrentDir bool) string {
	rel, err := filepath.Rel(cwd, targetDir)
	if err == nil && rel != "" {
		return rel
	}
	if useCurrentDir {
		return "."
	}
	return targetDir
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func parsePresetProjectNames(raw string, want int) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	names := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for i, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			return nil, cliErrors.New(cliErrors.INVALID_NAME,
				fmt.Sprintf("--project-names 第 %d 项为空。", i+1)).
				WithContext(map[string]any{
					"project_names": raw,
					"index":         i,
				})
		}
		if !workspace.IsValidProjectName(name) {
			return nil, cliErrors.New(cliErrors.INVALID_NAME,
				fmt.Sprintf("子项目名称格式不合法: %q（来自 --project-names）", name)).
				WithContext(map[string]any{
					"project_names": raw,
					"invalid_name":  name,
					"index":         i,
				})
		}
		if seen[name] {
			return nil, cliErrors.New(cliErrors.INVALID_NAME,
				fmt.Sprintf("--project-names 包含重复名称: %q", name)).
				WithContext(map[string]any{
					"project_names":  raw,
					"duplicate_name": name,
					"index":          i,
				})
		}
		seen[name] = true
		names = append(names, name)
	}
	if len(names) != want {
		return nil, cliErrors.New(cliErrors.PRESET_INVALID,
			fmt.Sprintf("--project-names 数量为 %d，但 preset 会展开 %d 个子项目。", len(names), want)).
			WithContext(map[string]any{
				"project_names": raw,
				"provided":      len(names),
				"expected":      want,
			})
	}
	return names, nil
}

// resolveCreateEnables resolves the list of backend ids to enable when
// scaffolding a new workspace. The post-trim policy:
//
//  1. workspaceDefaultEnables baseline (env/dotenv + ci/github-actions
//     + dev/process) — always applied unless overridden by --env-provider.
//     ci and dev have exactly one registered backend each so there's
//     nothing to pick. env defaults to dotenv because it's the
//     lowest-friction option for solo / OSS users.
//
//  2. --env-provider flag explicitly picks dotenv or infisical.
//     No interactive prompt at create time — users who don't pass the
//     flag get dotenv silently, and can switch later via `one env switch`.
//
//  3. deploy and container Domains are template-driven (registry.json
//     defaults applied at `one add` time). Not asked at create
//     time; not in workspaceDefaultEnables.
//
// Returned ids are namespaced (e.g. "env/dotenv"); per-Domain
// uniqueness is guaranteed by the time this returns.
func resolveCreateEnables(envProvider string, interactive bool) ([]string, error) {
	_ = interactive

	// Build a domain -> id map starting from the workspace defaults.
	byDomain := map[string]string{}
	for _, id := range workspaceDefaultEnables {
		byDomain[domainOf(id)] = id
	}

	// --env-provider accepts the bare backend name ("dotenv" / "infisical").
	// Empty → keep the dotenv default.
	switch strings.TrimSpace(envProvider) {
	case "":
		// keep default (env/dotenv)
	case "dotenv":
		byDomain["env"] = "env/dotenv"
	case "infisical":
		byDomain["env"] = "env/infisical"
	default:
		return nil, cliErrors.New(cliErrors.BACKEND_ID_UNKNOWN,
			fmt.Sprintf("--env-provider 值无效: %q（合法值: dotenv / infisical）", envProvider))
	}

	// Flatten deterministically by canonical domain order.
	out := []string{}
	for _, d := range canonicalDomainOrder {
		if id, ok := byDomain[d]; ok && id != "" {
			out = append(out, id)
		}
	}
	return out, nil
}

// domainOf extracts the domain component of a namespaced id
// ("env/dotenv" -> "env"). Returns "" if the id has no slash.
func domainOf(id string) string {
	if i := strings.Index(id, "/"); i > 0 {
		return id[:i]
	}
	return ""
}

// resolveTargetPath turns a user-supplied target ("." | "./foo" | absolute |
// relative) into an absolute path rooted at cwd. Used by both the pre-form
// validator and the post-form scaffold step so they always agree on what
// directory we're talking about.
func resolveTargetPath(cwd, raw string) string {
	switch raw {
	case ".", "./":
		return cwd
	}
	if filepath.IsAbs(raw) {
		return raw
	}
	return filepath.Join(cwd, raw)
}

// runCreateWithPreset is the --preset code path. The non-preset path
// (runCreate's tail) is unchanged. This function:
//
//  1. Parses the preset id (no FS mutation), runs Resolve against the
//     registry (still no FS mutation).
//  2. Refuses to prompt — `--preset` without a dir argument errors
//     with PROJECT_NAME_REQUIRED rather than dropping into a TTY form.
//  3. Validates the env provider doesn't collide with --env-provider.
//  4. Runs the same scaffold + workspace.ApplyBackendSelection path
//     as the legacy create.
//  5. Hands off to preset.Apply, which materializes every project
//     segment in canonical order.
//  6. Emits an envelope with schema=one-cli/create/v3 (the non-preset
//     path keeps v2 unchanged).
func runCreateWithPreset(cmd *cobra.Command, cwd, rawDir string, flags *createFlags) error {
	// Step 1: parse the preset id (pure, no IO).
	spec, err := preset.Parse(flags.preset)
	if err != nil {
		var pe *preset.ParseError
		ctx := map[string]any{"preset_id": flags.preset}
		if errors.As(err, &pe) {
			if pe.Segment != "" {
				ctx["failed_segment"] = pe.Segment
				ctx["segment_index"] = pe.SegmentIndex
			}
			ctx["reason"] = pe.Reason
		}
		return cliErrors.New(cliErrors.PRESET_INVALID, err.Error()).WithContext(ctx)
	}

	// Step 2: fetch registry + resolve codes -> templates / backends.
	registry, err := template.Fetch(cmd.Context(), "")
	if err != nil {
		return err
	}
	resolved, err := preset.Resolve(spec, registry)
	if err != nil {
		var re *preset.ResolveError
		if errors.As(err, &re) {
			ctx := map[string]any{
				"preset_id": flags.preset,
				"reason":    re.Reason,
			}
			if re.Segment != "" {
				ctx["failed_segment"] = re.Segment
			}
			if re.Code != "" {
				ctx["failed_code"] = re.Code
			}
			if re.TemplateID != "" {
				ctx["template_id"] = re.TemplateID
			}
			if len(re.Compat) > 0 {
				ctx["compat"] = re.Compat
			}
			switch re.Kind {
			case "template":
				return cliErrors.New(cliErrors.TEMPLATE_NOT_FOUND, err.Error()).WithContext(ctx)
			case "deploy", "container":
				return cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID, err.Error()).WithContext(ctx)
			case "env", "extension":
				return cliErrors.New(cliErrors.PRESET_INVALID, err.Error()).WithContext(ctx)
			}
		}
		return err
	}

	// Step 3: env-provider conflict check.
	flagProvider := strings.TrimSpace(flags.envProvider)
	if flagProvider != "" && flagProvider != "dotenv" && flagProvider != "infisical" {
		return cliErrors.New(cliErrors.BACKEND_ID_UNKNOWN,
			fmt.Sprintf("--env-provider 值无效: %q（合法值: dotenv / infisical）", flagProvider))
	}
	effectiveEnv, err := preset.ResolveEnvWithFlag(resolved.EnvProvider, flagProvider)
	if err != nil {
		var ce *preset.EnvConflictError
		if errors.As(err, &ce) {
			return cliErrors.New(cliErrors.PRESET_FLAG_CONFLICT,
				fmt.Sprintf("preset 声明 env=%q，但 --env-provider %q 与之冲突", ce.Preset, ce.Flag)).
				WithContext(map[string]any{
					"preset_env_provider": ce.Preset,
					"flag_env_provider":   ce.Flag,
				})
		}
		return err
	}
	if effectiveEnv == "" {
		effectiveEnv = "dotenv"
	}

	customProjectNames, err := parsePresetProjectNames(flags.projectNames, len(resolved.Items))
	if err != nil {
		return err
	}

	// Step 4: directory validation (same checks as non-preset path,
	// minus the interactive form).
	if rawDir == "" {
		return cliErrors.New(cliErrors.PROJECT_NAME_REQUIRED,
			"--preset 模式下必须提供 [dir] 位置参数（使用 `.` 表示当前目录）。").
			WithContext(map[string]any{"preset_id": flags.preset})
	}
	useCurrentDir := rawDir == "." || rawDir == "./"
	targetDir := resolveTargetPath(cwd, rawDir)
	projectName := strings.TrimSpace(flags.name)
	if projectName == "" {
		projectName = filepath.Base(targetDir)
	}
	if !workspace.IsValidProjectName(projectName) {
		return cliErrors.New(cliErrors.INVALID_NAME,
			fmt.Sprintf("项目名称格式不合法: %q（来自 --name 或 basename(dir)）", projectName))
	}
	displayPath := relativeOrAbs(cwd, targetDir, useCurrentDir)

	dirExists := pathExists(targetDir)
	empty := true
	if dirExists {
		empty, err = scaffold.IsDirectoryEmpty(targetDir)
		if err != nil {
			return err
		}
	}
	createdFromScratch := !dirExists
	if dirExists && !empty {
		return cliErrors.New(cliErrors.EXISTING_TARGET_NOT_EMPTY,
			fmt.Sprintf("目标目录 %s 已存在且非空。请删除目录后重试，或换一个目标位置。", displayPath)).
			WithContext(map[string]any{
				"target_path":  targetDir,
				"display_path": displayPath,
			})
	}
	if conflict := findEnclosingWorkspace(targetDir); conflict != "" {
		return cliErrors.New(cliErrors.WORKSPACE_NESTED_FORBIDDEN,
			fmt.Sprintf("拒绝在已存在的工作区里创建新工作区：%s 已经是一个 one workspace。", conflict)).
			WithContext(map[string]any{
				"target_path":         targetDir,
				"enclosing_workspace": conflict,
			})
	}

	// Step 5: scaffold workspace skeleton.
	options := scaffold.Options{
		ProjectName:    projectName,
		PackageManager: "pnpm",
		Docker:         false,
		K8s:            false,
	}
	if err := prompt.Spin("正在生成工作区骨架", func() error {
		return scaffold.Generate(targetDir, options)
	}); err != nil {
		if createdFromScratch && !useCurrentDir {
			_ = os.RemoveAll(targetDir)
		}
		return err
	}
	prompt.Step(fmt.Sprintf("骨架生成完成 → %s", displayPath))

	// Step 6: apply workspace-level backend selection (env / ci / dev).
	enables := []string{
		"env/" + effectiveEnv,
		"ci/github-actions",
		"dev/process",
	}
	canonicalEnables := []string{}
	for _, d := range canonicalDomainOrder {
		for _, id := range enables {
			if domainOf(id) == d {
				canonicalEnables = append(canonicalEnables, id)
			}
		}
	}
	if err := workspace.ApplyBackendSelection(targetDir, canonicalEnables); err != nil {
		return cliErrors.New(cliErrors.BACKEND_ID_UNKNOWN, err.Error()).
			WithContext(map[string]any{"enabled_backends": canonicalEnables})
	}
	if err := dotenv.Sync(targetDir); err != nil {
		return cliErrors.New(cliErrors.STATUS_FIX_FAILED,
			fmt.Sprintf("env/dotenv 同步失败: %v", err))
	}
	infisicalBound := false
	if effectiveEnv == workspace.EnvBackendInfisical {
		if err := infisical.Sync(); err != nil {
			return cliErrors.New(cliErrors.STATUS_FIX_FAILED,
				fmt.Sprintf("env/infisical 同步失败: %v", err))
		}
		_ = prompt.Spin("正在绑定 Infisical 项目", func() error {
			_, bindErr := infisical.Init(cmd.Context(), targetDir, infisical.InitInput{
				ProjectName: projectName,
			})
			if bindErr != nil {
				prompt.Step(fmt.Sprintf("Infisical 自动绑定未完成（%v）；首次运行 `one env set/get/list/pull` 时会再尝试一次", bindErr))
				return nil
			}
			infisicalBound = true
			return nil
		})
	}

	// Step 7: hand off to the preset engine for the per-project work.
	applyResult, applyErr := preset.Apply(cmd.Context(), targetDir, resolved, preset.ApplyOptions{
		ProjectNames: customProjectNames,
	})

	partialState := "none"
	if applyErr != nil && len(applyResult.Projects) > 0 {
		partialState = "partial_projects"
	}

	// Step 8: best-effort git init and skill install (unchanged from
	// the non-preset path).
	_ = scaffold.InitGitRepo(targetDir)
	var skillsResult skillsPayload
	_ = prompt.Spin("正在安装 skill 到本机 agent", func() error {
		skillsResult = installSkills()
		return nil
	})

	// If preset.Apply failed mid-way, surface the failure but still
	// emit a partial envelope so agents can program around it.
	if applyErr != nil {
		ctx := map[string]any{
			"preset_id":          flags.preset,
			"resolved_preset_id": applyResult.PresetID,
			"partial_state":      partialState,
			"completed_projects": projectNames(applyResult.Projects),
		}
		return cliErrors.New(cliErrors.STATUS_FIX_FAILED, applyErr.Error()).WithContext(ctx)
	}

	// Step 9: emit the v3 envelope.
	payload := createPresetResult{
		Schema:         "one-cli/create/v3",
		Preset:         presetEnvelope{ID: applyResult.PresetID, Version: preset.SchemaVersion},
		ProjectName:    projectName,
		CreatedPath:    targetDir,
		CreatedInPlace: useCurrentDir,
		PackageManager: options.PackageManager,
		SecretsBackend: effectiveEnv,
		CIEnabled:      true,
		DevEnabled:     true,
		Projects:       presetProjectsPayload(applyResult.Projects),
		DeploySummary:  applyResult.SummarizeDeploys(),
		EnvSummary: envSummary{
			Backend:        effectiveEnv,
			InfisicalBound: infisicalBound,
		},
		Skills:       skillsResult,
		PartialState: partialState,
	}
	if len(applyResult.UnknownSegments) > 0 {
		payload.UnknownSegments = applyResult.UnknownSegments
	}
	output.Emit(&payload)

	if skillsResult.Status == "failed" {
		os.Exit(1)
	}
	return nil
}

// projectNames is a small helper used in the partial-failure envelope
// to list which projects landed before the failure.
func projectNames(projects []preset.ProjectResult) []string {
	out := make([]string, 0, len(projects))
	for _, p := range projects {
		out = append(out, p.Name)
	}
	return out
}

func presetProjectsPayload(projects []preset.ProjectResult) []presetProjectPayload {
	out := make([]presetProjectPayload, 0, len(projects))
	for _, p := range projects {
		out = append(out, presetProjectPayload{
			Name:           p.Name,
			TemplateID:     p.TemplateID,
			TargetPath:     p.TargetPath,
			DeployBackend:  p.DeployBackend,
			Toolchain:      p.Toolchain,
			PackageManager: p.PackageManager,
		})
	}
	return out
}

// createPresetResult is the v3 envelope for `one create --preset`. The
// non-preset path continues to emit createResult (v2) so its snapshot
// fixture stays stable.
type createPresetResult struct {
	Schema          string                 `json:"schema"`
	Preset          presetEnvelope         `json:"preset"`
	ProjectName     string                 `json:"project_name"`
	CreatedPath     string                 `json:"created_path"`
	CreatedInPlace  bool                   `json:"created_in_place"`
	PackageManager  string                 `json:"package_manager"`
	SecretsBackend  string                 `json:"secrets_backend,omitempty"`
	CIEnabled       bool                   `json:"ci_enabled"`
	DevEnabled      bool                   `json:"dev_enabled"`
	Projects        []presetProjectPayload `json:"projects"`
	DeploySummary   map[string]int         `json:"deploy_summary"`
	EnvSummary      envSummary             `json:"env_summary"`
	Skills          skillsPayload          `json:"skills"`
	PartialState    string                 `json:"partial_state"`
	UnknownSegments []string               `json:"preset_unknown_segments,omitempty"`
}

type presetEnvelope struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type presetProjectPayload struct {
	Name           string `json:"name"`
	TemplateID     string `json:"template_id"`
	TargetPath     string `json:"target_path"`
	DeployBackend  string `json:"deploy_backend,omitempty"`
	Toolchain      string `json:"toolchain"`
	PackageManager string `json:"package_manager,omitempty"`
}

type envSummary struct {
	Backend        string `json:"backend"`
	InfisicalBound bool   `json:"infisical_bound"`
}

func (r *createPresetResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	fmt.Fprintf(w, "✓ Workspace created from preset: %s\n", r.Preset.ID)
	fmt.Fprintf(w, "  Path: %s\n", r.CreatedPath)
	for _, p := range r.Projects {
		if p.DeployBackend != "" {
			fmt.Fprintf(w, "  + %s (%s, deploy=%s)\n", p.Name, p.TemplateID, p.DeployBackend)
		} else {
			fmt.Fprintf(w, "  + %s (%s)\n", p.Name, p.TemplateID)
		}
	}
	fmt.Fprintf(w, "  Env: %s\n", r.EnvSummary.Backend)
}

// findEnclosingWorkspace walks up from targetDir looking for a one.manifest.json.
// Checks targetDir itself first (catches `one create .` inside a workspace),
// then each parent. Returns the absolute path of the enclosing workspace, or
// "" when targetDir is not inside any workspace.
func findEnclosingWorkspace(targetDir string) string {
	cur := filepath.Clean(targetDir)
	for {
		if workspace.HasManifest(cur) {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
}
