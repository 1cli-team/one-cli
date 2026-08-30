// Package creation owns the complete Template-to-Workspace/Project lifecycle.
// Preset parsing stays pure in modules/preset; Cobra owns prompts and rendering.
package creation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/ai"
	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/preset"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

type Service struct {
	environments *environmentmodule.Service
	observer     WorkspaceObserver
}

// WorkspaceObserver is the optional machine-local discovery hook invoked
// after a workspace has been created successfully. Discovery is auxiliary:
// a registry failure must never roll back an otherwise valid workspace.
type WorkspaceObserver func(context.Context, string, string) error

func NewService(
	environments *environmentmodule.Service,
	observers ...WorkspaceObserver,
) (*Service, error) {
	if environments == nil {
		return nil, errors.New("creation: environment service is required")
	}
	var observer WorkspaceObserver
	if len(observers) > 0 {
		observer = observers[0]
	}
	return &Service{environments: environments, observer: observer}, nil
}

type WorkspaceInput struct {
	TargetDir      string
	DisplayPath    string
	Name           string
	EnvBackend     string
	CreatedInPlace bool
	Preset         *preset.ResolvedSpec
	ProjectNames   []string
}

type WorkspaceResult struct {
	Name            string
	TargetDir       string
	CreatedInPlace  bool
	PackageManager  string
	EnvBackend      string
	InfisicalBound  bool
	EnvironmentWarn error
	RegistryWarn    error
	Preset          PresetResult
	PartialState    string
}

// ValidateWorkspaceTarget performs the same final safety check used by
// CreateWorkspace. Cobra may call it while a form is open for early feedback;
// the mutation path always rechecks it to avoid stale validation.
func (s *Service) ValidateWorkspaceTarget(targetDir string) error {
	return validateWorkspaceTarget(targetDir, targetDir)
}

// EnclosingWorkspace reports the nearest existing One workspace for target.
// It is exposed only so Cobra can fail before opening an interactive form;
// CreateWorkspace performs the authoritative check again.
func (s *Service) EnclosingWorkspace(targetDir string) string {
	return enclosingWorkspace(targetDir)
}

func validateWorkspaceTarget(targetDir, displayPath string) error {
	empty, err := isDirectoryEmpty(targetDir)
	if err != nil {
		return err
	}
	if !empty {
		return cliErrors.New(
			cliErrors.EXISTING_TARGET_NOT_EMPTY,
			fmt.Sprintf("目标目录 %s 已存在且非空。请删除目录后重试，或换一个目标位置。", displayPath),
		).WithContext(map[string]any{"target_path": targetDir, "display_path": displayPath})
	}
	if enclosing := enclosingWorkspace(targetDir); enclosing != "" {
		return cliErrors.New(
			cliErrors.WORKSPACE_NESTED_FORBIDDEN,
			fmt.Sprintf("拒绝在已存在的工作区里创建新工作区：%s 已经是一个 one workspace。", enclosing),
		).WithContext(map[string]any{
			"target_path": targetDir, "enclosing_workspace": enclosing,
		})
	}
	return nil
}

// CreateWorkspace is the single workspace-creation mutation boundary used by
// ordinary create and create --preset.
func (s *Service) CreateWorkspace(ctx context.Context, input WorkspaceInput) (WorkspaceResult, error) {
	result := WorkspaceResult{
		Name: input.Name, TargetDir: input.TargetDir, CreatedInPlace: input.CreatedInPlace,
		PackageManager: "pnpm", EnvBackend: strings.TrimSpace(input.EnvBackend), PartialState: "none",
	}
	if !workspace.IsValidProjectName(input.Name) {
		return result, cliErrors.New(
			cliErrors.INVALID_NAME,
			fmt.Sprintf("工作区名称格式不合法: %q", input.Name),
		)
	}
	if result.EnvBackend == "" {
		result.EnvBackend = workspace.EnvBackendDotenv
	}
	if result.EnvBackend != workspace.EnvBackendDotenv && result.EnvBackend != workspace.EnvBackendInfisical {
		return result, cliErrors.New(
			cliErrors.BACKEND_ID_UNKNOWN,
			fmt.Sprintf("--env-provider 值无效: %q（合法值: dotenv / infisical）", result.EnvBackend),
		)
	}
	displayPath := input.DisplayPath
	if displayPath == "" {
		displayPath = input.TargetDir
	}
	if err := validateWorkspaceTarget(input.TargetDir, displayPath); err != nil {
		return result, err
	}

	_, statErr := os.Stat(input.TargetDir)
	createdFromScratch := os.IsNotExist(statErr)
	if err := generateWorkspaceFiles(input.TargetDir, workspaceFilesOptions{
		ProjectName: input.Name, PackageManager: result.PackageManager,
	}); err != nil {
		if createdFromScratch && !input.CreatedInPlace {
			_ = os.RemoveAll(input.TargetDir)
		}
		return result, err
	}

	enables := []string{"dev/process", "env/" + result.EnvBackend}
	if err := workspace.ApplyBackendSelection(input.TargetDir, enables); err != nil {
		return result, cliErrors.New(cliErrors.BACKEND_ID_UNKNOWN, err.Error()).
			WithContext(map[string]any{"enabled_backends": enables})
	}
	environment, err := s.environments.PrepareWorkspace(ctx, environmentmodule.PrepareWorkspaceInput{
		ProjectRoot: input.TargetDir,
		ProjectName: input.Name,
		Backend:     result.EnvBackend,
	})
	if err != nil {
		return result, err
	}
	result.InfisicalBound = environment.InfisicalBound
	result.EnvironmentWarn = environment.BindWarning

	if input.Preset != nil {
		applied, applyErr := ApplyPreset(ctx, input.TargetDir, *input.Preset, PresetOptions{
			ProjectNames: input.ProjectNames,
		})
		result.Preset = applied
		if applyErr != nil {
			if len(applied.Projects) > 0 {
				result.PartialState = "partial_projects"
			}
			_ = initGitRepo(input.TargetDir)
			return result, applyErr
		}
	}

	_ = initGitRepo(input.TargetDir)
	if s.observer != nil {
		result.RegistryWarn = s.observer(ctx, input.TargetDir, "create")
	}
	return result, nil
}

type AddProjectResult struct {
	Project ProjectResult
	Guides  ai.RefreshResult
}

func (s *Service) AddProject(
	ctx context.Context,
	projectRoot string,
	input ProjectInput,
) (AddProjectResult, error) {
	project, err := materializeProject(ctx, projectRoot, input)
	if err != nil {
		return AddProjectResult{}, err
	}
	return AddProjectResult{
		Project: project,
		Guides:  ai.Refresh(projectRoot, false),
	}, nil
}

func (s *Service) ConfigureProjectDeployment(
	ctx context.Context,
	projectRoot string,
	tpl *template.Template,
	projectName string,
	backend string,
) error {
	return configureProjectDeployment(ctx, projectRoot, tpl, projectName, backend)
}

func enclosingWorkspace(targetDir string) string {
	current := filepath.Clean(targetDir)
	for {
		if workspace.HasManifest(current) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}
