package deploycmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	deploymentapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/deployment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/prompt"
	configurecmd "github.com/torchstellar-team/one-cli/packages/cli/internal/transport/cobra/configure"
)

type deployDeferredResult struct {
	Schema   string `json:"schema"`
	Project  string `json:"project"`
	Provider string `json:"provider"`
	Recovery string `json:"recovery_command"`
}

func (r *deployDeferredResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	fmt.Fprintln(w, i18n.T("deploy.deferred"))
	fmt.Fprintf(w, i18n.T("deploy.deferred_project")+"\n", r.Project)
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("common.next_steps"))
	fmt.Fprintln(w, "  "+r.Recovery)
}

func selectProjectForDeployment(projects []string) (string, error) {
	options := make([]prompt.Option[string], 0, len(projects))
	for _, project := range projects {
		options = append(options, prompt.Option[string]{Label: project, Value: project})
	}
	return prompt.Select(i18n.T("deploy.prompt_project"), options)
}

func providerCategory(provider string) string {
	switch provider {
	case workspace.DeployBackendKustomize:
		return "kubernetes"
	case workspace.DeployBackendVercel, workspace.DeployBackendCloudflare, workspace.DeployBackendEdgeOne:
		return "platform"
	default:
		return "static"
	}
}

func providerDisplayLabel(provider string) string {
	key := "deploy.provider." + provider
	label := i18n.T(key)
	if label == key {
		return provider
	}
	return label
}

func selectCompatibleProvider(compatible []string) (string, error) {
	categoryOrder := []string{"static", "platform", "kubernetes"}
	grouped := map[string][]string{}
	for _, provider := range compatible {
		category := providerCategory(provider)
		grouped[category] = append(grouped[category], provider)
	}
	categoryOptions := []prompt.Option[string]{}
	for _, category := range categoryOrder {
		if len(grouped[category]) > 0 {
			categoryOptions = append(categoryOptions, prompt.Option[string]{
				Label: i18n.T("deploy.category." + category), Value: category,
			})
		}
	}
	category, err := prompt.Select(i18n.T("deploy.prompt_category"), categoryOptions)
	if err != nil {
		return "", err
	}
	providerOptions := make([]prompt.Option[string], 0, len(grouped[category]))
	for _, provider := range grouped[category] {
		providerOptions = append(providerOptions, prompt.Option[string]{Label: providerDisplayLabel(provider), Value: provider})
	}
	return prompt.Select(i18n.T("deploy.prompt_provider"), providerOptions)
}

func configureFirstDeployment(
	deps Dependencies,
	service *deploymentapp.Service,
	cmd *cobra.Command,
	root string,
	m *workspace.Manifest,
	setup *deploymentapp.TargetSetup,
	profileFlag string,
) (deploymentapp.Target, error) {
	if setup == nil || setup.Project == nil {
		return deploymentapp.Target{}, cliErrors.New(
			cliErrors.ONE_CLI_ERROR,
			"deploy target setup 不能为空",
		)
	}
	providerID := setup.Backend
	if providerID == "" {
		if !output.CanPrompt() {
			return deploymentapp.Target{}, cliErrors.New(cliErrors.BACKEND_NOT_ENABLED,
				i18n.Tf("deploy.provider_required", setup.Project.Name)).
				WithRemediation(output.Remediation{Action: "choose-deployment-target", Command: "one deploy " + setup.Project.Name + " --provider <target> --profile <connection>"})
		}
		var err error
		providerID, err = selectCompatibleProvider(setup.CompatibleBackends)
		if err != nil {
			return deploymentapp.Target{}, err
		}
	}
	target, err := setup.ResolveTarget(root, providerID)
	if err != nil {
		return deploymentapp.Target{}, err
	}
	resolved, err := service.ResolveProfile(m, profileFlag, target)
	if err != nil {
		return deploymentapp.Target{}, err
	}
	if resolved == nil {
		if !output.CanPrompt() {
			return deploymentapp.Target{}, cliErrors.New(cliErrors.PROFILE_NONE_CONFIGURED,
				i18n.Tf("deploy.connection_required", providerID)).
				WithRemediation(output.Remediation{
					Action: "configure-connection", Command: "one configure add deploy/" + providerID + " --profile <connection>",
				})
		}
		choice, err := prompt.Select(i18n.Tf("deploy.prompt_missing_connection", providerID), []prompt.Option[string]{
			{Label: i18n.T("deploy.configure_now"), Value: "now"},
			{Label: i18n.T("deploy.configure_later"), Value: "later"},
		})
		if err != nil {
			return deploymentapp.Target{}, err
		}
		if choice == "later" {
			recovery := "one deploy " + setup.Project.Name
			output.Emit(&deployDeferredResult{
				Schema: "one-cli/deploy-deferred/v1", Project: setup.Project.Name, Provider: providerID, Recovery: recovery,
			})
			return deploymentapp.Target{}, cliErrors.New(cliErrors.PROMPT_CANCELLED, i18n.T("deploy.deferred")).WithExit0()
		}
		if err := configurecmd.ConfigureService(deps.Catalog, deps.Profiles, cmd, profile.DomainDeploy, providerID); err != nil {
			return deploymentapp.Target{}, err
		}
		resolved, err = service.ResolveProfile(m, profileFlag, target)
		if err != nil {
			return deploymentapp.Target{}, err
		}
		if resolved == nil {
			return deploymentapp.Target{}, cliErrors.New(cliErrors.PROFILE_NONE_CONFIGURED,
				i18n.Tf("deploy.connection_required", providerID))
		}
	}
	// Only now, after the user has chosen a usable local connection, mutate
	// workspace deployment state and generate its artifacts.
	if err := prompt.Spin(i18n.T("deploy.generating_config"), func() error {
		return deps.Creation.ConfigureProjectDeployment(
			cmd.Context(), root, setup.Template, setup.Project.Name, providerID,
		)
	}); err != nil {
		return deploymentapp.Target{}, err
	}
	return target, nil
}
