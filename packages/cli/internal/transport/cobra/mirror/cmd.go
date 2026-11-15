package mirrorcmd

import (
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/mirror"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/process"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/prompt"
)

type result struct {
	Schema     string                     `json:"schema"`
	ConfigPath string                     `json:"config_path,omitempty"`
	Group      string                     `json:"group,omitempty"`
	Preset     string                     `json:"preset,omitempty"`
	Action     string                     `json:"action"`
	DryRun     bool                       `json:"dry_run"`
	Groups     map[string]string          `json:"groups,omitempty"`
	Presets    map[string][]mirror.Preset `json:"presets,omitempty"`
	Entries    []mirror.Entry             `json:"entries,omitempty"`
	Checks     []mirror.Check             `json:"checks,omitempty"`
}

func presetLabel(id string) string { return i18n.T("mirror.preset." + id) }
func groups(group string) []string {
	if group != "" {
		return []string{group}
	}
	return []string{mirror.EnvGroup, mirror.ToolsGroup}
}
func sourceKeys(group string) []string {
	if group == mirror.ToolsGroup {
		return []string{"mise", "process-compose"}
	}
	return []string{"node", "npm", "go", "goproxy", "pypi", "pub", "flutter"}
}

func Command() *cobra.Command {
	cmd := &cobra.Command{Use: "mirror", Args: i18n.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if !output.CanPrompt() {
			return emitStatus("", false)
		}
		if err := emitStatus("", true); err != nil {
			return err
		}
		var options []prompt.Option[string]
		for _, group := range groups("") {
			options = append(options, prompt.Option[string]{Label: i18n.T("mirror.group." + group), Value: group})
		}
		choice, err := prompt.SelectWithDescriptions(i18n.T("mirror.select_group"), options)
		if err != nil {
			return err
		}
		return pick(cmd, choice)
	}}
	i18n.MarkShort(cmd, "mirror.short")
	i18n.MarkLong(cmd, "mirror.long")
	cmd.Example = "  one mirror env use huawei\n  one mirror tools use gh-proxy\n  one mirror status\n  one mirror test\n  one mirror env reset"
	for _, group := range groups("") {
		groupCmd := &cobra.Command{Use: group, Args: i18n.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			if !output.CanPrompt() {
				return emitStatus(group, false)
			}
			return pick(cmd, group)
		}}
		i18n.MarkShort(groupCmd, "mirror.group."+group)
		groupCmd.AddCommand(actions(group)...)
		cmd.AddCommand(groupCmd)
	}
	// Preserve the original commands as environment-only aliases; status/test/reset
	// without a group operate on both settings.
	cmd.AddCommand(actions("")...)
	return cmd
}

func pick(cmd *cobra.Command, group string) error {
	var options []prompt.Option[string]
	for _, p := range mirror.Presets(group) {
		options = append(options, prompt.Option[string]{Label: p.ID, Description: presetLabel(p.ID), Value: p.ID})
	}
	options = append(options, prompt.Option[string]{Label: i18n.T("mirror.reset.short"), Value: "reset"})
	choice, err := prompt.SelectWithDescriptions(i18n.T("mirror.group."+group), options)
	if err != nil {
		return err
	}
	return apply(cmd, group, choice, false)
}

