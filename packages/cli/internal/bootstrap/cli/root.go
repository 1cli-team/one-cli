// Package cli assembles the command tree and runs the CLI lifecycle.
// Vendor integrations are supplied explicitly by the built-in composition.
package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/lifecycle"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins"

	addcmd "github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/add"
	createcmd "github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/create"
	execcmd "github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/exec"
	groupcmd "github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/group"
	hookscmd "github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/hooks"
	initcmd "github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/init"
	localecmd "github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/locale"
	mirrorcmd "github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/mirror"
	misecmd "github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/mise"
	runcmd "github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/run"
	skillscmd "github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/skills"
	templatescmd "github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/templates"
	upgradecmd "github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/upgrade"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/bootstrap"
	skillsmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/skills"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/helpui"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
)

// Plugin contributes both shared capabilities and command lifecycle hooks.
type Plugin interface {
	plugins.Plugin
	lifecycle.Plugin
}

type Application struct {
	root      *cobra.Command
	lifecycle *lifecycle.Lifecycle
	rootHelp  func() string
}

// NewApplication constructs a command tree without opening credentials or
// making network requests. An empty plugin list is a valid core-only CLI.
// Application owns a single invocation; construct a fresh instance for each run.
func NewApplication(installed ...Plugin) (*Application, error) {
	shared := make([]plugins.Plugin, len(installed))
	hooks := make([]lifecycle.Plugin, len(installed))
	for i, plugin := range installed {
		shared[i], hooks[i] = plugin, plugin
	}
	deps, err := bootstrap.ComposePlugins(shared...)
	if err != nil {
		return nil, err
	}
	phases, err := lifecycle.New(hooks...)
	if err != nil {
		return nil, err
	}
	skillsRunner := skillsmodule.Runner{Runtime: deps.Runtime}
	root := &cobra.Command{
		Use:           "one",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	groups := [][]*cobra.Command{
		addcmd.Commands(deps.Creation),
		{localecmd.Command(), mirrorcmd.Command(), initcmd.Command(skillsmodule.Installer{Run: skillsRunner.Run, Hash: skillsRunner.Hash, Inspect: skillsRunner.Inspect, InstallGroup: skillsRunner.InstallGroup}), upgradecmd.Command()},
		createcmd.Commands(createcmd.Dependencies{Creation: deps.Creation}),
		misecmd.RuntimeCommands(deps.Runtime),
		skillscmd.Commands(skillsRunner.Run),
		hookscmd.Commands(deps.Runtime),
		execcmd.Commands(deps.Loaders, deps.Runtime, deps.Global),
		runcmd.Commands(deps.Loaders, deps.Runtime),
		templatescmd.Commands(),
		{groupcmd.Command(deps.Groups)},
	}
	for _, commands := range groups {
		root.AddCommand(commands...)
	}
	i18n.MarkShort(root, "root.short")
	root.SetVersionTemplate("{{.Version}}\n")
	root.SetHelpFunc(helpui.Render)
	root.SetFlagErrorFunc(i18n.FlagError)
	root.PersistentFlags().StringP("output", "o", "", i18n.T("common.flag.output"))
	i18n.MarkFlagUsage(root, "output", "common.flag.output")
	if err := phases.Register(lifecycle.Host{Root: root, Dependencies: deps}); err != nil {
		return nil, err
	}
	return &Application{root: root, lifecycle: phases}, nil
}

// RootHelp returns the curated rootHelp text — what `one --help` emits.
// Exposed for verify-help, which ensures the concise daily command list
// stays intentional. The text is locale-dependent;
// callers that need the deterministic English form should call
// i18n.Init("en-US") first.
func RootHelp() string { return i18n.T("root.help") }

// Execution leaves and previews must not start another background network check.
func shouldCheckUpdates(args []string) bool {
	if first, _ := firstPositional(args); strings.HasPrefix(first, "__") || first == "mise" || first == "hk" || first == "skills" || first == "upgrade" || first == "mirror" {
		return false
	}
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--dry-run" || arg == "--dry-run=true" {
			return false
		}
	}
	return true
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
	return shouldRenderHelp(args, isKnownSubcommand)
}

func shouldRenderHelp(args []string, known func(string) bool) bool {
	if len(args) == 0 {
		return true
	}
	first := args[0]
	if _, ok := helpFlags[first]; !ok {
		return false
	}
	// `one help <known-subcommand>` should fall through to cobra so the
	// subcommand's own help is rendered.
	if len(args) > 1 && known(args[1]) {
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
func isKnownSubcommand(name string) bool { return knownSubcommand(RootCmd(), name) }

func knownSubcommand(root *cobra.Command, name string) bool {
	for _, c := range root.Commands() {
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
	first, _ := firstPositional(args)
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		if a == "skills" && first == "skills" {
			break // All following flags belong to the upstream Skills CLI.
		}
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
