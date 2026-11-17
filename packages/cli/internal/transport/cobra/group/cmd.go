package groupcmd

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/workspacegroups"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
)

func Command(service *workspacegroups.Service) *cobra.Command {
	list := func(cmd *cobra.Command, _ []string) error {
		result, err := service.List(cmd.Context())
		if err != nil {
			return err
		}
		output.Emit(&listResult{Result: result})
		return nil
	}
	root := &cobra.Command{Use: "group", Args: i18n.NoArgs, RunE: list}
	i18n.MarkShort(root, "groups.short")
	i18n.MarkLong(root, "groups.long")
	root.Example = "  one group create my-project\n  one group list\n  one group add my-project <workspace-id>\n  one group show my-project"
	listCmd := &cobra.Command{Use: "list", Args: i18n.NoArgs, RunE: list}
	i18n.MarkShort(listCmd, "groups.list.short")
	show := &cobra.Command{Use: "show <group>", Args: i18n.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		result, err := service.Get(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		output.Emit(&detailResult{Detail: result})
		return nil
	}}
	i18n.MarkShort(show, "groups.show.short")

	create := changeCommand("create <name>", "create", i18n.ExactArgs(1), func(ctx context.Context, args []string) (*workspacegroups.Group, error) {
		return service.Create(ctx, args[0])
	})
	rename := changeCommand("rename <group> <name>", "rename", i18n.ExactArgs(2), func(ctx context.Context, args []string) (*workspacegroups.Group, error) {
		return service.Rename(ctx, args[0], args[1])
	})
	add := changeCommand("add <group> <workspace-id>...", "add", i18n.MinimumNArgs(2), func(ctx context.Context, args []string) (*workspacegroups.Group, error) {
		return service.Add(ctx, args[0], args[1:])
	})
	remove := changeCommand("remove <group> <workspace-id>...", "remove", i18n.MinimumNArgs(2), func(ctx context.Context, args []string) (*workspacegroups.Group, error) {
		return service.Remove(ctx, args[0], args[1:])
	})
	var before string
	move := changeCommand("move <group> <workspace-id>", "move", i18n.ExactArgs(2), func(ctx context.Context, args []string) (*workspacegroups.Group, error) {
		return service.Move(ctx, args[0], args[1], before)
	})
	move.Flags().StringVar(&before, "before", "", "")
	i18n.MarkFlagUsage(move, "before", "groups.flag.before")
	deleteCmd := &cobra.Command{Use: "delete <group>", Args: i18n.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := service.Delete(cmd.Context(), args[0]); err != nil {
			return err
		}
		output.Emit(&changeResult{Schema: "one-cli/workspace-group-change/v1", Action: "delete", Reference: args[0]})
		return nil
	}}
	i18n.MarkShort(deleteCmd, "groups.delete.short")
	root.AddCommand(listCmd, show, create, rename, add, remove, move, deleteCmd)
	return root
}

func changeCommand(use, action string, args cobra.PositionalArgs, change func(context.Context, []string) (*workspacegroups.Group, error)) *cobra.Command {
	command := &cobra.Command{Use: use, Args: args, RunE: func(cmd *cobra.Command, args []string) error {
		group, err := change(cmd.Context(), args)
		if err != nil {
			return err
		}
		output.Emit(&changeResult{Schema: "one-cli/workspace-group-change/v1", Action: action, Group: group})
		return nil
	}}
	i18n.MarkShort(command, "groups."+action+".short")
	return command
}

type listResult struct{ *workspacegroups.Result }

func (r *listResult) RenderTTY(w io.Writer) {
	if len(r.Projects) == 0 {
		fmt.Fprintln(w, i18n.T("groups.empty"))
		return
	}
	for _, project := range r.Projects {
		fmt.Fprintln(w, i18n.Tf("groups.project_summary", project.Name, project.ID, i18n.T("groups.kind."+project.Kind), len(project.WorkspaceIDs)))
		if project.Kind == "workspace" {
			for _, workspace := range r.Workspaces {
				if workspace.EntryID == project.ID {
					fmt.Fprintf(w, "  %s\n", workspace.Root)
					break
				}
			}
		}
	}
}

type detailResult struct{ *workspacegroups.Detail }

func (r *detailResult) RenderTTY(w io.Writer) {
	fmt.Fprintln(w, i18n.Tf("groups.summary", r.Group.Name, r.Group.ID, len(r.Group.WorkspaceIDs)))
	for _, id := range r.Group.WorkspaceIDs {
		name, root := i18n.T("groups.member_missing"), ""
		for _, workspace := range r.Workspaces {
			if workspace.EntryID == id {
				name, root = workspace.Name, workspace.Root
				break
			}
		}
		fmt.Fprintf(w, "  %s  %s\n", id, name)
		if root != "" {
			fmt.Fprintf(w, "    %s\n", root)
		}
	}
}

type changeResult struct {
	Schema    string                 `json:"schema"`
	Action    string                 `json:"action"`
	Group     *workspacegroups.Group `json:"group,omitempty"`
	Reference string                 `json:"reference,omitempty"`
}

func (r *changeResult) RenderTTY(w io.Writer) {
	if r.Group == nil {
		fmt.Fprintln(w, i18n.Tf("groups.deleted", r.Reference))
		return
	}
	fmt.Fprintln(w, i18n.Tf("groups.changed."+r.Action, r.Group.Name))
	fmt.Fprintln(w, r.Group.ID)
}
