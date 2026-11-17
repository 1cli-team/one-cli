package git

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
)

func repositoryCommand(service *gitmodule.Service, directory *string) *cobra.Command {
	parent := &cobra.Command{Use: "repo"}
	i18n.MarkShort(parent, "git.repo.short")
	add := &cobra.Command{Use: "add <path>", Args: i18n.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		entry, err := service.Associate(cmd.Context(), repositoryDirectory(*directory, args[0]))
		if err != nil {
			return err
		}
		output.Emit(&repositoryResult{Schema: "one-cli/git-repository/v1", Repository: entry})
		return nil
	}}
	i18n.MarkShort(add, "git.repo.add.short")
	list := &cobra.Command{Use: "list", Args: i18n.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		result, err := service.Repositories(cmd.Context())
		if err != nil {
			return err
		}
		output.Emit(&repositoriesResult{result})
		return nil
	}}
	i18n.MarkShort(list, "git.repo.list.short")
	forget := &cobra.Command{Use: "forget <id>", Args: i18n.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := service.ForgetRepository(cmd.Context(), args[0]); err != nil {
			return err
		}
		output.Emit(&forgottenResult{Schema: "one-cli/git-repository-forget/v1", ID: args[0]})
		return nil
	}}
	i18n.MarkShort(forget, "git.repo.forget.short")
	locate := &cobra.Command{Use: "locate <id> <path>", Args: i18n.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		entry, err := service.Relocate(cmd.Context(), args[0], repositoryDirectory(*directory, args[1]))
		if err != nil {
			return err
		}
		output.Emit(&repositoryResult{Schema: "one-cli/git-repository/v1", Repository: entry})
		return nil
	}}
	i18n.MarkShort(locate, "git.repo.locate.short")
	parent.AddCommand(add, list, forget, locate)
	return parent
}

func cloneCommand(service *gitmodule.Service, directory *string) *cobra.Command {
	cmd := &cobra.Command{Use: "clone <url> <path>", Args: i18n.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		result, err := service.Clone(cmd.Context(), args[0], repositoryDirectory(*directory, args[1]))
		if err != nil {
			return err
		}
		output.Emit(&cloneResult{result})
		return nil
	}}
	i18n.MarkShort(cmd, "git.clone.short")
	return cmd
}

type repositoryResult struct {
	Schema     string                          `json:"schema"`
	Repository *gitmodule.RegisteredRepository `json:"repository"`
}

func (r *repositoryResult) RenderTTY(w io.Writer) {
	fmt.Fprintln(w, i18n.Tf("git.repository_linked", r.Repository.Name, r.Repository.Path))
	fmt.Fprintln(w, r.Repository.ID)
}

type repositoriesResult struct{ *gitmodule.RepositoriesResult }

func (r *repositoriesResult) RenderTTY(w io.Writer) {
	if len(r.Repositories) == 0 {
		fmt.Fprintln(w, i18n.T("git.repositories_empty"))
	}
	for _, entry := range r.Repositories {
		fmt.Fprintf(w, "%s  %s  %s\n  %s\n", entry.Name, entry.ID, i18n.T("git.repository_status."+entry.Status), entry.Path)
		if entry.Error != "" {
			fmt.Fprintln(w, entry.Error)
		}
	}
}

type forgottenResult struct {
	Schema string `json:"schema"`
	ID     string `json:"id"`
}

func (r *forgottenResult) RenderTTY(w io.Writer) {
	fmt.Fprintln(w, i18n.Tf("git.repository_forgotten", r.ID))
}

type cloneResult struct{ *gitmodule.CloneResult }

func (r *cloneResult) RenderTTY(w io.Writer) {
	fmt.Fprintln(w, i18n.Tf("git.repository_cloned", r.Path))
	if r.Warning != "" {
		fmt.Fprintln(w, r.Warning)
	}
}

func repositoryDirectory(directory, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(directory, path)
}
