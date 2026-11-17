// Package terminalui provides the shared visual language for One's terminal output.
// It only formats human-readable output; callers still own output mode and content.
package terminalui

import (
	"os"

	"charm.land/lipgloss/v2"
)

// Styles uses One Desktop's primary orange for branding and the terminal's
// palette for status colors. Construct it at render time to respect NO_COLOR.
type Styles struct {
	Title, Accent, Muted, Success, Warning, Error lipgloss.Style
}

func NewStyles() Styles {
	s := Styles{Title: lipgloss.NewStyle().Bold(true)}
	if ColorDisabled() {
		return s
	}
	// Keep this aligned with --color-primary-500 in apps/desktop-renderer/src/styles/tokens.css.
	s.Accent = lipgloss.NewStyle().Foreground(lipgloss.Color("#ea580c"))
	s.Muted = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	s.Success = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	s.Warning = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	s.Error = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	return s
}

func ColorDisabled() bool {
	return os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" ||
		os.Getenv("CLICOLOR") == "0" || os.Getenv("FORCE_COLOR") == "0"
}
