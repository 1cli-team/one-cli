package envcmd

import (
	"fmt"
	"io"
	"strings"

	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

type summaryOutput struct{ *environmentmodule.Summary }

func (r summaryOutput) RenderTTY(w io.Writer) {
	if r.Summary == nil {
		return
	}
	source := r.Source
	if source == "" {
		source = i18n.T("env.not_configured")
	}
	scope := i18n.T("env.scope_workspace")
	if r.Scope == "project" {
		scope = i18n.Tf("env.scope_project", r.Project)
	}
	fmt.Fprintf(w, i18n.T("env.summary_source")+"\n", source)
	fmt.Fprintf(w, i18n.T("env.summary_default")+"\n", r.DefaultEnvironment)
	fmt.Fprintf(w, i18n.T("env.summary_available")+"\n", strings.Join(r.AvailableEnvironments, ", "))
	fmt.Fprintf(w, i18n.T("env.summary_scope")+"\n", scope)
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("env.common_commands"))
	for _, command := range r.Commands {
		fmt.Fprintln(w, "  "+command)
	}
}

type listOutput struct{ *environmentmodule.ListResult }

type bindOutput struct {
	*environmentmodule.GlobalLocation
}

type workspaceBindOutput struct {
	*environmentmodule.WorkspaceBindingResult
}

func (r workspaceBindOutput) RenderTTY(w io.Writer) {
	if r.WorkspaceBindingResult == nil {
		return
	}
	if r.Created {
		if r.RequestedName != "" {
			fmt.Fprintln(w, i18n.Tf("env.project_renamed", r.RequestedName, r.ProjectName))
		}
		fmt.Fprintln(w, i18n.Tf("env.project_created", r.ProjectName))
	}
	name := r.ProjectName
	if name == "" {
		name = r.ProjectID
	}
	fmt.Fprintln(w, i18n.Tf("env.bind.workspace_success", name, r.ProjectID, r.WrittenTo))
}

func (r bindOutput) RenderTTY(w io.Writer) {
	if r.GlobalLocation == nil {
		return
	}
	fmt.Fprintln(w, i18n.Tf("env.bind.success", r.ProjectName, r.ProjectID, r.DefaultEnvironment))
}

func (r listOutput) RenderTTY(w io.Writer) {
	if r.ListResult == nil {
		return
	}
	for _, key := range r.Keys {
		fmt.Fprintln(w, key)
	}
}

type setOutput struct{ *environmentmodule.SetResult }

func (r setOutput) RenderTTY(w io.Writer) {
	if r.SetResult == nil {
		return
	}
	if binding := r.Binding; binding != nil {
		if binding.Created {
			if binding.RequestedName != "" {
				fmt.Fprintf(w, i18n.T("env.project_renamed")+"\n", binding.RequestedName, binding.ProjectName)
			}
			fmt.Fprintf(w, i18n.T("env.project_created")+"\n", binding.ProjectName)
		}
		fmt.Fprintf(w, i18n.T("env.project_target")+"\n", binding.ProjectName, binding.ProjectID)
	}
	fmt.Fprintf(w, i18n.T("env.set_success_remote")+"\n", r.Key, r.Path, r.Environment)
}
