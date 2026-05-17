package processorch

// ops.go exposes Start as the package-level entry point for `one dev`.
// Behaviour summary:
//   - Reads the workspace manifest at <projectRoot>/one.manifest.json
//   - Walks projects[] and gathers each project's domains.dev.command
//   - Wraps each command as `one run -p <relativeDir> -- <cmd>` so
//     per-project secrets injection still happens
//   - Runs the built-in supervisor (supervisor_unix.go on Unix, stub on
//     other platforms)
//
// Procfile.dev is no longer written or read. External Procfile runners
// (overmind / hivemind / foreman / honcho) are no longer probed —
// `one dev` is self-contained.

import (
	"context"
	"fmt"
	"os"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// StartInput addresses Start.
type StartInput struct {
	ProjectRoot string
	DryRun      bool
	// Process, when non-empty, restricts the supervisor to a single
	// project entry by manifest project name.
	Process string
}

// StartResult is the Start envelope.
type StartResult struct {
	Schema string   `json:"schema"`
	Argv   []string `json:"argv"`
	// Runner is always "builtin" now — kept for forward-compat with
	// JSON consumers that switch on it.
	Runner  string `json:"runner"`
	DryRun  bool   `json:"dry_run"`
	Process string `json:"process,omitempty"`
}

// Start launches the built-in supervisor against the projects declared
// in the workspace manifest. Each project's dev command comes from
// projects[].domains.dev.command (written by `one add`). Returns the
// synthetic argv for dry-run / JSON envelopes; blocks until the
// supervisor exits otherwise.
func Start(ctx context.Context, in StartInput) (*StartResult, error) {
	m, err := workspace.ReadManifest(in.ProjectRoot)
	if err != nil {
		return nil, err
	}
	entries := buildEntriesFromManifest(m, in.Process)
	if len(entries) == 0 {
		return nil, cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND,
			selectorErrorMessage(m, in.Process))
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
	if in.DryRun {
		return res, nil
	}
	if err := runBuiltin(ctx, in.ProjectRoot, entries, BuiltinOpts{Out: os.Stdout}); err != nil {
		return nil, err
	}
	return res, nil
}

// buildEntriesFromManifest walks m.Projects in declaration order,
// gathers each project's domains.dev.command, and wraps it with
// `one run -p <relativeDir> -- <cmd>` so the secrets injection (dotenv
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
			Cmd:  fmt.Sprintf("one run -p %s -- %s", p.RelativeDir, cmd),
		})
	}
	return entries
}

// selectorErrorMessage explains why no entries matched. Different copy
// depending on whether the user asked for one specific project vs no
// filter at all.
func selectorErrorMessage(m *workspace.Manifest, selector string) string {
	if selector != "" {
		return fmt.Sprintf("项目 %q 在 one.manifest.json 里没有声明 dev 命令。"+
			"重新 `one add %s` 让模板写入，或手工编辑 projects[].domains.dev.command。", selector, selector)
	}
	if m == nil || len(m.Projects) == 0 {
		return "工作区里还没有任何项目。先 `one add <template>` 加一个再跑 one dev。"
	}
	return "工作区里没有项目声明 dev 命令。" +
		"重新 `one add <template>` 让 dev 配置重建，或手工编辑 one.manifest.json 的 projects[].domains.dev.command。"
}
