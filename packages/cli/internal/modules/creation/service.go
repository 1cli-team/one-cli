// Package creation owns the complete Template-to-Workspace/Project lifecycle.
// Preset parsing stays pure in modules/preset; Cobra owns prompts and rendering.
package creation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/hooks"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/miseconfig"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/preset"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
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
		return nil, errors.New(i18n.T("creation.environment_required"))
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
	HooksWarn       error
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
			i18n.Tf("creation.target_not_empty", displayPath),
		).WithContext(map[string]any{"target_path": targetDir, "display_path": displayPath})
	}
	if enclosing := enclosingWorkspace(targetDir); enclosing != "" {
		return cliErrors.New(
			cliErrors.WORKSPACE_NESTED_FORBIDDEN,
			i18n.Tf("creation.nested_workspace", enclosing),
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
		EnvBackend: strings.TrimSpace(input.EnvBackend), PartialState: "none",
	}
	if !workspace.IsValidProjectName(input.Name) {
		return result, cliErrors.New(
			cliErrors.INVALID_NAME,
			i18n.Tf("creation.workspace_name_invalid", input.Name),
		)
	}
	if result.EnvBackend == "" {
		result.EnvBackend = workspace.EnvBackendDotenv
	}
	if result.EnvBackend != workspace.EnvBackendDotenv && result.EnvBackend != workspace.EnvBackendInfisical {
		return result, cliErrors.New(
			cliErrors.BACKEND_ID_UNKNOWN,
			i18n.Tf("env.provider_invalid", result.EnvBackend),
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
		ProjectName: input.Name,
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
	manifest, err := workspace.ReadManifest(input.TargetDir)
	if err != nil {
		return result, err
	}
	files := fsutil.NewFilePlan(input.TargetDir)
	if err := hooks.PlanFiles(files, manifest); err != nil {
		return result, err
	}
	if err := files.Apply(ctx); err != nil {
		return result, err
	}

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

	plan, err := miseconfig.Build(input.TargetDir, miseconfig.Options{})
	if err == nil {
		err = plan.Apply(ctx)
	}
	if err != nil {
		return result, cliErrors.New(cliErrors.ONE_CLI_ERROR,
			i18n.Tf("creation.mise_warning", err)).
			WithContext(map[string]any{"workspace": input.TargetDir, "partial_state": "mise_configuration_incomplete"})
	}
	if pkg, err := workspace.ReadPackageJSON(input.TargetDir); err == nil && pkg != nil {
		result.PackageManager, _, _ = strings.Cut(pkg.PackageManager, "@")
	}
	if err := initGitRepo(input.TargetDir); err != nil {
		result.HooksWarn = i18n.Errorf("creation.git_failed", err)
	} else {
		install, err := hooks.PlanInstall(ctx, input.TargetDir, "", false)
		if err == nil {
			err = install.Apply(ctx)
		}
		result.HooksWarn = err
	}
	if s.observer != nil {
		result.RegistryWarn = s.observer(ctx, input.TargetDir, "create")
	}
	return result, nil
}

type AddProjectResult struct {
	Project ProjectResult
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
	}, nil
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
