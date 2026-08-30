package deploycmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	deploymentapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/deployment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/helpui"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

type commandObserver struct {
	command *cobra.Command
	dryRun  bool
}

func (o commandObserver) TargetStarted(target deploymentapp.Target) {
	if output.IsTTY() {
		fmt.Fprintf(o.command.OutOrStderr(),
			i18n.T("deploy.starting")+"\n", target.Project.Name, providerDisplayLabel(target.Backend))
	}
}

func (o commandObserver) TargetCompleted(result deploymentapp.TargetResult) error {
	if o.dryRun {
		if result.Injection != nil {
			environment := result.Injection.EnvName
			if environment == "" {
				environment = "(default)"
			}
			_, _ = fmt.Fprintf(o.command.OutOrStdout(),
				"# injected env (source: %s, env=%s): %s\n",
				result.Injection.Source, environment, strings.Join(result.Injection.Keys, ", "))
		}
		lines := result.Apply.CommandLines
		if len(lines) == 0 {
			lines = []string{strings.Join(result.Apply.Argv, " ")}
		}
		lines = append(result.BuildCommandLines, lines...)
		for _, line := range lines {
			_, _ = o.command.OutOrStdout().Write([]byte(line + "\n"))
		}
		return nil
	}
	output.Emit(result.Apply)
	return nil
}

func newDeployCmd(deps Dependencies) *cobra.Command {
	var (
		profileFlag, providerFlag, buildVersion, project string
		envProvider                                      string
		envFlag                                          string
		dryRun                                           bool
	)
	cmd := &cobra.Command{
		Use:     "deploy [project]",
		Long:    i18n.T("deploy.tip"),
		Example: "  one deploy\n  one deploy web\n  one deploy web --provider vercel --profile work",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			positional := ""
			if len(args) > 0 {
				positional = args[0]
			}
			if positional != "" && project != "" && positional != project {
				return cliErrors.New(cliErrors.ONE_CLI_ERROR,
					i18n.T("deploy.selector_conflict"))
			}
			if positional != "" {
				project = positional
			}
			activeWorkspace, err := execution.ResolveWorkspace(cmd.Context())
			if err != nil {
				return err
			}
			root := activeWorkspace.Root()
			m := activeWorkspace.Manifest()
			registry, err := template.Fetch(cmd.Context(), "")
			if err != nil {
				return err
			}

			deployments := deps.NewService(buildVersion)
			plan, err := deployments.PlanTargets(deploymentapp.PlanRequest{
				ProjectRoot: root,
				Manifest:    m,
				Templates:   registry,
				Project:     project,
				Backend:     providerFlag,
			})
			if err != nil {
				return err
			}
			if len(plan.ProjectChoices) > 0 {
				if !output.CanPrompt() {
					return cliErrors.New(cliErrors.BACKEND_NOT_ENABLED,
						i18n.T("deploy.project_required")).
						WithRemediation(output.Remediation{Action: "choose-project", Command: "one deploy <project> --provider <target> --profile <connection>"})
				}
				selected, selectErr := selectProjectForDeployment(plan.ProjectChoices)
				if selectErr != nil {
					return selectErr
				}
				plan, err = deployments.PlanTargets(deploymentapp.PlanRequest{
					ProjectRoot: root,
					Manifest:    m,
					Templates:   registry,
					Project:     selected,
					Backend:     providerFlag,
				})
				if err != nil {
					return err
				}
			}

			targets := plan.Targets
			if plan.Setup != nil {
				target, configureErr := configureFirstDeployment(
					deps, deployments, cmd, root, m, plan.Setup, profileFlag,
				)
				if configureErr != nil {
					return configureErr
				}
				targets = []deploymentapp.Target{target}
				activeWorkspace, err = activeWorkspace.Reload()
				if err != nil {
					return err
				}
				m = activeWorkspace.Manifest()
			}
			_, err = deployments.Execute(cmd.Context(), deploymentapp.ExecuteRequest{
				ProjectRoot: root,
				Manifest:    m,
				Targets:     targets,
				Profile:     profileFlag,
				EnvProvider: envProvider,
				Environment: envFlag,
				DryRun:      dryRun,
				Stdout:      cmd.OutOrStdout(),
				Stderr:      cmd.ErrOrStderr(),
				ProfileFallback: func(target deploymentapp.Target, resolved *profile.Resolved) (*profile.Resolved, error) {
					return ensureInteractiveCloudflareProfile(profileFlag, target, resolved)
				},
			}, commandObserver{command: cmd, dryRun: dryRun})
			return err
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, i18n.T("deploy.flag.dry_run"))
	cmd.Flags().StringVar(&profileFlag, "profile", "", i18n.T("deploy.flag.profile"))
	cmd.Flags().StringVar(&providerFlag, "provider", "", i18n.T("deploy.flag.provider"))
	cmd.Flags().StringVar(&buildVersion, "build-version", "", i18n.T("deploy.flag.build_version"))
	cmd.Flags().StringVarP(&project, "project", "p", "", i18n.T("deploy.flag.project"))
	cmd.Flags().StringVar(&envProvider, "env-provider", "", i18n.T("deploy.flag.env_provider"))
	cmd.Flags().StringVar(&envFlag, "env", "", i18n.T("deploy.flag.env"))
	for name, key := range map[string]string{
		"dry-run": "deploy.flag.dry_run", "profile": "deploy.flag.profile",
		"provider": "deploy.flag.provider", "build-version": "deploy.flag.build_version",
		"project": "deploy.flag.project", "env-provider": "deploy.flag.env_provider",
		"env": "deploy.flag.env",
	} {
		i18n.MarkFlagUsage(cmd, name, key)
	}
	helpui.MarkAdvanced(cmd, "profile", "provider", "project", "build-version", "env-provider")
	i18n.MarkShort(cmd, "deploy.short")
	i18n.MarkLong(cmd, "deploy.tip")
	return cmd
}
