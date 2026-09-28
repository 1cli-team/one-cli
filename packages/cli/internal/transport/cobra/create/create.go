package createcmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	creationmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/creation"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/prompt"
)

func runCreate(deps Dependencies, cmd *cobra.Command, rawDir string, flags *createFlags) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// --preset implies fully non-interactive: the whole point is a
	// reproducible scaffold from a single string. Force -y, parse the
	// preset id up-front, and short-circuit to runCreateWithPreset.
	// Pre-flight (parse + registry resolve) runs BEFORE any
	// filesystem mutation so PRESET_INVALID never leaves a half-baked
	// dir behind.
	if flags.preset != "" {
		flags.yes = true
		return runCreateWithPreset(deps, cmd, cwd, rawDir, flags)
	}

	interactive := !flags.yes && output.CanPrompt()

	// validateDir is the unified target-directory validator: same logic
	// the post-form code path runs (existence + emptiness + nesting),
	// surfaced inside the huh prompt so the user sees the conflict
	// before they finish filling in the form. Without this, you fill in
	// dir + name picks and only THEN learn the directory was
	// already a workspace — bad UX.
	validateDir := func(v string) error {
		v = strings.TrimSpace(v)
		if v == "" {
			return errors.New(i18n.T("create.enter_directory"))
		}
		abs := resolveTargetPath(cwd, v)
		return deps.Creation.ValidateWorkspaceTarget(abs)
	}
	// Form-mode validation allows empty (we'll fall back to basename(dir));
	// but if the user types something, it must parse as a valid name.
	validateNameOptional := func(v string) error {
		v = strings.TrimSpace(v)
		if v == "" {
			return nil
		}
		if !workspace.IsValidProjectName(v) {
			return errors.New(i18n.T("common.name_format"))
		}
		return nil
	}

	// Pre-flight: if the user hasn't told us a directory yet AND cwd is
	// itself inside a workspace, refuse before opening any prompt. The
	// per-field validator above also catches this once they type a path,
	// but failing earlier is friendlier when the answer is "you're in
	// the wrong place to begin with".
	if rawDir == "" && interactive {
		if conflict := deps.Creation.EnclosingWorkspace(cwd); conflict != "" {
			return cliErrors.New(cliErrors.WORKSPACE_NESTED_FORBIDDEN,
				i18n.Tf("create.workspace_nested", conflict)).
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
			Text(&dirInput, i18n.T("create.prompt_dir"), "./my-app", validateDir).
			Text(&nameInput, i18n.T("create.prompt_name"), "", validateNameOptional).
			Run(); err != nil {
			return err
		}
		rawDir = strings.TrimSpace(dirInput)
		flags.name = strings.TrimSpace(nameInput)
	} else if rawDir == "" {
		if !interactive {
			return cliErrors.New(cliErrors.PROJECT_NAME_REQUIRED,
				i18n.T("create.directory_required")).
				WithContext(map[string]any{"interactive": false})
		}
		got, err := prompt.Text(i18n.T("create.prompt_dir"), "./my-app", validateDir)
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
			i18n.Tf("create.name_invalid", projectName))
	}

	displayPath := relativeOrAbs(cwd, targetDir, useCurrentDir)

	// Backend selection: --env-provider flag wins; otherwise interactive
	// prompt asks dotenv vs infisical.
	enables, err := resolveCreateEnables(flags.envProvider, interactive)
	if err != nil {
		return err
	}

	var result creationmodule.WorkspaceResult
	if err := prompt.Spin(i18n.T("create.generating"), func() error {
		var createErr error
		result, createErr = deps.Creation.CreateWorkspace(cmd.Context(), creationmodule.WorkspaceInput{
			TargetDir:      targetDir,
			DisplayPath:    displayPath,
			Name:           projectName,
			EnvBackend:     selectedEnvironmentBackend(enables),
			CreatedInPlace: useCurrentDir,
		})
		return createErr
	}); err != nil {
		return err
	}
	prompt.Step(i18n.Tf("create.generated", displayPath))
	if result.EnvironmentWarn != nil {
		prompt.Step(i18n.Tf(
			"create.infisical_binding_warning",
			result.EnvironmentWarn,
		))
	}
	if result.RegistryWarn != nil {
		prompt.Step(i18n.Tf("create.registry_warning", result.RegistryWarn))
	}

	// v2 envelope: replaces the v1 `enabled_backends []string` with
	// per-domain semantic fields. `secrets_backend` names the env
	// backend ("dotenv" / "infisical"); `ci_enabled` / `dev_enabled` are
	// booleans. Container / deploy are template-driven and live on the
	// subproject record, not in this envelope.
	secretsBackend := ""
	devEnabled := false
	for _, id := range enables {
		switch {
		case strings.HasPrefix(id, "env/"):
			secretsBackend = strings.TrimPrefix(id, "env/")
		case strings.HasPrefix(id, "dev/"):
			devEnabled = true
		}
	}
	payload := createResult{
		Schema:         "one-cli/create/v2",
		ProjectName:    projectName,
		CreatedPath:    result.TargetDir,
		displayPath:    displayPath,
		CreatedInPlace: result.CreatedInPlace,
		PackageManager: result.PackageManager,
		SecretsBackend: secretsBackend,
		DevEnabled:     devEnabled,
	}
	if result.HooksWarn != nil {
		payload.Warnings = []string{i18n.Tf("create.hooks_warning", result.HooksWarn)}
	}
	output.Emit(&payload)

	return nil
}
