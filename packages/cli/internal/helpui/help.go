// Package helpui renders the user-facing CLI help surface. It keeps Cobra as
// the command/flag source of truth while presenting flags in two groups:
// everyday options and automation/advanced options.
package helpui

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/i18n"
)

const advancedAnnotation = "one.help.advanced"

// MarkAdvanced moves named local flags into the automation/advanced section
// of the command help. Unknown names are ignored so callers can annotate a
// shared list across small command variants.
func MarkAdvanced(cmd *cobra.Command, names ...string) {
	if cmd == nil {
		return
	}
	for _, name := range names {
		if flag := cmd.Flags().Lookup(name); flag != nil {
			if flag.Annotations == nil {
				flag.Annotations = map[string][]string{}
			}
			flag.Annotations[advancedAnnotation] = []string{"true"}
		}
	}
}

// Render is installed as the Cobra HelpFunc for subcommands. Root help is
// handled separately because it has both a concise and a complete form.
func Render(cmd *cobra.Command, _ []string) {
	w := cmd.OutOrStdout()
	if flag := cmd.Flags().Lookup("help"); flag != nil {
		flag.Usage = i18n.T("common.flag.help")
	}
	description := strings.TrimSpace(cmd.Short)
	if description == "" {
		description = firstParagraph(cmd.Long)
	}
	section(w, i18n.T("help.description"))
	fmt.Fprintln(w, description)

	section(w, i18n.T("help.usage"))
	fmt.Fprintf(w, "  %s\n", cmd.UseLine())

	children := visibleChildren(cmd)
	if len(children) > 0 {
		section(w, i18n.T("help.subcommands"))
		for _, child := range children {
			fmt.Fprintf(w, "  %-14s %s\n", child.Name(), child.Short)
		}
	}

	if example := strings.TrimSpace(cmd.Example); example != "" {
		section(w, i18n.T("help.examples"))
		writeIndented(w, example, "  ")
	}

	if tips := strings.TrimSpace(cmd.Long); tips != "" && tips != description {
		section(w, i18n.T("help.tips"))
		fmt.Fprintln(w, tips)
	}

	common, advanced := splitFlags(cmd)
	if len(common) > 0 {
		section(w, i18n.T("help.common_options"))
		writeFlags(w, common)
	}
	if len(advanced) > 0 {
		section(w, i18n.T("help.advanced_options"))
		writeFlags(w, advanced)
	}
}

// RenderAll builds the complete command catalogue from the registered Cobra
// tree, so adding a command cannot leave `one help --all` stale.
func RenderAll(root *cobra.Command, w io.Writer) {
	fmt.Fprintln(w, i18n.T("root.help_all_title"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("help.all_intro"))
	fmt.Fprintln(w)
	for _, cmd := range visibleChildren(root) {
		fmt.Fprintf(w, "  %-14s %s\n", cmd.Name(), cmd.Short)
		for _, child := range visibleChildren(cmd) {
			fmt.Fprintf(w, "    %-12s %s\n", child.Name(), child.Short)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T("help.all_tip"))
}

func section(w io.Writer, title string) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, title)
}

func firstParagraph(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "\n\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

func visibleChildren(cmd *cobra.Command) []*cobra.Command {
	children := make([]*cobra.Command, 0, len(cmd.Commands()))
	for _, child := range cmd.Commands() {
		if child.Hidden || child.Deprecated != "" || child.Name() == "help" {
			continue
		}
		children = append(children, child)
	}
	sort.Slice(children, func(i, j int) bool { return children[i].Name() < children[j].Name() })
	return children
}

func splitFlags(cmd *cobra.Command) (common, advanced []*pflag.Flag) {
	seen := map[string]bool{}
	collect := func(set *pflag.FlagSet) {
		set.VisitAll(func(flag *pflag.Flag) {
			if seen[flag.Name] || flag.Hidden {
				return
			}
			seen[flag.Name] = true
			if values := flag.Annotations[advancedAnnotation]; len(values) > 0 && values[0] == "true" {
				advanced = append(advanced, flag)
				return
			}
			common = append(common, flag)
		})
	}
	collect(cmd.NonInheritedFlags())
	collect(cmd.InheritedFlags())
	sort.Slice(common, func(i, j int) bool { return common[i].Name < common[j].Name })
	sort.Slice(advanced, func(i, j int) bool { return advanced[i].Name < advanced[j].Name })
	return common, advanced
}

func writeFlags(w io.Writer, flags []*pflag.Flag) {
	for _, flag := range flags {
		name := "--" + flag.Name
		if flag.Shorthand != "" {
			name = "-" + flag.Shorthand + ", " + name
		}
		if flag.NoOptDefVal == "" {
			name += " <value>"
		}
		usage := flag.Usage
		if keys := flag.Annotations[i18n.AnnotationFlag]; len(keys) > 0 {
			usage = i18n.T(keys[0])
		}
		fmt.Fprintf(w, "  %-24s %s\n", name, usage)
	}
}

func writeIndented(w io.Writer, s, prefix string) {
	for _, line := range strings.Split(s, "\n") {
		fmt.Fprintln(w, prefix+strings.TrimPrefix(line, "  "))
	}
}
