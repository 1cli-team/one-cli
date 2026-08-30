package cicmd

import (
	"fmt"
	"io"

	ciapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/ci"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	pkgci "github.com/torchstellar-team/one-cli/packages/cli/pkg/ci"
)

type statusOutput struct{ *ciapp.StatusResult }

func (r statusOutput) RenderTTY(w io.Writer) {
	if r.StatusResult == nil {
		return
	}
	state := i18n.T("ci.status.not_configured")
	if r.Configured {
		state = providerLabel(r.Provider)
	}
	fmt.Fprintf(w, "%s%s\n", i18n.T("ci.status.heading"), state)
	fmt.Fprintln(w, i18n.T("ci.status.projects"))
	if len(r.Projects) == 0 {
		fmt.Fprintln(w, i18n.T("ci.status.no_projects"))
	}
	for _, project := range r.Projects {
		state := i18n.T("ci.status.disabled")
		path := ""
		if project.Enabled {
			state = i18n.T("ci.status.enabled")
			path = "  " + project.WorkflowPath
		}
		fmt.Fprintf(w, "  %-16s %s%s\n", project.Name, state, path)
	}
	if r.NextCommand != "" {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%s%s\n", i18n.T("ci.status.next"), r.NextCommand)
	}
}

type actionOutput struct{ *ciapp.ActionResult }

func (r actionOutput) RenderTTY(w io.Writer) {
	if r.ActionResult == nil {
		return
	}
	switch r.Action {
	case "enable":
		fmt.Fprintf(w, i18n.T("ci.enable.success")+"\n", len(r.Projects))
		fmt.Fprintf(w, "%s%s\n", i18n.T("ci.action.service"), providerLabel(r.Provider))
	case "sync":
		fmt.Fprintf(w, i18n.T("ci.sync.success")+"\n", len(r.Projects))
	case "disable":
		changed := 0
		for _, project := range r.Projects {
			if project.Changed {
				changed++
			}
		}
		fmt.Fprintf(w, i18n.T("ci.disable.success")+"\n", changed)
	}
	for _, project := range r.Projects {
		label := i18n.T("ci.action.unchanged")
		if project.Changed {
			label = i18n.T("ci.action." + project.Status)
		}
		fmt.Fprintf(w, "  %s  %s  %s\n", project.Name, label, project.WorkflowPath)
	}
	if r.NextCommand != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, i18n.T("common.next_steps"))
		fmt.Fprintf(w, "  %s\n", r.NextCommand)
	}
}

func providerLabel(provider string) string {
	if provider == pkgci.DefaultProviderID {
		return "GitHub Actions"
	}
	return provider
}
