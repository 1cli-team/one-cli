package envcmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

type summaryOutput struct{ *environmentmodule.Summary }

func (r summaryOutput) RenderTTY(w io.Writer) {
	if r.Summary == nil {
		return
	}
	source := r.Source
	if source == workspace.EnvBackendDotenv {
		source = i18n.T("env.source_dotenv")
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

type pullOutput struct{ *environmentmodule.PullResult }

func (r pullOutput) RenderTTY(w io.Writer) {
	if r.PullResult == nil {
		return
	}
	summaryKey := "env.pull.summary"
	if r.DryRun {
		summaryKey = "env.pull.summary_dry_run"
	}
	fmt.Fprintf(w, i18n.T(summaryKey)+"\n", r.Environment, r.WrittenCount, r.SkippedCount)
	for _, entry := range r.PerSubproject {
		mark := "·"
		switch entry.Status {
		case "written", "dry-run":
			mark = "✓"
		case "unchanged":
			mark = "="
		case "skipped":
			mark = "✗"
		}
		line := fmt.Sprintf("  %s %s [%s] → %s", mark, entry.RelativeDir, entry.Status, entry.EnvFilePath)
		if entry.Reason != "" {
			line += " — " + entry.Reason
		}
		fmt.Fprintln(w, line)
		if len(entry.KeysWritten) > 0 {
			fmt.Fprintf(w, i18n.T("env.pull.keys")+"\n", strings.Join(entry.KeysWritten, ", "))
		}
	}
}

type setOutput struct{ *environmentmodule.SetResult }

func (r setOutput) RenderTTY(w io.Writer) {
	if r.SetResult == nil {
		return
	}
	if r.Path != "" {
		fmt.Fprintf(w, i18n.T("env.set_success_remote")+"\n", r.Key, r.Path, r.Environment)
		return
	}
	if r.Environment != "" {
		fmt.Fprintf(w, i18n.T("env.set_success_env")+"\n", r.Key, r.Source, r.Environment)
		return
	}
	fmt.Fprintf(w, i18n.T("env.set_success")+"\n", r.Key, r.Source)
}

type switchOutput struct {
	*environmentmodule.SwitchResult
}

func (r switchOutput) RenderTTY(w io.Writer) {
	if r.SwitchResult == nil {
		return
	}
	fmt.Fprintf(w, i18n.T("env.switch.success")+"\n", r.From, r.To)
	if r.SkippedSync {
		fmt.Fprintln(w, i18n.T("env.switch.skipped_sync"))
		return
	}
	if r.Synced > 0 || r.Conflicts > 0 {
		fmt.Fprintf(w, i18n.T("env.switch.synced")+"\n", r.Synced, r.To)
		if r.Conflicts > 0 {
			fmt.Fprintf(w, i18n.T("env.switch.conflicts")+"\n", r.Conflicts)
		}
	}
}
