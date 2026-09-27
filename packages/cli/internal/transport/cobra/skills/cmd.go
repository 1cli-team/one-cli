// Package skillscmd exposes installation of the bundled one-cli skill.
package skillscmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/skills"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/prompt"
)

func Commands() []*cobra.Command {
	parent := &cobra.Command{Use: "skills", Args: i18n.NoArgs}
	i18n.MarkShort(parent, "skills.short")
	var agents []string
	var yes bool
	install := &cobra.Command{
		Use: "install", Args: i18n.NoArgs,
		Example: "one skills install\none skills install --agent claude-code --agent codex\none skills install --yes -o json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			targets, err := selectTargets(agents, yes)
			if err != nil {
				return err
			}
			installed, err := skills.Install(cmd.Context(), targets)
			if err != nil {
				return cliErrors.New(cliErrors.SKILLS_INSTALL_FAILED, err.Error()).WithContext(map[string]any{"installed_to": installed})
			}
			output.Emit(&result{
				Schema: "one-cli/skills-install/v1", Status: "completed",
				Targets: targets, InstalledTo: installed, SkillCount: 1,
			})
			return nil
		},
	}
	i18n.MarkShort(install, "skills.install.short")
	i18n.MarkLong(install, "skills.install.long")
	install.Flags().StringSliceVarP(&agents, "agent", "a", nil, "")
	install.Flags().BoolVarP(&yes, "yes", "y", false, "")
	i18n.MarkFlagUsage(install, "agent", "skills.flag.agent")
	i18n.MarkFlagUsage(install, "yes", "skills.flag.yes")
	// Render the IDs from the registry, so help and validation cannot drift.
	install.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		cmd.Long = i18n.T("skills.install.long") + "\n\n" + i18n.T("skills.supported") + "\n" + strings.Join(skills.AgentIDs(), ", ")
		parent.HelpFunc()(cmd, args)
	})
	parent.AddCommand(install)
	return []*cobra.Command{parent}
}

func selectTargets(ids []string, yes bool) ([]skills.Agent, error) {
	targets, err := skills.ResolveTargets(ids)
	if err != nil {
		return nil, cliErrors.New(cliErrors.SKILLS_INSTALL_FAILED, err.Error())
	}
	if len(ids) > 0 || yes || !output.CanPrompt() {
		return targets, nil
	}
	options := make([]prompt.Option[string], 0, len(targets))
	defaults := make([]string, 0, len(targets))
	for _, target := range targets {
		options = append(options, prompt.Option[string]{Label: target.DisplayName, Description: target.GlobalPath, Value: target.ID})
		defaults = append(defaults, target.ID)
	}
	picked, err := prompt.MultiSelect(i18n.T("skills.select"), options, defaults)
	if err != nil {
		return nil, err
	}
	if len(picked) == 0 {
		return nil, cliErrors.New(cliErrors.SKILLS_INSTALL_FAILED, i18n.T("skills.no_selection"))
	}
	return skills.ResolveTargets(picked)
}

type result struct {
	Schema      string         `json:"schema"`
	Status      string         `json:"status"`
	Targets     []skills.Agent `json:"targets"`
	InstalledTo []string       `json:"installed_to"`
	SkillCount  int            `json:"skill_count"`
}

func (r *result) RenderTTY(w io.Writer) {
	fmt.Fprintln(w, i18n.T("skills.installed"))
	for _, target := range r.Targets {
		fmt.Fprintf(w, "  %s: %s\n", target.DisplayName, target.GlobalPath)
	}
}
