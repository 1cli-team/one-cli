package skillscmd

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	skillsmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/skills"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
	platformprocess "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/process"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/prompt"
)

type recommendationPrompts struct {
	Skills   selectionPrompt
	Settings func(context.Context, *skillsmodule.Inspection, skillsmodule.InstallOptions) (skillsmodule.InstallOptions, error)
	Review   func(context.Context, []skillsmodule.InstallGroup, skillsmodule.InstallOptions, *skillsmodule.Inspection) (string, error)
	Retry    func(context.Context) (bool, error)
}

func defaultRecommendationPrompts() recommendationPrompts {
	return recommendationPrompts{
		Skills: prompt.MultiSelectContext[string], Settings: chooseInstallSettings, Review: reviewInstallation,
		Retry: func(ctx context.Context) (bool, error) {
			action, err := prompt.SelectContext(ctx, i18n.T("skills.batch.retry_title"), "", []prompt.Option[string]{
				{Label: i18n.T("skills.batch.retry"), Value: "retry"},
				{Label: i18n.T("skills.batch.finish"), Value: "finish"},
			}, "retry")
			return action == "retry", err
		},
	}
}

func runRecommended(cmd *cobra.Command, installer skillsmodule.Installer, root string, options skillsmodule.InstallOptions, yes, dryRun bool, ui recommendationPrompts) error {
	ctx := cmd.Context()
	scan, err := skillsmodule.Recommend(ctx, root)
	if err != nil {
		return err
	}
	if dryRun {
		if options.Scope == "global" {
			scan, err = scan.ForScope(options.Scope, nil)
			if err != nil {
				return err
			}
			scan.Warnings = append(scan.Warnings, i18n.T("skills.batch.global_preview"))
		}
		output.Emit(scan)
		return nil
	}
	var inspection *skillsmodule.Inspection
	inspect := func() error {
		if inspection != nil {
			return nil
		}
		if installer.Inspect == nil {
			return i18n.Errorf("skills.runner_required")
		}
		fmt.Fprintln(cmd.ErrOrStderr(), i18n.T("skills.batch.detecting"))
		inspection, err = installer.Inspect(ctx, root)
		return err
	}
	if options.Scope == "global" {
		if err := inspect(); err != nil {
			return err
		}
	}
	var selected []string
	var groups []skillsmodule.InstallGroup
	back := false
	for {
		scan, err = skillsmodule.Recommend(ctx, root)
		if err != nil {
			return err
		}
		scan, err = scan.ForScope(options.Scope, inspection)
		if err != nil {
			return err
		}
		choices, defaults, description := recommendationOptions(scan)
		if back {
			defaults = availableSelection(choices, selected)
		}
		selected = defaults
		if !yes && len(choices) > 0 {
			selected, err = ui.Skills(ctx, i18n.T("skills.recommend.title"), description, choices, defaults)
			if err != nil {
				return err
			}
		}
		if len(selected) == 0 {
			output.Emit(&skillsmodule.BatchResult{Schema: "one-cli/install-skills/v1", Root: root, Options: options, Targets: []skillsmodule.TargetResult{}})
			return nil
		}
		if err := inspect(); err != nil {
			return err
		}
		if len(options.Agents) == 0 {
			options.Agents = defaultAgents(inspection, options.Scope)
		}
		if err := options.Validate(inspection); err != nil {
			return err
		}
		previousScope := options.Scope
		if !yes {
			options, err = ui.Settings(ctx, inspection, options)
			if err != nil {
				return err
			}
		}
		if err := options.Validate(inspection); err != nil {
			return err
		}
		var unique []string
		for _, id := range options.Agents {
			if !slices.Contains(unique, id) {
				unique = append(unique, id)
			}
		}
		options.Agents = unique
		if previousScope != options.Scope {
			scan, err = scan.ForScope(options.Scope, inspection)
			if err != nil {
				return err
			}
			choices, defaults, description = recommendationOptions(scan)
			if carried := availableSelection(choices, selected); len(carried) > 0 {
				defaults = carried
			}
			description = i18n.T("skills.batch.scope_changed") + "\n" + description
			selected, err = ui.Skills(ctx, i18n.T("skills.recommend.title"), description, choices, defaults)
			if err != nil {
				return err
			}
		}
		// Recheck placements and recorded sources immediately before review.
		scan, err = skillsmodule.Recommend(ctx, root)
		if err != nil {
			return err
		}
		scan, err = scan.ForScope(options.Scope, inspection)
		if err != nil {
			return err
		}
		groups, err = scan.InstallGroups(selected, options)
		if err != nil {
			return err
		}
		if len(groups) == 0 {
			break
		}
		if yes {
			break
		}
		action, err := ui.Review(ctx, groups, options, inspection)
		if err != nil {
			return err
		}
		if action == "cancel" {
			return nil
		}
		if action == "install" {
			break
		}
		back = true
	}
	result := &skillsmodule.BatchResult{Schema: "one-cli/install-skills/v1", Root: root, Options: options, Targets: []skillsmodule.TargetResult{}}
	skipExisting := true
	for len(groups) > 0 {
		round, err := installer.InstallGroups(ctx, root, groups, options, skipExisting, func(index, total int, group skillsmodule.InstallGroup) {
			fmt.Fprintln(cmd.ErrOrStderr(), i18n.Tf("skills.batch.progress", index, total, group.Source, strings.Join(group.Names, ", ")))
		})
		result.Merge(round)
		if err != nil {
			output.Emit(result)
			return err
		}
		groups = result.RetryGroups()
		if len(groups) == 0 || yes {
			break
		}
		result.RenderTTY(cmd.ErrOrStderr())
		retry, err := ui.Retry(ctx)
		if err != nil {
			output.Emit(result)
			return err
		}
		if !retry {
			break
		}
		skipExisting = false
	}
	output.Emit(result)
	if len(result.RetryGroups()) > 0 {
		return &platformprocess.ExitStatus{Code: 1}
	}
	return nil
}