func actions(group string) []*cobra.Command {
	list := &cobra.Command{Use: "list", Args: i18n.NoArgs, RunE: func(_ *cobra.Command, _ []string) error {
		path, err := mirror.Path()
		if err != nil {
			return err
		}
		presets := map[string][]mirror.Preset{}
		for _, g := range groups(group) {
			presets[g] = mirror.Presets(g)
		}
		output.Emit(&result{Schema: "one-cli/mirror/v2", Action: "list", Group: group, ConfigPath: path, Presets: presets})
		return nil
	}}
	i18n.MarkShort(list, "mirror.list.short")
	status := &cobra.Command{Use: "status", Args: i18n.NoArgs, RunE: func(_ *cobra.Command, _ []string) error { return emitStatus(group, false) }}
	i18n.MarkShort(status, "mirror.status.short")
	useGroup := group
	if useGroup == "" {
		useGroup = mirror.EnvGroup
	}
	var ids []string
	for _, p := range mirror.Presets(useGroup) {
		ids = append(ids, p.ID)
	}
	var dryRun bool
	use := &cobra.Command{Use: "use <preset>", Args: i18n.ExactArgs(1), ValidArgs: ids, RunE: func(cmd *cobra.Command, args []string) error {
		if args[0] == "reset" {
			return i18n.Errorf("mirror.preset_unknown", args[0])
		}
		return apply(cmd, useGroup, args[0], dryRun)
	}}
	i18n.MarkShort(use, "mirror.use.short")
	use.Flags().BoolVar(&dryRun, "dry-run", false, "")
	i18n.MarkFlagUsage(use, "dry-run", "mirror.flag.dry_run")
	var resetDry bool
	reset := &cobra.Command{Use: "reset", Args: i18n.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return apply(cmd, group, "reset", resetDry) }}
	i18n.MarkShort(reset, "mirror.reset.short")
	reset.Flags().BoolVar(&resetDry, "dry-run", false, "")
	i18n.MarkFlagUsage(reset, "dry-run", "mirror.flag.dry_run")
	var timeout time.Duration
	test := &cobra.Command{Use: "test [preset]", Args: i18n.MaximumNArgs(1), ValidArgs: ids, RunE: func(cmd *cobra.Command, args []string) error {
		id := ""
		if len(args) > 0 {
			id = args[0]
		}
		dir, err := os.Getwd()
		if err != nil {
			return err
		}
		ctx, stop := process.SignalContext(cmd.Context())
		defer stop()
		checks, err := mirror.CheckSources(ctx, dir, os.Environ(), group, id, timeout)
		if err != nil {
			return err
		}
		output.Emit(&result{Schema: "one-cli/mirror/v2", Action: "test", Group: group, Preset: id, Checks: checks})
		for _, check := range checks {
			if check.Status == "failed" {
				return i18n.Errorf("mirror.test_failed")
			}
		}
		return nil
	}}
	i18n.MarkShort(test, "mirror.test.short")
	test.Flags().DurationVar(&timeout, "timeout", 8*time.Second, "")
	i18n.MarkFlagUsage(test, "timeout", "mirror.flag.timeout")
	return []*cobra.Command{list, status, use, reset, test}
}

func emitStatus(group string, summary bool) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	r, err := mirror.Resolve(dir, os.Environ())
	if err != nil {
		return err
	}
	path, err := mirror.Path()
	if err != nil {
		return err
	}
	value := &result{Schema: "one-cli/mirror/v2", Action: "status", Group: group, ConfigPath: path, Groups: map[string]string{}}
	for _, g := range groups(group) {
		value.Groups[g] = mirror.Current(r.Config, g).ID
	}
	if summary {
		value.Action = "summary"
	} else {
		for _, entry := range r.Entries {
			if group == "" || entry.Group == group {
				value.Entries = append(value.Entries, entry)
			}
		}
	}
	output.Emit(value)
	return nil
}

func apply(cmd *cobra.Command, group, id string, dryRun bool) error {
	path, err := mirror.Path()
	if err != nil {
		return err
	}
	value := &result{Schema: "one-cli/mirror/v2", Action: "use", Group: group, ConfigPath: path, Preset: id, DryRun: dryRun}
	if id == "reset" {
		value.Action, value.Preset = "reset", ""
		if err := mirror.Reset(cmd.Context(), group, dryRun); err != nil {
			return err
		}
	} else {
		cfg, err := mirror.Select(cmd.Context(), group, id, dryRun)
		if err != nil {
			return err
		}
		p := mirror.Current(cfg, group)
		for _, key := range sourceKeys(group) {
			value.Entries = append(value.Entries, mirror.Entry{Group: group, Target: key, URL: p.Sources[key], Source: "preset", Status: "configured"})
		}
	}
	output.Emit(value)
	return nil
}
