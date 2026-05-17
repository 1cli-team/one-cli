// Package cli is the CLI harness: it owns the root cobra command, the
// Execute entry point, output-mode detection (-o / --output / TTY),
// help interception, pre-cobra unknown-command handling, and the final
// error envelope. Subcommand implementations live in sibling packages
// under internal/cmd/<name>cmd/; each contributes via the cliexts
// registry (see internal/cliexts) and is wired in here by side-effect
// import. cliexts.Mount(rootCmd) below realises every registration
// onto root in alphabetical order.
//
// Layering rule: this package may call into internal/workspace,
// internal/template, etc. for harness-level needs, but those packages
// must NOT import this one back. CLI concerns (args parsing, TTY
// rendering, JSON emission, output mode) live here; everything below
// is pure.
package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/cliexts"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/preferences"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/updatecheck"

	// Side-effect imports: per-command cobra packages. Each registers
	// its top-level command via cliexts.Register() in its own init();
	// cliexts.Mount(rootCmd) below realises them onto root in a stable
	// alphabetical order. Adding a new command means adding one import
	// line here and one new package under internal/cmd/.
	//
	// Per-domain dispatch (container → Dockerfile, dev → Procfile.dev,
	// deploy → kustomize/s3, env → dotenv/infisical) happens inside the
	// command packages, not at this layer. configurecmd contributes the
	// cross-domain `one configure` tree; skillscmd contributes
	// `one skills install`.
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/cmd/addcmd"
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/cmd/configurecmd"
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/cmd/containercmd"
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/cmd/createcmd"
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/cmd/deploycmd"
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/cmd/devcmd"
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/cmd/envcmd"
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/cmd/runcmd"
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/cmd/servecmd"
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/cmd/skillscmd"
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/cmd/templatescmd"

	// Side-effect imports for the secrets loaders. secrets.Register puts
	// the loader in the registry so `one run --env-provider` finds it.
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/secrets/dotenv"
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/secrets/infisical"

	// Side-effect import: register the bundled toolchain adapters
	// (Node + Go) into pkg/toolchain's registry. Without this, Get()
	// returns nil and the infra/ci packages panic when rendering.
	_ "github.com/torchstellar-team/one-cli/packages/cli/internal/toolchain"
)

// rootCmd is the singleton root cobra command. Subcommands attach via init().
// Short is filled by i18n.MarkShort in init() so it tracks the active locale.
var rootCmd = &cobra.Command{
	Use:           "one",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// RootCmd returns the configured root cobra command for read-only
// introspection by tools (e.g. verify-cli-references walks the command
// tree to validate `one <subcmd>` references in documentation). Importing
// this package triggers init() so all side-effect command registrations
// are settled before this returns.
func RootCmd() *cobra.Command { return rootCmd }

// RootHelp returns the curated rootHelp text — what `one --help` emits.
// Exposed for verify-help, which parses the COMMANDS block to diff
// against registered top-level commands. The text is locale-dependent;
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
func Execute(version string, args []string) error {
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
	// paths. See internal/updatecheck.
	updatecheck.MaybeRefreshAsync(version)
	defer updatecheck.Notify(version)

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
// render the curated rootHelp text. Returns true for:
//   - bare invocation: `one`
//   - help on the root: `one --help`, `one -h`, `one help`
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
// command. We inspect rootCmd.Commands() so adding a new subcommand via
// init() automatically participates without touching this list.
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

func init() {
	// Tag the root with its i18n key so RefreshTree picks it up on
	// locale change. The initial value resolves under DefaultLocale
	// (en-US) so tests and `--help` before Init() see deterministic
	// English text.
	i18n.MarkShort(rootCmd, "root.short")

	// Mount cliexts contributors onto rootCmd. Imported domain
	// packages have already run their init() (Go guarantees imported-
	// package inits run before importer-package inits), so every
	// cliexts.Register call is settled by the time we hit this line.
	// Conflicts (duplicate top-level names) are programmer errors:
	// panic so the binary refuses to start rather than silently dropping
	// a command.
	if err := cliexts.Mount(rootCmd); err != nil {
		panic(err)
	}

	rootCmd.SetVersionTemplate("{{.Version}}\n")
	// Help interception is done in Execute() via shouldRenderRootHelp for
	// the root command (`one --help` / `one help`). We deliberately do NOT
	// set SetHelpTemplate / SetUsageTemplate / SetHelpFunc here — those
	// propagate to every subcommand and clobber the per-subcommand help
	// text that cobra generates from each command's Use / Short / Long
	// fields. Subcommand help (`one env --help`, `one create --help`)
	// uses cobra's defaults, which is exactly what we want.

	// -o / --output is the global persistent output-mode flag, kubectl-style.
	// Accepts "json" | "yaml" | "text"; unknown values fall through to auto.
	// Output mode detection in detectOutputMode reads os.Args directly so the
	// value is honoured before cobra parses; this PFlag registration is
	// the canonical Cobra declaration so the flag appears in --help and
	// cobra accepts the token without "unknown flag" errors.
	rootCmd.PersistentFlags().StringP("output", "o", "",
		"输出格式: json | yaml | text（默认 auto：TTY 时人类格式，pipe 时 JSON）")
}

// (Previously: the rootHelp constant. The curated help text now lives
// in internal/i18n/locales/{en-US,zh-CN}.json under the "root.help"
// key. Edit the JSON, not Go. The text is rendered via i18n.T at
// runtime in Execute().)
