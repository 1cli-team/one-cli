package cli

import (
	"context"
	"errors"
	"os"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/lifecycle"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/execution"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/bootstrap"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/tasks"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/helpui"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/preferences"
	platformprocess "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/process"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/updatecheck"
)

// InitializeStorage preserves the CLI error envelope even before Cobra starts.
func InitializeStorage(args []string) error {
	detectOutputMode(args)
	_ = i18n.Init(i18n.Resolve("auto"))
	_, err := bootstrap.InitializeStorage()
	if err != nil {
		wrapped := cliErrors.New(cliErrors.ONE_CLI_ERROR, err.Error()).WithCause(err)
		output.EmitError(wrapped)
		return wrapped
	}
	return nil
}

// Execute runs one invocation after storage initialization. It owns the
// output-mode detection (-o / --output / TTY), help interception,
// pre-validation of the first positional (so unknown commands emit our
// structured UNKNOWN_COMMAND envelope rather than cobra's generic error),
// and final error envelope emission.
//
// version is the build-time value injected via -ldflags; see cmd/one/main.go.
func (app *Application) Execute(ctx context.Context, version string, args []string) error {
	rootCmd := app.root
	known := func(name string) bool { return knownSubcommand(rootCmd, name) }
	workingDirectory, _ := os.Getwd()
	executionScope := execution.NewScope(ctx, workingDirectory)
	rootCmd.SetContext(execution.WithScope(executionScope.Context(), executionScope))

	rootCmd.Version = version
	args = expandTaskShorthand(args, known)
	rootCmd.SetArgs(args)

	// Output mode detection runs before cobra so subcommands can already
	// query output.IsJSON() during args validation.
	detectOutputMode(args)

	// Resolve the active locale (stored preference > env vars > default)
	// and re-apply every annotated cmd.Short under the new locale.
	// Failure to read preferences.json or to parse a locale bundle is
	// non-fatal — we boot in DefaultLocale (en-US) and continue. The
	// CLI's job is to run user commands, not to refuse to start over a
	// preference file.
	prefs, _ := preferences.Load()
	stored := preferences.LocaleAuto
	if prefs != nil {
		stored = prefs.Locale
	}
	_ = i18n.Init(i18n.Resolve(stored))
	i18n.RefreshTree(rootCmd)

	// Official releases may start an independent update worker. The command
	// never waits for a download; cached completion/failure notices are printed
	// on exit. Development builds, CI, and structured output skip this path.
	if shouldCheckUpdates(args) {
		updatecheck.MaybeRefreshAsync(version)
		defer updatecheck.Notify(version)
	}

	if shouldRenderAllHelp(args) {
		helpui.RenderAll(rootCmd, os.Stdout)
		return nil
	}
	if isBareInvocation(args) {
		root, err := workspace.ResolveProjectRoot("")
		if err != nil {
			return err
		}
		if workspace.HasManifest(root) {
			summary, err := workspace.BuildSummary(root)
			if err != nil {
				var cliErr *output.Error
				if errors.As(err, &cliErr) {
					output.EmitError(cliErr)
					return cliErr
				}
				return err
			}
			available := map[string]bool{}
			if w, e := execution.ResolveWorkspaceScope(execution.NewScope(ctx, root)); e == nil {
				if catalog, _, e := tasks.Catalog(w); e == nil {
					for _, task := range catalog {
						if task.Operation == "dev" {
							available[task.Project] = true
						}
					}
				}
			}
			summary.ApplyTaskCapabilities(available)
			output.Emit(&summary)
			return nil
		}
		// Flags such as `-o text` do not change the bare-command behavior:
		// outside a workspace the concise everyday help is still the useful
		// answer.
		app.renderRootHelp()
		return nil
	}

	// Bypass cobra entirely for root help — emit the curated root help
	// text in the active locale. Subcommand help (`one env --help`)
	// still goes through cobra so each subcommand gets its own
	// auto-generated help with flag list + examples.
	if shouldRenderHelp(args, known) {
		app.renderRootHelp()
		return nil
	}

	// Pre-validate: if the first non-flag token is something that doesn't
	// match a registered subcommand, emit UNKNOWN_COMMAND with remediation
	// rather than cobra's generic "unknown command" error.
	if first, ok := firstPositional(args); ok && !known(first) {
		err := cliErrors.New(
			cliErrors.UNKNOWN_COMMAND,
			i18n.Tf("command.unknown", first),
		).WithContext(map[string]any{"command": "one " + first})
		output.EmitError(err)
		return err
	}

	// Command help is inspection: registration is enough and no plugin resources start.
	if isHelpInvocation(args) {
		return emitError(rootCmd.ExecuteContext(rootCmd.Context()))
	}

	ctx, stop := platformprocess.SignalContext(rootCmd.Context())
	defer stop()
	err := app.lifecycle.Run(ctx, lifecycle.Invocation{Version: version, Args: args}, func(ctx context.Context) error {
		return rootCmd.ExecuteContext(ctx)
	})
	var exit *platformprocess.ExitStatus
	if cause := context.Cause(ctx); errors.As(cause, &exit) {
		err = errors.Join(cause, err)
	}
	return emitError(err)
}

func emitError(err error) error {
	if err == nil {
		return nil
	}
	var exit *platformprocess.ExitStatus
	if errors.As(err, &exit) {
		return err
	}
	var cliErr *output.Error
	if errors.As(err, &cliErr) {
		output.EmitError(cliErr)
		return err
	}
	wrapped := cliErrors.New(cliErrors.ONE_CLI_ERROR, err.Error()).WithCause(err)
	output.EmitError(wrapped)
	return wrapped
}

func (app *Application) renderRootHelp() {
	if app.rootHelp != nil {
		_, _ = os.Stdout.WriteString(app.rootHelp())
		return
	}
	helpui.Render(app.root, nil)
}

func isHelpInvocation(args []string) bool {
	if first, _ := firstPositional(args); first == "" && len(args) > 0 && args[0] == "help" {
		return true
	}
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--help" || arg == "-h" || arg == "--version" {
			return true
		}
	}
	return false
}
