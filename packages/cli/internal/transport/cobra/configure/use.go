package configurecmd

import (
	"fmt"
	"io"

	"strings"

	"github.com/spf13/cobra"

	configureapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/configure"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/helpui"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

// ───────────────────── use ─────────────────────

type useResult struct {
	Schema      string `json:"schema"`
	Domain      string `json:"domain"`
	Backend     string `json:"backend"`
	Name        string `json:"name"`
	Scope       string `json:"scope"`
	WorkspaceID string `json:"workspaceId,omitempty"`
	Project     string `json:"project,omitempty"`
}

func (r useResult) RenderTTY(w io.Writer) {
	switch r.Scope {
	case "workspace-project":
		fmt.Fprintf(w, i18n.T("configure.use_project_success")+"\n",
			r.Name, r.WorkspaceID, r.Project, serviceLabel(profile.Domain(r.Domain), r.Backend))
	case "workspace":
		fmt.Fprintf(w, i18n.T("configure.use_workspace_success")+"\n",
			r.Name, r.WorkspaceID, serviceLabel(profile.Domain(r.Domain), r.Backend))
	default:
		fmt.Fprintf(w, i18n.T("configure.use_default_success")+"\n", r.Name, serviceLabel(profile.Domain(r.Domain), r.Backend))
	}
}

func buildUseCmd(profiles *configureapp.ProfileService) *cobra.Command {
	var profileName string
	var bindWorkspace bool
	var projectName string
	cmd := &cobra.Command{
		Use:   "use [service-id] [--profile <name>] [--workspace] [--project <name|path>]",
		Short: i18n.T("configure.use.short"),
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			selection, err := resolveExistingConnection(profiles, args, profileName)
			if err != nil {
				return err
			}
			result := useResult{
				Schema:  "one-cli/configure-use/v1",
				Domain:  string(selection.Domain),
				Backend: selection.Backend,
				Name:    selection.Name,
				Scope:   "default",
			}
			if bindWorkspace || strings.TrimSpace(projectName) != "" {
				activeWorkspace, err := execution.ResolveWorkspace(cmd.Context())
				if err != nil {
					return err
				}
				root := activeWorkspace.Root()
				m := activeWorkspace.Manifest()
				workspaceID := workspace.WorkspaceID(m)
				if strings.TrimSpace(workspaceID) == "" {
					return cliErrors.New(cliErrors.MANIFEST_INVALID,
						"当前 workspace 缺少 one.manifest.json#workspace.id，无法写入本机 workspace profile 绑定。")
				}
				project := strings.TrimSpace(projectName)
				if project != "" {
					if selected, ok := activeWorkspace.Project(project); ok {
						project = selected.Name
					}
				}
				workspaceName := ""
				if m.Workspace != nil {
					workspaceName = m.Workspace.Name
				}
				if err := profiles.BindWorkspaceProfile(workspaceID, workspaceName, root, project, selection.Domain, selection.Backend, selection.Name); err != nil {
					return err
				}
				result.Scope = "workspace"
				result.WorkspaceID = workspaceID
				if project != "" {
					result.Scope = "workspace-project"
					result.Project = project
				}
			} else {
				if err := profiles.SetDefault(selection.Domain, selection.Backend, selection.Name); err != nil {
					return err
				}
			}
			output.Emit(result)
			return nil
		},
		ValidArgsFunction: pairCompletion(profiles),
	}
	cmd.Flags().StringVar(&profileName, "profile", "", i18n.T("configure.flag.profile_existing"))
	cmd.Flags().BoolVar(&bindWorkspace, "workspace", false, i18n.T("configure.flag.workspace"))
	cmd.Flags().StringVarP(&projectName, "project", "p", "", i18n.T("configure.flag.project"))
	i18n.MarkFlagUsage(cmd, "profile", "configure.flag.profile_existing")
	i18n.MarkFlagUsage(cmd, "workspace", "configure.flag.workspace")
	i18n.MarkFlagUsage(cmd, "project", "configure.flag.project")
	helpui.MarkAdvanced(cmd, "profile", "project", "workspace")
	i18n.MarkShort(cmd, "configure.use.short")
	return cmd
}
