package processorch

// Start resolves manifest commands and delegates execution to the shared task
// session. one exec remains the owner of runtime and per-project environment.

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/taskrun"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

// ProcEntry is a manifest task before terminal execution is selected.
type ProcEntry struct {
	Name, Cmd string
	Argv      []string
}

// StartInput addresses Start.
type StartInput struct {
	Environment string
	Runtime     string
	ProjectRoot string
	DryRun      bool
	// Process, when non-empty, restricts the supervisor to a single
	// project entry by manifest project name.
	Process   string
	Processes []string
	UI        taskrun.Mode
	KeepGoing bool
}

// StartResult is the Start envelope.
type StartResult struct {
	Runtime string   `json:"runtime,omitempty"`
	Schema  string   `json:"schema"`
	Argv    []string `json:"argv"`
	// Runner is always "builtin" now — kept for forward-compat with
	// JSON consumers that switch on it.
	Runner    string   `json:"runner"`
	DryRun    bool     `json:"dry_run"`
	Process   string   `json:"process,omitempty"`
	Processes []string `json:"processes,omitempty"`
}

// Start launches the built-in supervisor against the projects declared
// in the workspace manifest. Each project's dev command comes from
// projects[].domains.dev.command (written by `one add`). Returns the
// synthetic argv for dry-run / JSON envelopes; blocks until the
// supervisor exits otherwise.
func Start(ctx context.Context, in StartInput) (*StartResult, error) {
	if err := runtimeport.Validate(in.Runtime); err != nil {
		return nil, err
	}
	m, err := workspace.ReadManifest(in.ProjectRoot)
	if err != nil {
		return nil, err
	}
	selectors := in.Processes
	if len(selectors) == 0 && in.Process != "" {
		selectors = []string{in.Process}
	}
	entries, err := EntriesForProjects(m, selectors)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND,
			selectorErrorMessage(m, in.Process))
	}

	{
		binary, err := os.Executable()
		if err != nil {
			return nil, err
		}
		for i := range entries {
			command := workspace.ProjectDev(m, entries[i].Name)
			shell := []string{"sh", "-c", command}
			if runtime.GOOS == "windows" {
				comspec := os.Getenv("ComSpec")
				if comspec == "" {
					comspec = "cmd.exe"
				}
				shell = []string{comspec, "/d", "/s", "/c", command}
			}
			args := []string{binary, "exec", "--project", entries[i].Name}
			if in.Environment != "" {
				args = append(args, "--env", in.Environment)
			}
			entries[i].Argv = append(append(args, "--"), shell...)
			entries[i].Cmd = strings.Join(entries[i].Argv, " ")
		}
	}
	argv := []string{"<one cli builtin supervisor>"}
	for _, e := range entries {
		argv = append(argv, e.Name+"="+e.Cmd)
	}
	res := &StartResult{
		Schema:  SchemaStart,
		Argv:    argv,
		Runner:  builtinRunnerID,
		DryRun:  in.DryRun,
		Process: in.Process,
	}
	if in.Runtime == runtimeport.Mise {
		res.Runtime = runtimeport.Mise
	}
	if len(selectors) == 1 {
		res.Process = selectors[0]
	} else if len(selectors) > 1 {
		res.Processes = selectors
	}
	if in.DryRun {
		return res, nil
	}
	tasks := make([]taskrun.Task, 0, len(entries))
	for _, e := range entries {
		tasks = append(tasks, taskrun.Task{Name: e.Name, Directory: in.ProjectRoot, Argv: e.Argv})
	}
	log := os.Stdout
	if output.IsStructured() {
		log = os.Stderr
	}
	if _, err := taskrun.Run(ctx, tasks, taskrun.Options{Mode: in.UI, Title: "dev", Development: true, KeepGoing: in.KeepGoing, Output: log}); err != nil {
		return nil, err
	}
	return res, nil
}

// buildEntriesFromManifest walks m.Projects in declaration order,
// gathers each project's domains.dev.command, and wraps it with
// `one exec -p <relativeDir> -- <cmd>` so the secrets injection (dotenv
// or infisical) configured by `one env` still runs per-project. When
// selector is non-empty, only the matching project is returned. When
// selector is "", projects without a dev command are skipped silently.
func buildEntriesFromManifest(m *workspace.Manifest, selector string) []ProcEntry {
	if m == nil {
		return nil
	}
	entries := make([]ProcEntry, 0, len(m.Projects))
	for _, p := range m.Projects {
		if selector != "" && p.Name != selector {
			continue
		}
		cmd := workspace.ProjectDev(m, p.Name)
		if cmd == "" {
			continue
		}
		entries = append(entries, ProcEntry{
			Name: p.Name,
			Cmd:  fmt.Sprintf("one exec -p %s -- %s", p.RelativeDir, cmd),
		})
	}
	return entries
}

// selectorErrorMessage explains why no entries matched. Different copy
// depending on whether the user asked for one specific project vs no
// filter at all.
func selectorErrorMessage(m *workspace.Manifest, selector string) string {
	if selector != "" {
		return i18n.Tf("dev.project_command_missing", selector, selector)
	}
	if m == nil || len(m.Projects) == 0 {
		return i18n.T("dev.no_projects")
	}
	return i18n.T("dev.commands_missing")
}

// EntriesForProjects validates every explicit selection before any installation.
func EntriesForProjects(m *workspace.Manifest, selectors []string) ([]ProcEntry, error) {
	if len(selectors) == 0 {
		return buildEntriesFromManifest(m, ""), nil
	}
	var entries []ProcEntry
	seen := map[string]bool{}
	for _, name := range selectors {
		if seen[name] {
			continue
		}
		seen[name] = true
		matching := buildEntriesFromManifest(m, name)
		if len(matching) == 0 {
			return nil, cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND, selectorErrorMessage(m, name))
		}
		entries = append(entries, matching...)
	}
	return entries, nil
}
