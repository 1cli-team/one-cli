package tasks

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"golang.org/x/term"
)

type catalogLayout struct {
	width          int
	color, verbose bool
}

func RenderCatalog(w io.Writer, tasks []Task, verbose bool) {
	layout := catalogLayout{width: 100, verbose: verbose}
	if terminal, ok := w.(*os.File); ok && term.IsTerminal(int(terminal.Fd())) {
		if width, _, err := term.GetSize(int(terminal.Fd())); err == nil && width > 0 {
			layout.width = width
		}
		layout.color = os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb" && os.Getenv("CLICOLOR") != "0" && os.Getenv("FORCE_COLOR") != "0"
	}
	layout.render(w, tasks)
}

type catalogGroup struct {
	project, scope string
	tasks          []Task
}

// Group by task namespace, not its effective working directory: a root build
// entry can run inside a project without becoming a project-prefixed task.
func groupCatalog(tasks []Task) []catalogGroup {
	groups := map[string]*catalogGroup{}
	for _, task := range tasks {
		key, project, scope := "", "", ""
		if !strings.HasPrefix(task.Name, "//:") && strings.HasPrefix(task.Name, "//") {
			scope, _, _ = strings.Cut(task.Name, ":")
			key = "scope:" + scope
			if task.Project != "" {
				project = task.Project
				scope = ""
				key = "project:" + project
			}
		} else if task.Project != "" && strings.HasPrefix(strings.TrimPrefix(task.Name, "//:"), task.Project+":") {
			project = task.Project
			key = "project:" + project
		}
		if groups[key] == nil {
			groups[key] = &catalogGroup{project: project, scope: scope}
		}
		groups[key].tasks = append(groups[key].tasks, task)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]catalogGroup, 0, len(keys))
	for _, key := range keys {
		group := groups[key]
		sort.SliceStable(group.tasks, func(i, j int) bool { return group.tasks[i].Name < group.tasks[j].Name })
		out = append(out, *group)
	}
	return out
}

func catalogText(text string) string { return strings.Join(strings.Fields(ansi.Strip(text)), " ") }

func (l catalogLayout) render(w io.Writer, tasks []Task) {
	l.width = max(8, l.width)
	paint := func(text string, style lipgloss.Style) string {
		if l.color {
			return style.Render(text)
		}
		return text
	}
	titleStyle := lipgloss.NewStyle().Bold(true)
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	// Wrap first, then style each line: continuation rows keep their indentation.
	write := func(text string, indent int, style lipgloss.Style) {
		for _, line := range strings.Split(ansi.Wrap(catalogText(text), max(1, l.width-indent), ""), "\n") {
			fmt.Fprintln(w, strings.Repeat(" ", indent)+paint(line, style))
		}
	}
	if len(tasks) == 0 {
		write(i18n.T("tasks.list_empty"), 0, titleStyle)
		return
	}
	column := 0
	for _, task := range tasks {
		column = max(column, ansi.StringWidth(catalogText(strings.TrimPrefix(task.Name, "//:"))))
	}
	column = min(column, min(32, max(12, l.width/3)))
	for index, group := range groupCatalog(tasks) {
		if index > 0 {
			fmt.Fprintln(w)
		}
		title := i18n.Tf("tasks.list_workspace", len(group.tasks))
		if group.project != "" {
			title = i18n.Tf("tasks.list_project", group.project, len(group.tasks))
		} else if group.scope != "" {
			title = i18n.Tf("tasks.list_scope", group.scope, len(group.tasks))
		}
		write(title, 0, titleStyle)
		for i, task := range group.tasks {
			if i > 0 && (l.width < 64 || l.verbose) {
				fmt.Fprintln(w)
			}
			name := catalogText(strings.TrimPrefix(task.Name, "//:"))
			description := catalogText(task.Description)
			if l.width >= 64 && ansi.StringWidth(name) <= column && description != "" {
				lines := strings.Split(ansi.Wrap(description, l.width-column-4, ""), "\n")
				fmt.Fprintf(w, "  %s%s  %s\n", paint(name, nameStyle), strings.Repeat(" ", column-ansi.StringWidth(name)), lines[0])
				for _, line := range lines[1:] {
					fmt.Fprintln(w, strings.Repeat(" ", column+4)+line)
				}
			} else {
				write(name, 2, nameStyle)
				if description != "" {
					write(description, 4, lipgloss.NewStyle())
				}
			}
			if l.verbose {
				boolean := func(value bool) string {
					if value {
						return i18n.T("common.yes")
					}
					return i18n.T("common.no")
				}
				write(i18n.Tf("tasks.list_details", task.Source, boolean(task.Cached), boolean(task.Interactive || task.Raw)), 4, lipgloss.NewStyle())
			}
		}
	}
	fmt.Fprintln(w)
	write(i18n.T("tasks.list_run_hint"), 0, lipgloss.NewStyle())
	write(i18n.T("tasks.list_options_hint"), 0, lipgloss.NewStyle())
}
