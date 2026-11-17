package gatewaycmd

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
)

func dispatchCommand() *cobra.Command {
	var selectedProject string
	cmd := &cobra.Command{
		Use:  "dispatch <issue>",
		Args: i18n.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			issue := strings.TrimSpace(args[0])
			if !gateway.ValidIssueReference(issue) {
				return i18n.Errorf("gateway.cli.invalid_issue")
			}
			client, url, err := openClient()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 45*time.Second)
			defer cancel()
			snapshot, err := client.Snapshot(ctx)
			if err != nil {
				return i18n.Errorf("gateway.cli.request_failed", err)
			}
			project, err := dispatchProject(snapshot.Mappings, strings.TrimSpace(selectedProject))
			if err != nil {
				return err
			}
			status, err := client.Dispatch(ctx, project, issue)
			if err != nil {
				return i18n.Errorf("gateway.cli.dispatch_failed", err)
			}
			output.Emit(result{
				Schema: "one-cli/gateway/v1", URL: url,
				Data: dispatchResult{Issue: issue, ProjectID: project, Status: status},
			})
			return nil
		},
	}
	i18n.MarkShort(cmd, "gateway.cli.dispatch")
	i18n.MarkLong(cmd, "gateway.cli.dispatch_long")
	cmd.Flags().StringVar(&selectedProject, "project", "", "")
	i18n.MarkFlagUsage(cmd, "project", "gateway.cli.flag_project")
	return cmd
}

type dispatchResult struct {
	Issue     string         `json:"issue"`
	ProjectID string         `json:"projectId"`
	Status    gateway.Status `json:"status"`
}

func dispatchProject(mappings []gateway.ProjectSummary, selected string) (string, error) {
	switch {
	case len(mappings) == 0:
		return "", i18n.Errorf("gateway.cli.no_mappings")
	case selected == "" && len(mappings) == 1:
		return mappings[0].ProjectID, nil
	case selected == "":
		return "", i18n.Errorf("gateway.cli.project_required")
	case !slices.ContainsFunc(mappings, func(project gateway.ProjectSummary) bool {
		return project.ProjectID == selected
	}):
		return "", i18n.Errorf("gateway.cli.project_unmapped")
	default:
		return selected, nil
	}
}
