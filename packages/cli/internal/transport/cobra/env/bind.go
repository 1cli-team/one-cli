package envcmd

import (
	"github.com/spf13/cobra"

	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/prompt"
)

func runWorkspaceBind(cmd *cobra.Command, deps Dependencies, projectID string, create bool) error {
	if projectID == "" && !create && output.CanPrompt() {
		projects, err := environmentmodule.Projects(cmd.Context())
		if err != nil {
			return err
		}
		const createChoice = "__create_workspace_project__"
		options := []prompt.Option[string]{{Label: i18n.T("env.bind.create_option"), Value: createChoice}}
		for _, project := range projects {
			options = append(options, prompt.Option[string]{Label: project.Name, Value: project.ID})
		}
		choice, err := prompt.Select(i18n.T("env.bind.select_workspace_project"), options)
		if err != nil {
			return err
		}
		if choice == createChoice {
			create = true
		} else {
			projectID = choice
		}
	}
	var result *environmentmodule.WorkspaceBindingResult
	err := prompt.Spin(i18n.T("env.bind.progress"), func() error {
		var err error
		result, err = deps.Service.BindWorkspace(cmd.Context(), environmentmodule.BindWorkspaceInput{
			Scope: commandScope(cmd), ProjectID: projectID, Create: create,
		})
		return err
	})
	if err != nil {
		return err
	}
	output.Emit(workspaceBindOutput{result})
	return nil
}
