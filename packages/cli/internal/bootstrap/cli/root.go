// Package cli is the CLI harness: it owns the root cobra command, the
// Execute entry point, output-mode detection (-o / --output / TTY),
// help interception, pre-cobra unknown-command handling, and the final
// error envelope. Subcommand implementations live in sibling packages
// under internal/transport/cobra/ and are wired by newRootCommand below.
//
// Layering rule: this package may call into internal/core/workspace,
// internal/core/template, etc. for harness-level needs, but those packages
// must NOT import this one back. CLI concerns (args parsing, TTY
// rendering, JSON emission, output mode) live here; everything below
// is pure.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/helpui"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/preferences"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/updatecheck"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/add"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/ci"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/configure"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/container"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/create"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/deploy"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/dev"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/env"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/run"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/serve"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/skills"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/templates"
)

func newRootCommand() *cobra.Command {
	deps := composeDependencies()
	root := &cobra.Command{
		Use:           "one",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	groups := [][]*cobra.Command{
		addcmd.Commands(deps.creation),
		cicmd.Commands(deps.ci),
		configurecmd.Commands(deps.catalog, deps.profiles, deps.workspaces, deps.registry),
		containercmd.Commands(containercmd.Dependencies{
			Service: deps.containers,
		}),
		createcmd.Commands(createcmd.Dependencies{Creation: deps.creation}),
		deploycmd.Commands(deploycmd.Dependencies{
			Catalog:    deps.catalog,
			Profiles:   deps.profiles,
			Creation:   deps.creation,
			NewService: deps.newDeploymentService,
		}),
		devcmd.Commands(),
		envcmd.Commands(envcmd.Dependencies{Service: deps.environments}),
		runcmd.Commands(deps.loaders),
		servecmd.Commands(servecmd.Dependencies{
			Catalog: deps.catalog, Profiles: deps.profiles, Workspaces: deps.workspaces,
			Registry: deps.registry, Manifest: deps.manifest, Environments: deps.environments,
		}),
		skillscmd.Commands(),
		templatescmd.Commands(),
	}
	for _, commands := range groups {
		root.AddCommand(commands...)
	}
	i18n.MarkShort(root, "root.short")
	root.SetVersionTemplate("{{.Version}}\n")
	root.SetHelpFunc(helpui.Render)
	root.PersistentFlags().StringP("output", "o", "", i18n.T("common.flag.output"))
	i18n.MarkFlagUsage(root, "output", "common.flag.output")
	return root
}

// rootCmd is built by the explicit composition root above. It remains a
// singleton only because Cobra mutates parsed flag state; backend services and
// registries are command-tree instances, not package globals.
var rootCmd = newRootCommand()

// RootCmd returns the configured root cobra command for read-only
// introspection by tools (e.g. verify-cli-references walks the command
// tree to validate `one <subcmd>` references in documentation). The command
// graph has already been assembled explicitly by newRootCommand.
func RootCmd() *cobra.Command { return rootCmd }

// RootHelp returns the curated rootHelp text — what `one --help` emits.
// Exposed for verify-help, which ensures the concise daily command list
// stays intentional. The text is locale-dependent;
// callers that need the deterministic English form should call
// i18n.Init("en-US") first.
func RootHelp() string { return i18n.T("root.help") }

// Execute is the single entry point called by cmd/one/main.go. It owns the
// output-mode detection (-o / --output / TTY), help interception,
// pre-validation of the first positional (so unknown commands emit our
// structured UNKNOWN_COMMAND envelope rather than cobra's generic error),
// and final error envelope emission.
//
// version is the build-time value injected via -ldflags; see cmd/one/main.go.
func Execute(version string, args []string) (resultErr error) {
	workingDirectory, _ := os.Getwd()
	executionScope := execution.NewScope(context.Background(), workingDirectory)
	rootCmd.SetContext(execution.WithScope(executionScope.Context(), executionScope))
	defer func() {
		if closeErr := executionScope.Close(context.Background()); resultErr == nil && closeErr != nil {
			resultErr = closeErr
		}
	}()

	rootCmd.Version = version
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

	// Background update check kicks off here so its goroutine has the
	// whole command runtime to finish; the notification (if any) prints
	// in the defer below from cached state. Both calls are no-ops on
	// CI / -o json / dev builds / opt-out, so this is free in those
	// paths. See internal/platform/updatecheck.
	updatecheck.MaybeRefreshAsync(version)
	defer updatecheck.Notify(version)

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
			output.Emit(&summary)
			return nil
		}
		// Flags such as `-o text` do not change the bare-command behavior:
		// outside a workspace the concise everyday help is still the useful
		// answer.
		os.Stdout.WriteString(i18n.T("root.help"))
		return nil
	}

	// Bypass cobra entirely for root help — emit the curated root help
	// text in the active locale. Subcommand help (`one env --help`)
	// still goes through cobra so each subcommand gets its own
	// auto-generated help with flag list + examples.
	if shouldRenderRootHelp(args) {
		os.Stdout.WriteString(i18n.T("root.help"))
		return nil
	}

	// Pre-validate: if the first non-flag token is something that doesn't
	// match a registered subcommand, emit UNKNOWN_COMMAND with remediation
	// rather than cobra's generic "unknown command" error.
	if first, ok := firstPositional(args); ok && !isKnownSubcommand(first) {
		err := cliErrors.New(
			cliErrors.UNKNOWN_COMMAND,
			fmt.Sprintf("未知命令: %s", first),
		).WithContext(map[string]any{"command": "one " + first})
		output.EmitError(err)
		return err
	}

	if err := rootCmd.Execute(); err != nil {
		// Wrap unknown errors into the structured envelope so JSON consumers
		// always see one-cli/error/v1.
		var cliErr *output.Error
		if errors.As(err, &cliErr) {
			output.EmitError(cliErr)
			return cliErr
		}
		wrapped := cliErrors.New(cliErrors.ONE_CLI_ERROR, err.Error())
		output.EmitError(wrapped)
		return wrapped
	}
	return nil
}