func availableSelection(options []prompt.Option[string], selected []string) []string {
	return slices.DeleteFunc(slices.Clone(selected), func(name string) bool {
		return !slices.ContainsFunc(options, func(option prompt.Option[string]) bool { return option.Value == name })
	})
}

func defaultAgents(inspection *skillsmodule.Inspection, scope string) []string {
	var ids []string
	for _, agent := range inspection.Agents {
		if agent.Detected && (scope != "global" || agent.GlobalSkillsDir != nil) {
			ids = append(ids, agent.ID)
		}
	}
	if len(ids) == 0 {
		ids = []string{"universal"}
	}
	return ids
}

func chooseInstallSettings(ctx context.Context, inspection *skillsmodule.Inspection, options skillsmodule.InstallOptions) (skillsmodule.InstallOptions, error) {
	scope, err := prompt.SelectContext(ctx, i18n.T("skills.batch.scope_title"), "", []prompt.Option[string]{
		{Label: i18n.T("skills.batch.scope.project"), Value: "project"},
		{Label: i18n.T("skills.batch.scope.global"), Value: "global"},
	}, options.Scope)
	if err != nil {
		return options, err
	}
	options.Scope = scope
	agents := slices.Clone(inspection.Agents)
	slices.SortStableFunc(agents, func(a, b skillsmodule.Agent) int {
		if a.Detected != b.Detected {
			if a.Detected {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	var choices []prompt.Option[string]
	for _, agent := range agents {
		if scope == "global" && agent.GlobalSkillsDir == nil {
			continue
		}
		choices = append(choices, prompt.Option[string]{Label: agent.Name, Value: agent.ID})
	}
	selected := availableSelection(choices, options.Agents)
	if len(selected) == 0 {
		selected = defaultAgents(inspection, scope)
	}
	options.Agents, err = prompt.MultiSelectValidatedContext(ctx, i18n.T("skills.batch.agents_title"), i18n.T("skills.batch.agents_hint"), choices, selected, func(names []string) error {
		if len(names) == 0 {
			return i18n.Errorf("skills.batch.no_agents")
		}
		return nil
	})
	if err != nil {
		return options, err
	}
	options.Mode, err = prompt.SelectContext(ctx, i18n.T("skills.batch.mode_title"), "", []prompt.Option[string]{
		{Label: i18n.T("skills.batch.mode.symlink"), Value: "symlink"},
		{Label: i18n.T("skills.batch.mode.copy"), Value: "copy"},
	}, options.Mode)
	return options, err
}

func reviewInstallation(ctx context.Context, groups []skillsmodule.InstallGroup, options skillsmodule.InstallOptions, inspection *skillsmodule.Inspection) (string, error) {
	var names, agents []string
	for _, id := range options.Agents {
		name := id
		for _, agent := range inspection.Agents {
			if agent.ID == id {
				name = agent.Name
				break
			}
		}
		agents = append(agents, name)
	}
	var sources []string
	for _, group := range groups {
		names = append(names, group.Names...)
		sources = append(sources, "  "+group.Source+": "+strings.Join(group.Names, ", "))
	}
	description := i18n.Tf("skills.batch.review_summary", len(names), len(groups), strings.Join(agents, ", "), i18n.T("skills.batch.scope."+options.Scope), i18n.T("skills.batch.mode."+options.Mode))
	description += "\n\n" + strings.Join(sources, "\n")
	return prompt.SelectContext(ctx, i18n.T("skills.batch.review_title"), description, []prompt.Option[string]{
		{Label: i18n.T("skills.batch.install"), Value: "install"},
		{Label: i18n.T("skills.batch.back"), Value: "back"},
		{Label: i18n.T("skills.batch.cancel"), Value: "cancel"},
	}, "install")
}
