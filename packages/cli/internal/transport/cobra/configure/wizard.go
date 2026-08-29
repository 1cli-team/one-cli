package configurecmd

import (
	"fmt"

	"sort"
	"strings"

	"github.com/spf13/cobra"

	configureapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/configure"
	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/prompt"
)

// runConfigureWizard handles bare `one configure` and bare `one
// configure add`. In an interactive TTY it prompts the user for a
// (domain, backend) pair and then runs that pair's add flow. In a
// non-TTY shell (CI / -y mode) it errors with guidance to use the
// explicit path so scripts never accidentally hang on a prompt.
func runConfigureWizard(backendCatalog *catalog.Catalog, profiles *configureapp.ProfileService, cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return cliErrors.New(cliErrors.UNKNOWN_COMMAND,
			fmt.Sprintf("`one configure %s` 不是已知子命令；可用 (domain, backend) 见 `one configure --help`", strings.Join(args, " ")))
	}
	if !output.CanPrompt() {
		return cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			"非交互式调用 `one configure [add]` 不支持；请显式指定 `one configure add <domain>/<backend> --profile <name>`，"+
				"如 `one configure add env/infisical --profile work --client-id ... --client-secret ...`。")
	}
	pair, err := pickPair(profiles)
	if err != nil {
		return err
	}
	domain, backend := splitPair(pair)
	return runSelectedAddBackend(backendCatalog, profiles, cmd, domain, backend)
}

func runSelectedAddBackend(backendCatalog *catalog.Catalog, profiles *configureapp.ProfileService, cmd *cobra.Command, domain profile.Domain, backend string) error {
	spec, err := profiles.Lookup(domain, backend)
	if err != nil {
		return err
	}
	addCmd := newAddBackendCmd(profiles, spec)
	addCmd.SetIn(cmd.InOrStdin())
	addCmd.SetOut(cmd.OutOrStdout())
	addCmd.SetErr(cmd.ErrOrStderr())
	// Cobra treats nil args as "read os.Args[1:]" on Execute. The
	// wizard is dispatching a fresh leaf command, so pass a real empty
	// slice to avoid replaying the original "configure add" args into
	// the selected backend.
	addCmd.SetArgs([]string{})
	return addCmd.Execute()
}

// ConfigureService runs the existing interactive connection builder for a
// concrete service. Deployment uses it on first deploy instead of duplicating
// credential prompts or storage rules.
func ConfigureService(backendCatalog *catalog.Catalog, profiles *configureapp.ProfileService, cmd *cobra.Command, domain profile.Domain, backend string) error {
	return runSelectedAddBackend(backendCatalog, profiles, cmd, domain, backend)
}

// pickPair prompts the user to choose one (domain, backend) pair from
// the five supported options. Returns the SectionKey ("env/infisical"
// etc.) so callers can split it back into the typed pieces.
func pickPair(profiles *configureapp.ProfileService) (string, error) {
	type pairChoice struct {
		key   string
		label string
	}
	choices := make([]pairChoice, 0)
	for _, pair := range supportedPairs(profiles) {
		choices = append(choices, pairChoice{
			key:   profile.SectionKey(pair.Domain, pair.Backend),
			label: serviceLabel(pair.Domain, pair.Backend),
		})
	}
	options := make([]prompt.Option[string], 0, len(choices))
	for _, c := range choices {
		options = append(options, prompt.Option[string]{Label: c.label, Value: c.key})
	}
	return prompt.Select(i18n.T("configure.prompt_service"), options)
}

// splitPair turns "env/infisical" back into (DomainEnv, "infisical").
// Caller has already validated the pair string came from pickPair so a
// missing slash is treated as a programmer bug, not user input.
func splitPair(pair string) (profile.Domain, string) {
	parts := strings.SplitN(pair, "/", 2)
	return profile.Domain(parts[0]), parts[1]
}

func serviceLabel(domain profile.Domain, backend string) string {
	key := "configure.service." + string(domain) + "." + backend
	label := i18n.T(key)
	if label == key {
		return backend
	}
	return label
}

