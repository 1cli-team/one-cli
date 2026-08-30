package createcmd

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	creationmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/creation"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/preset"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/prompt"
)

// runCreateWithPreset is the --preset code path. The non-preset path
// (runCreate's tail) is unchanged. This function:
//
//  1. Parses the preset id (no FS mutation), runs Resolve against the
//     registry (still no FS mutation).
//  2. Refuses to prompt — `--preset` without a dir argument errors
//     with PROJECT_NAME_REQUIRED rather than dropping into a TTY form.
//  3. Validates the env provider doesn't collide with --env-provider.
//  4. Hands one resolved plan to creation.Service, which owns the same
//     Workspace and Project mutation path as ordinary create/add.
//  6. Emits an envelope with schema=one-cli/create/v3 (the non-preset
//     path keeps v2 unchanged).
func runCreateWithPreset(deps Dependencies, cmd *cobra.Command, cwd, rawDir string, flags *createFlags) error {
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
			fmt.Sprintf("工作区名称格式不合法: %q（来自 --name 或 basename(dir)）", projectName))
	}
	displayPath := relativeOrAbs(cwd, targetDir, useCurrentDir)

	// Step 5: creation owns the complete mutation, including the final target
	// safety check, workspace files, Backend selection, environment preparation,
	// every Template, and best-effort Git initialisation.
	var creationResult creationmodule.WorkspaceResult
	if err := prompt.Spin(i18n.T("create.generating"), func() error {
		var createErr error
		creationResult, createErr = deps.Creation.CreateWorkspace(cmd.Context(), creationmodule.WorkspaceInput{
			TargetDir:      targetDir,
			DisplayPath:    displayPath,
			Name:           projectName,
			EnvBackend:     effectiveEnv,
			CreatedInPlace: useCurrentDir,
			Preset:         &resolved,
			ProjectNames:   customProjectNames,
		})
		return createErr
	}); err != nil {
		if creationResult.Preset.PresetID == "" {
			return err
		}
		ctx := map[string]any{
			"preset_id":          flags.preset,
			"resolved_preset_id": creationResult.Preset.PresetID,
			"partial_state":      creationResult.PartialState,
			"completed_projects": projectNames(creationResult.Preset.Projects),
		}
		return cliErrors.New(cliErrors.STATUS_FIX_FAILED, err.Error()).WithContext(ctx)
	}
	prompt.Step(i18n.Tf("create.generated", displayPath))
	if creationResult.EnvironmentWarn != nil {
		prompt.Step(fmt.Sprintf(
			"Infisical 自动绑定未完成（%v）；首次运行 `one env set/get/list/pull` 时会再尝试一次",
			creationResult.EnvironmentWarn,
		))
	}
	if creationResult.RegistryWarn != nil {
		prompt.Step(i18n.Tf("create.registry_warning", creationResult.RegistryWarn))
	}
	skillsResult := skillsPayload{Status: "skipped", Reason: "manual-install"}

	// Step 6: emit the v3 envelope.
	payload := createPresetResult{
		Schema:         "one-cli/create/v3",
		Preset:         presetEnvelope{ID: creationResult.Preset.PresetID, Version: preset.SchemaVersion},
		ProjectName:    projectName,
		CreatedPath:    creationResult.TargetDir,
		CreatedInPlace: creationResult.CreatedInPlace,
		PackageManager: creationResult.PackageManager,
		SecretsBackend: effectiveEnv,
		CIEnabled:      false,
		DevEnabled:     true,
		Projects:       presetProjectsPayload(creationResult.Preset.Projects),
		DeploySummary:  creationResult.Preset.SummarizeDeploys(),
		EnvSummary: envSummary{
			Backend:        effectiveEnv,
			InfisicalBound: creationResult.InfisicalBound,
		},
		Skills:       skillsResult,
		PartialState: creationResult.PartialState,
	}
	if len(creationResult.Preset.UnknownSegments) > 0 {
		payload.UnknownSegments = creationResult.Preset.UnknownSegments
	}
	output.Emit(&payload)

	return nil
}

// projectNames is a small helper used in the partial-failure envelope
// to list which projects landed before the failure.
func projectNames(projects []creationmodule.ProjectResult) []string {
	out := make([]string, 0, len(projects))
	for _, p := range projects {
		out = append(out, p.Name)
	}
	return out
}

func presetProjectsPayload(projects []creationmodule.ProjectResult) []presetProjectPayload {
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
	fmt.Fprintf(w, i18n.T("create.preset_success")+"\n", r.Preset.ID)
	fmt.Fprintf(w, i18n.T("create.location")+"\n", r.CreatedPath)
	for _, p := range r.Projects {
		if p.DeployBackend != "" {
			fmt.Fprintf(w, i18n.T("create.preset_project_deployed")+"\n", p.Name, p.TemplateID, p.DeployBackend)
		} else {
			fmt.Fprintf(w, i18n.T("create.preset_project")+"\n", p.Name, p.TemplateID)
		}
	}
	fmt.Fprintf(w, i18n.T("create.env_source")+"\n", r.EnvSummary.Backend)
}
