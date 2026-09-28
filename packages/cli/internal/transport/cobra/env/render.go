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

type getOutput struct{ *environmentmodule.GetResult }

func (r getOutput) RenderTTY(w io.Writer) {
	if r.GetResult != nil {
		fmt.Fprintln(w, r.Value)
	}
}

type listOutput struct{ *environmentmodule.ListResult }

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
	fmt.Fprintf(w, i18n.T("env.set_success_remote")+"\n", r.Key, r.Path, r.Environment)
}