// helpFlags catches the tokens we treat as a request for help.
var helpFlags = map[string]struct{}{
	"-h":     {},
	"--help": {},
	"help":   {},
}

// shouldRenderRootHelp reports whether `args` should bypass cobra and
// render the curated rootHelp text. Returns true for root help requests.
// A bare invocation is handled earlier: it renders a workspace summary
// inside a workspace and this help outside one.
//
// Returns false for help on a subcommand (`one env --help`,
// `one help create`) so cobra's per-subcommand help template runs.
func shouldRenderRootHelp(args []string) bool {
	if len(args) == 0 {
		return true
	}
	first := args[0]
	if _, ok := helpFlags[first]; !ok {
		return false
	}
	// `one help <known-subcommand>` should fall through to cobra so the
	// subcommand's own help is rendered.
	if len(args) > 1 && isKnownSubcommand(args[1]) {
		return false
	}
	return true
}

func shouldRenderAllHelp(args []string) bool {
	return len(args) == 2 && args[0] == "help" && args[1] == "--all"
}

func isBareInvocation(args []string) bool {
	if len(args) == 0 {
		return true
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-o" || a == "--output":
			i++
			if i >= len(args) {
				return false
			}
		case strings.HasPrefix(a, "--output="), strings.HasPrefix(a, "-o="):
		case strings.HasPrefix(a, "-o") && len(a) > 2:
		default:
			return false
		}
	}
	return true
}

// firstPositional returns the first arg that doesn't look like a flag.
// Flags / help tokens / version tokens are not subcommands.
//
// We must also skip the *value* of value-taking persistent flags
// (`-o json` / `--output json`) — otherwise `one -o json templates`
// would treat "json" as the subcommand and emit UNKNOWN_COMMAND.
// The equals / concatenated forms (`-o=json`, `-ojson`, `--output=json`)
// are already covered by the `a[0] == '-'` skip above.
func firstPositional(args []string) (string, bool) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "" {
			continue
		}
		if a[0] == '-' {
			if a == "-o" || a == "--output" {
				i++ // also skip the next token (the flag's value)
			}
			continue
		}
		// Cobra accepts `help` as a synonym for --help; let it through to
		// cobra so the per-subcommand help still works (`one help create`).
		if a == "help" {
			return "", false
		}
		return a, true
	}
	return "", false
}

// isKnownSubcommand reports whether name is registered on the root cobra
// command. We inspect the explicitly assembled rootCmd.Commands() collection
// so this validation never maintains a second command-name list.
func isKnownSubcommand(name string) bool {
	for _, c := range rootCmd.Commands() {
		if c.Name() == name {
			return true
		}
		for _, alias := range c.Aliases {
			if alias == name {
				return true
			}
		}
	}
	return false
}

// detectOutputMode resolves the active output mode. Order:
//  1. -o / --output flag (kubectl-style; values json | yaml | text)
//  2. ModeAuto — TTY auto-detect at decision time (pipe → JSON, terminal → text)
//
// We scan os.Args manually because subcommand arg validation can call
// output.IsJSON() before cobra has parsed the flag set.
func detectOutputMode(args []string) {
	if v := scanOutputValue(args); v != "" {
		switch strings.ToLower(v) {
		case "json":
			output.SetMode(output.ModeJSON)
		case "yaml":
			output.SetMode(output.ModeYAML)
		case "text":
			output.SetMode(output.ModeTTY)
		}
	}
}

// scanOutputValue extracts the value of -o / --output from args,
// honouring every form cobra accepts:
//
//	-o json
//	-o=json
//	-ojson
//	--output json
//	--output=json
//
// Returns "" when no flag is present. Validation of the value is the
// caller's responsibility — unknown values silently fall through to
// ModeAuto, matching kubectl's leniency.
func scanOutputValue(args []string) string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-o" || a == "--output":
			if i+1 < len(args) {
				return args[i+1]
			}
		case strings.HasPrefix(a, "--output="):
			return strings.TrimPrefix(a, "--output=")
		case strings.HasPrefix(a, "-o=") && len(a) > 3:
			return strings.TrimPrefix(a, "-o=")
		case strings.HasPrefix(a, "-o") && len(a) > 2:
			// -ojson concatenated form
			return a[2:]
		}
	}
	return ""
}

// (Previously: the rootHelp constant. The curated help text now lives
// in internal/platform/i18n/locales/{en-US,zh-CN}.json under the "root.help"
// key. Edit the JSON, not Go. The text is rendered via i18n.T at
// runtime in Execute().)
