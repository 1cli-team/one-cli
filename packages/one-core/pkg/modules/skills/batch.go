package skills

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type BatchResult struct {
	Schema   string         `json:"schema"`
	Root     string         `json:"root"`
	Options  InstallOptions `json:"options"`
	Targets  []TargetResult `json:"targets"`
	Warnings []string       `json:"warnings,omitempty"`
}

func (o InstallOptions) Validate(inspection *Inspection) error {
	if o.Scope != "project" && o.Scope != "global" {
		return i18n.Errorf("skills.batch.invalid_scope", o.Scope)
	}
	if o.Mode != "symlink" && o.Mode != "copy" {
		return i18n.Errorf("skills.batch.invalid_mode", o.Mode)
	}
	if len(o.Agents) == 0 {
		return i18n.Errorf("skills.batch.no_agents")
	}
	for _, id := range o.Agents {
		index := slices.IndexFunc(inspection.Agents, func(agent Agent) bool { return agent.ID == id })
		if index < 0 {
			return i18n.Errorf("skills.batch.unknown_agent", id)
		}
		if o.Scope == "global" && inspection.Agents[index].GlobalSkillsDir == nil {
			return i18n.Errorf("skills.batch.global_unsupported", id)
		}
	}
	return nil
}

// InstallGroups serializes lock-file writes across sources. Failures are kept
// per target, so a later retry cannot reinstall already successful targets.
func (s Installer) InstallGroups(ctx context.Context, root string, groups []InstallGroup, options InstallOptions, skipExisting bool, progress func(int, int, InstallGroup)) (*BatchResult, error) {
	result := &BatchResult{Schema: "one-cli/install-skills/v1", Root: root, Options: options, Targets: []TargetResult{}}
	if s.InstallGroup == nil {
		return result, i18n.Errorf("skills.runner_required")
	}
	for index, group := range groups {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if progress != nil {
			progress(index+1, len(groups), group)
		}
		report, err := s.InstallGroup(ctx, root, group, options, skipExisting)
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		if report == nil {
			report = &GroupResult{}
		}
		for _, name := range group.Names {
			for _, agent := range group.Agents {
				item := TargetResult{Skill: name, Agent: agent, Source: group.Source, Ref: group.Ref, Status: "failed"}
				if err != nil {
					item.Message = err.Error()
				} else if found := slices.IndexFunc(report.Targets, func(target TargetResult) bool {
					return target.Skill == name && target.Agent == agent
				}); found >= 0 {
					item = report.Targets[found]
					item.Source = group.Source
					if item.Status != "installed" && item.Status != "skipped" && item.Status != "failed" {
						item.Status, item.Message = "failed", i18n.T("skills.batch.missing_result")
					}
					if item.Code == "SKILLS_AGENT_DIRECTORY_MISSING" {
						item.Message = i18n.Tf("skills.batch.agent_directory_missing", agent)
					}
				} else {
					item.Message = i18n.T("skills.batch.missing_result")
				}
				result.Targets = append(result.Targets, item)
			}
		}
		result.Warnings = append(result.Warnings, report.Warnings...)
	}
	return result, nil
}

func (r *BatchResult) Merge(next *BatchResult) {
	for _, target := range next.Targets {
		index := slices.IndexFunc(r.Targets, func(old TargetResult) bool {
			return old.Source == target.Source && old.Skill == target.Skill && old.Agent == target.Agent
		})
		if index >= 0 {
			r.Targets[index] = target
		} else {
			r.Targets = append(r.Targets, target)
		}
	}
	for _, warning := range next.Warnings {
		if !slices.Contains(r.Warnings, warning) {
			r.Warnings = append(r.Warnings, warning)
		}
	}
}

func (r *BatchResult) RetryGroups() []InstallGroup {
	var skills []InstallGroup
	for _, target := range r.Targets {
		if target.Status != "failed" {
			continue
		}
		index := slices.IndexFunc(skills, func(group InstallGroup) bool {
			return group.Source == target.Source && group.Ref == target.Ref && group.Names[0] == target.Skill
		})
		if index < 0 {
			skills = append(skills, InstallGroup{Source: target.Source, Ref: target.Ref, Names: []string{target.Skill}})
			index = len(skills) - 1
		}
		skills[index].Agents = append(skills[index].Agents, target.Agent)
	}
	var groups []InstallGroup
	for _, skill := range skills {
		slices.Sort(skill.Agents)
		skill.Agents = slices.Compact(skill.Agents)
		index := slices.IndexFunc(groups, func(group InstallGroup) bool {
			return group.Source == skill.Source && group.Ref == skill.Ref && slices.Equal(group.Agents, skill.Agents)
		})
		if index < 0 {
			groups = append(groups, skill)
		} else {
			groups[index].Names = append(groups[index].Names, skill.Names...)
		}
	}
	return groups
}

func (r *BatchResult) RenderTTY(w io.Writer) {
	if len(r.Targets) == 0 {
		fmt.Fprintln(w, i18n.T("skills.batch.no_selection"))
		return
	}
	states := map[string]string{}
	var names []string
	for _, target := range r.Targets {
		if _, found := states[target.Skill]; !found {
			names = append(names, target.Skill)
		}
		if target.Status == "failed" || states[target.Skill] == "failed" {
			states[target.Skill] = "failed"
		} else if target.Status == "installed" || states[target.Skill] == "installed" {
			states[target.Skill] = "installed"
		} else {
			states[target.Skill] = "skipped"
		}
	}
	counts := map[string]int{}
	for _, state := range states {
		counts[state]++
	}
	fmt.Fprintln(w, i18n.Tf("skills.batch.summary", counts["installed"], counts["failed"], counts["skipped"]))
	for _, name := range names {
		var agents []string
		for _, target := range r.Targets {
			if target.Skill != name {
				continue
			}
			if target.Status == "failed" {
				fmt.Fprintln(w, i18n.Tf("skills.batch.target_failed", name, target.Agent, target.Message))
			} else if target.Status == "installed" {
				agents = append(agents, target.Agent)
			}
		}
		if len(agents) > 0 {
			fmt.Fprintln(w, i18n.Tf("skills.batch.target_installed", name, strings.Join(agents, ", ")))
		}
		if states[name] == "skipped" {
			fmt.Fprintln(w, i18n.Tf("skills.batch.target_skipped", name))
		}
	}
	for _, warning := range r.Warnings {
		fmt.Fprintln(w, warning)
	}
}
