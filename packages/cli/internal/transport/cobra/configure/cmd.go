// Package configurecmd contributes the top-level `one configure`
// command to the explicit root composition. configure is the only entry point for the
// profile lifecycle: add (upsert) / list / current / show / use /
// remove, plus a no-arg interactive wizard that selects a (domain,
// backend) pair and dispatches to the matching add command.
//
// Naming note: the *command* is `configure` to align with industry
// standard CLIs (aws / gcloud / azure). The *data object* is still a
// "profile" — that survives in the --profile flag, local workspace
// bindings, and the internal/core/profile Go package.
//
// Tree shape (verb-first, v0.7+):
//
//	configure
//	├── add [pair] [--profile <name>]  # bare → interactive wizard
//	│   ├── env/infisical [--profile <name>]    # backend-specific flags
//	│   ├── deploy/aliyun-oss [--profile <name>]
//	│   ├── deploy/tencent-cos [--profile <name>]
//	│   ├── deploy/aws-s3 [--profile <name>]
//	│   ├── deploy/minio  [--profile <name>]
//	│   ├── deploy/rustfs [--profile <name>]
//	│   ├── deploy/r2     [--profile <name>]
//	│   ├── deploy/kustomize [--profile <name>]
//	│   └── container/docker [--profile <name>]
//	├── list    [pair]                 # no pair → aggregate all sections
//	├── current [pair]                 # no pair → aggregate all sections
//	├── show    <pair> --profile <name>
//	├── use     <pair> --profile <name> [--workspace] [--project <name|path>]
//	└── remove  <pair> --profile <name>
//
// Storage is the two-file split: ~/.config/one/config.json (non-
// sensitive) + ~/.config/one/credentials.json (secrets), both 0600.
// The (domain, backend) section split is unchanged. Profile resolution
// chain is --profile flag → local workspace binding → section.default.
package configurecmd

import (
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	configureapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/configure"
	workspaceapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/workspace"
	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	servecmd "github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/serve"
)

func Commands(
	backendCatalog *catalog.Catalog,
	profiles *configureapp.ProfileService,
	workspaces *workspaceapp.Service,
	registries ...*workspaceapp.RegistryService,
) []*cobra.Command {
	var registry *workspaceapp.RegistryService
	if len(registries) > 0 {
		registry = registries[0]
	}
	return buildContributions(backendCatalog, profiles, workspaces, registry)
}

type supportedPair struct {
	Domain  profile.Domain
	Backend string
}

// supportedPairs comes from the same catalog used by the Dashboard and HTTP
// API. Its order intentionally preserves the existing configure help output.
func supportedPairs(profiles *configureapp.ProfileService) []supportedPair {
	backends := profiles.ProfileBackends()
	pairs := make([]supportedPair, 0, len(backends))
	for _, backend := range backends {
		pairs = append(pairs, supportedPair{
			Domain: profile.Domain(backend.ID.Domain), Backend: backend.ID.Name,
		})
	}
	return pairs
}

func buildContributions(
	backendCatalog *catalog.Catalog,
	profiles *configureapp.ProfileService,
	workspaces *workspaceapp.Service,
	registries ...*workspaceapp.RegistryService,
) []*cobra.Command {
	var registry *workspaceapp.RegistryService
	if len(registries) > 0 {
		registry = registries[0]
	}
	parent := &cobra.Command{
		Use:     "configure",
		Long:    i18n.T("configure.tip"),
		Example: "  one configure\n  one configure open\n  one configure list",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigure(backendCatalog, profiles, cmd, args)
		},
	}
	children := []*cobra.Command{
		buildAddCmd(backendCatalog, profiles), buildListCmd(profiles), buildCurrentCmd(profiles), buildShowCmd(profiles),
		buildUseCmd(profiles), buildRemoveCmd(profiles), servecmd.NewOpenCmd(servecmd.Dependencies{
			Catalog: backendCatalog, Profiles: profiles, Workspaces: workspaces,
			Registry: registry,
		}), buildLocaleCmd(),
	}
	parent.AddCommand(children...)
	i18n.MarkShort(parent, "configure.short")
	i18n.MarkLong(parent, "configure.tip")
	return []*cobra.Command{parent}
}

type configureSummary struct {
	Schema      string                `json:"schema"`
	Connections []configureConnection `json:"connections"`
	ConfigPath  string                `json:"config_path"`
}

type configureConnection struct {
	ServiceID string `json:"service_id"`
	Name      string `json:"name"`
	Default   bool   `json:"default"`
}

func runConfigure(backendCatalog *catalog.Catalog, profiles *configureapp.ProfileService, cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return runConfigureWizard(backendCatalog, profiles, cmd, args)
	}
	cfg, err := profiles.Load()
	if err != nil {
		return err
	}
	connections := collectConnections(profiles, cfg)
	if len(connections) == 0 && output.CanPrompt() {
		return runConfigureWizard(backendCatalog, profiles, cmd, nil)
	}
	path, _, _ := profiles.Paths()
	output.Emit(&configureSummary{
		Schema: "one-cli/configure-summary/v1", Connections: connections, ConfigPath: path,
	})
	return nil
}

func collectConnections(profiles *configureapp.ProfileService, cfg *profile.Config) []configureConnection {
	result := make([]configureConnection, 0)
	for _, pair := range supportedPairs(profiles) {
		names, defaultName := listSection(profiles, cfg, pair.Domain, pair.Backend)
		sort.Strings(names)
		for _, name := range names {
			result = append(result, configureConnection{
				ServiceID: profile.SectionKey(pair.Domain, pair.Backend),
				Name:      name, Default: name == defaultName,
			})
		}
	}
	return result
}

func (s *configureSummary) RenderTTY(w io.Writer) {
	if s == nil {
		return
	}
	fmt.Fprintln(w, i18n.T("configure.summary_title"))
	if len(s.Connections) == 0 {
		fmt.Fprintln(w, i18n.T("configure.no_connections"))
	} else {
		for _, connection := range s.Connections {
			marker := ""
			if connection.Default {
				marker = i18n.T("configure.default_marker")
			}
			domain, backend := splitPair(connection.ServiceID)
			fmt.Fprintf(w, "  %s  %s%s\n", connection.Name, serviceLabel(domain, backend), marker)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("configure.local_only"))
	fmt.Fprintf(w, i18n.T("configure.settings_path")+"\n", s.ConfigPath)
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("configure.next_open"))
}
