package git

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/lifecycle"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
	core "github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/git"
)

type Plugin struct{ *core.Plugin }

func New() *Plugin { return &Plugin{Plugin: core.New()} }

func (p *Plugin) Register(host lifecycle.Host) error {
	service := host.Dependencies.Git
	var directory string
	parent := &cobra.Command{Use: "git"}
	i18n.MarkShort(parent, "git.short")
	parent.PersistentFlags().StringVarP(&directory, "repo", "C", ".", "")
	i18n.MarkFlagUsage(parent, "repo", "git.flag.repo")
	trees := &cobra.Command{Use: "worktree"}
	i18n.MarkShort(trees, "git.worktree.short")
	list := &cobra.Command{Use: "list", Args: i18n.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		result, err := service.List(cmd.Context(), directory)
		if err != nil {
			return err
		}
		output.Emit(&listResult{result})
		return nil
	}}
	i18n.MarkShort(list, "git.list.short")
	var branch string
	var detached bool
	add := &cobra.Command{Use: "add <path> [branch|start-point]", Args: i18n.RangeArgs(1, 2), RunE: func(cmd *cobra.Command, args []string) error {
		input := gitmodule.AddInput{Path: args[0], Branch: branch, NewBranch: cmd.Flags().Changed("branch"), Detached: detached}
		if len(args) == 2 {
			if input.NewBranch || detached {
				input.StartPoint = args[1]
			} else {
				input.Branch = args[1]
			}
		}
		result, err := service.Add(cmd.Context(), directory, input)
		if err != nil {
			return err
		}
		output.Emit(&changeResult{result})
		return nil
	}}
	i18n.MarkShort(add, "git.add.short")
	add.Flags().StringVarP(&branch, "branch", "b", "", "")
	i18n.MarkFlagUsage(add, "branch", "git.flag.branch")
	add.Flags().BoolVar(&detached, "detach", false, "")
	i18n.MarkFlagUsage(add, "detach", "git.flag.detach")
	remove := &cobra.Command{Use: "remove <path|id>", Args: i18n.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		repository, err := service.List(cmd.Context(), directory)
		if err != nil {
			return err
		}
		path := args[0]
		if !filepath.IsAbs(path) {
			path = filepath.Join(directory, path)
		}
		path, err = filepath.Abs(path)
		if err != nil {
			return err
		}
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			path = resolved
		}
		for _, tree := range repository.Worktrees {
			if tree.ID != args[0] && tree.Path != path {
				continue
			}
			if err := service.Remove(cmd.Context(), directory, tree.ID); err != nil {
				return err
			}
			output.Emit(&removedResult{Schema: "one-cli/git-worktree-remove/v1", Path: tree.Path})
			return nil
		}
		return i18n.Errorf("git.not_found", args[0])
	}}
	i18n.MarkShort(remove, "git.remove.short")
	trees.AddCommand(list, add, remove)
	parent.AddCommand(trees, repositoryCommand(service, &directory), cloneCommand(service, &directory))
	host.Root.AddCommand(parent)
	return nil
}

type listResult struct{ *gitmodule.Repository }

func (r *listResult) RenderTTY(w io.Writer) {
	for _, tree := range r.Worktrees {
		branch := tree.Branch
		if tree.Detached {
			branch = i18n.T("git.detached")
		}
		if tree.Bare {
			branch = i18n.T("git.bare")
		}
		state := i18n.T("git.clean")
		if tree.Dirty {
			state = i18n.T("git.dirty")
		}
		if tree.Missing {
			state = i18n.T("git.missing")
		}
		if tree.Locked {
			state = i18n.T("git.locked")
		}
		fmt.Fprintf(w, "%s  %s\n  %s\n", branch, state, tree.Path)
		if tree.StatusError != "" {
			fmt.Fprintln(w, tree.StatusError)
		}
	}
}

type changeResult struct{ *gitmodule.ChangeResult }

func (r *changeResult) RenderTTY(w io.Writer) {
	fmt.Fprintln(w, i18n.Tf("git.created", r.Worktree.Path))
	if r.Warning != "" {
		fmt.Fprintln(w, r.Warning)
	}
}

type removedResult struct {
	Schema string `json:"schema"`
	Path   string `json:"path"`
}

func (r *removedResult) RenderTTY(w io.Writer) { fmt.Fprintln(w, i18n.Tf("git.removed", r.Path)) }

var _ lifecycle.Plugin = (*Plugin)(nil)
