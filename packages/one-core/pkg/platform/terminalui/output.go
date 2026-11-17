package terminalui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

type UI struct {
	w      io.Writer
	width  int
	color  bool
	styles Styles
}

// New detects the destination rather than stdout: errors and progress use stderr.
// Redirected text stays free of escape sequences even when color is forced.
func New(w io.Writer) *UI {
	u := &UI{w: w, width: 80, styles: NewStyles()}
	if terminal, ok := w.(interface{ Fd() uintptr }); ok && term.IsTerminal(int(terminal.Fd())) {
		u.color = !ColorDisabled()
		if width, _, err := term.GetSize(int(terminal.Fd())); err == nil && width > 1 {
			u.width = width - 1
		}
	}
	return u
}

func (u *UI) Header(command string) {
	u.Blank()
	u.line("  ", u.styles.Accent.Bold(true).Render("ONE")+"  "+u.styles.Muted.Render(command))
	u.Blank()
}

func (u *UI) Success(message string) {
	u.line("  "+u.styles.Success.Render("✓")+" ", u.styles.Title.Render(message))
}

func (u *UI) Warning(message string) {
	u.line("  "+u.styles.Warning.Render("!")+" ", message)
}

func (u *UI) Error(message string) {
	u.line("  "+u.styles.Error.Render("✗")+" ", message)
}

func (u *UI) Section(title string) {
	u.Blank()
	u.line("  ", u.styles.Title.Render(title))
}

func (u *UI) Text(message string) { u.line("  ", message) }

func (u *UI) Note(message string) { u.line("  ", u.styles.Muted.Render(message)) }

// Command leaves the command intact for copying; the terminal handles soft wrap.
func (u *UI) Command(command string) {
	u.write("    " + u.styles.Accent.Render(command))
}

type Field struct {
	Label, Value string
}

func (u *UI) Fields(fields ...Field) {
	labelWidth := 0
	for _, field := range fields {
		labelWidth = max(labelWidth, ansi.StringWidth(field.Label))
	}
	for _, field := range fields {
		prefix := "  " + u.styles.Accent.Render("➜") + "  "
		// Stack values when aligned columns would leave little readable space.
		if u.width-labelWidth-7 < 20 {
			u.line(prefix, u.styles.Title.Render(field.Label))
			u.line("     ", u.styles.Accent.Render(field.Value))
			continue
		}
		prefix += u.styles.Title.Render(field.Label) + strings.Repeat(" ", labelWidth-ansi.StringWidth(field.Label)+2)
		u.line(prefix, u.styles.Accent.Render(field.Value))
	}
}

func (u *UI) Blank() { fmt.Fprintln(u.w) }

func (u *UI) line(prefix, message string) {
	indent := ansi.StringWidth(prefix)
	lines := strings.Split(ansi.Wrap(message, max(1, u.width-indent), ""), "\n")
	for i, line := range lines {
		if i == 0 {
			u.write(prefix + line)
		} else {
			u.write(strings.Repeat(" ", indent) + line)
		}
	}
}

func (u *UI) write(line string) {
	if !u.color {
		fmt.Fprintln(u.w, ansi.Strip(line))
		return
	}
	lipgloss.Fprintln(u.w, line)
}
