package prompt

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// defaultTheme is a clack-ish minimalist theme: thin cyan accents, gray
// descriptions, ◇/◆ glyphs for the prompt cursor, no thick left border.
// Tuned to read well on both light and dark terminals via lipgloss
// AdaptiveColor.
//
// Centralised here so every prompt helper (Text / Select / Confirm / …)
// applies it consistently. Callers don't need to know about huh.Theme.
func defaultTheme() *huh.Theme {
	t := huh.ThemeBase()

	var (
		// Soft cyan for active selection / cursor — clack uses cyan as its
		// signature accent. AdaptiveColor lets terminals with light bg pick
		// the deeper shade.
		accent = lipgloss.AdaptiveColor{Light: "#0E7490", Dark: "#22D3EE"}
		// Muted gray for descriptions / placeholders / blurred state.
		muted = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
		// Subtle green for confirmed selections.
		success = lipgloss.AdaptiveColor{Light: "#16A34A", Dark: "#4ADE80"}
		// Muted red for errors — not blood-red, easier on the eyes.
		danger = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"}
		// Body foreground colour — neutral, defers to the terminal scheme.
		body = lipgloss.AdaptiveColor{Light: "#111827", Dark: "#E5E7EB"}
	)

	// Replace the thick left border (huh default) with a thin one in the
	// accent colour. Reads more like clack's vertical connector.
	t.Focused.Base = lipgloss.NewStyle().
		PaddingLeft(1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderLeft(true).
		BorderForeground(accent)
	t.Focused.Card = t.Focused.Base

	t.Focused.Title = t.Focused.Title.Foreground(accent).Bold(true)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(accent).Bold(true).MarginBottom(1)
	t.Focused.Directory = t.Focused.Directory.Foreground(accent)
	t.Focused.Description = t.Focused.Description.Foreground(muted)

	// ◇ outlined diamond for the cursor — clack's hallmark glyph.
	t.Focused.SelectSelector = lipgloss.NewStyle().
		Foreground(accent).
		SetString("◇ ")
	t.Focused.MultiSelectSelector = lipgloss.NewStyle().
		Foreground(accent).
		SetString("◇ ")
	// ◆ filled diamond for picked entries in a multi-select.
	t.Focused.SelectedPrefix = lipgloss.NewStyle().
		Foreground(success).
		SetString("◆ ")
	t.Focused.UnselectedPrefix = lipgloss.NewStyle().
		Foreground(muted).
		SetString("◇ ")
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(success)
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(body)
	t.Focused.Option = t.Focused.Option.Foreground(body)

	t.Focused.NextIndicator = lipgloss.NewStyle().Foreground(accent).MarginLeft(1).SetString("→")
	t.Focused.PrevIndicator = lipgloss.NewStyle().Foreground(accent).MarginRight(1).SetString("←")

	t.Focused.ErrorIndicator = lipgloss.NewStyle().Foreground(danger).SetString(" ✗")
	t.Focused.ErrorMessage = lipgloss.NewStyle().Foreground(danger).SetString(" ✗")

	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(accent)
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(accent)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(muted)

	t.Focused.FocusedButton = t.Focused.FocusedButton.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(accent)
	t.Focused.BlurredButton = t.Focused.BlurredButton.
		Foreground(body).
		Background(lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#1F2937"})
	t.Focused.Next = t.Focused.FocusedButton

	// Blurred state: hide the border so non-active groups read as quiet.
	t.Blurred = t.Focused
	t.Blurred.Base = t.Blurred.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.Card = t.Blurred.Base
	t.Blurred.NextIndicator = lipgloss.NewStyle()
	t.Blurred.PrevIndicator = lipgloss.NewStyle()

	t.Group.Title = t.Focused.Title
	t.Group.Description = t.Focused.Description

	return t
}