// parsePair turns a positional CLI arg ("env/infisical") into the
// typed pieces, or returns PROFILE_BACKEND_INVALID with the list of
// valid pairs when the input doesn't match.
func parsePair(profiles *configureapp.ProfileService, arg string) (profile.Domain, string, error) {
	arg = strings.TrimSpace(arg)
	for _, p := range supportedPairs(profiles) {
		if fmt.Sprintf("%s/%s", p.Domain, p.Backend) == arg {
			return p.Domain, p.Backend, nil
		}
	}
	pairs := supportedPairs(profiles)
	valid := make([]string, 0, len(pairs))
	for _, p := range pairs {
		valid = append(valid, fmt.Sprintf("%s/%s", p.Domain, p.Backend))
	}
	return "", "", cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
		fmt.Sprintf("未知 (domain, backend) pair %q；可选：%s。", arg, strings.Join(valid, " / ")))
}

type connectionSelection struct {
	Domain  profile.Domain
	Backend string
	Name    string
}

func resolveExistingConnection(profiles *configureapp.ProfileService, args []string, nameFlag string) (connectionSelection, error) {
	cfg, err := profiles.Load()
	if err != nil {
		return connectionSelection{}, err
	}
	name := strings.TrimSpace(nameFlag)
	if len(args) == 1 {
		domain, backend, err := parsePair(profiles, args[0])
		if err != nil {
			return connectionSelection{}, err
		}
		if name != "" {
			return connectionSelection{Domain: domain, Backend: backend, Name: name}, nil
		}
		if !output.CanPrompt() {
			return connectionSelection{}, cliErrors.New(cliErrors.PROFILE_NOT_FOUND,
				i18n.T("configure.connection_name_required"))
		}
		names, _ := listSection(profiles, cfg, domain, backend)
		sort.Strings(names)
		if len(names) == 0 {
			return connectionSelection{}, cliErrors.New(cliErrors.PROFILE_NONE_CONFIGURED,
				i18n.Tf("configure.no_service_connections", args[0]))
		}
		options := make([]prompt.Option[string], 0, len(names))
		for _, candidate := range names {
			options = append(options, prompt.Option[string]{Label: candidate, Value: candidate})
		}
		selected, err := prompt.Select(i18n.T("configure.prompt_connection"), options)
		return connectionSelection{Domain: domain, Backend: backend, Name: selected}, err
	}
	if !output.CanPrompt() {
		return connectionSelection{}, cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			i18n.T("configure.service_required"))
	}
	connections := collectConnections(profiles, cfg)
	if len(connections) == 0 {
		return connectionSelection{}, cliErrors.New(cliErrors.PROFILE_NONE_CONFIGURED,
			i18n.T("configure.no_connections"))
	}
	options := make([]prompt.Option[string], 0, len(connections))
	for _, connection := range connections {
		value := connection.ServiceID + "\x00" + connection.Name
		domain, backend := splitPair(connection.ServiceID)
		options = append(options, prompt.Option[string]{
			Label: connection.Name + "  ·  " + serviceLabel(domain, backend),
			Value: value,
		})
	}
	selected, err := prompt.Select(i18n.T("configure.prompt_connection"), options)
	if err != nil {
		return connectionSelection{}, err
	}
	parts := strings.SplitN(selected, "\x00", 2)
	domain, backend, err := parsePair(profiles, parts[0])
	if err != nil {
		return connectionSelection{}, err
	}
	return connectionSelection{Domain: domain, Backend: backend, Name: parts[1]}, nil
}

// pairCompletion gives shell completion the list of valid pairs as
// the first positional. After that the verb decides what comes next
// (a profile name or nothing); we don't try to autocomplete profile
// names because that would require loading config from disk.
func pairCompletion(profiles *configureapp.ProfileService) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		pairs := supportedPairs(profiles)
		out := make([]string, 0, len(pairs))
		for _, p := range pairs {
			out = append(out, fmt.Sprintf("%s/%s", p.Domain, p.Backend))
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}
