package prompt

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// defaultTheme is a clack-ish minimalist theme: thin cyan accents, gray
// descriptions, ◇/◆ glyphs for the prompt cursor, no thick left border.
// Huh supplies the terminal background for accents. Body text keeps the
// terminal foreground because some terminals cannot report their background.
//
// Centralised here so every prompt helper (Text / Select / Confirm / …)
// applies it consistently. Callers don't need to know about huh.Theme.
func defaultTheme() huh.Theme {
	return huh.ThemeFunc(themeStyles)
}

func themeStyles(isDark bool) *huh.Styles {
	t := huh.ThemeBase(isDark)
	lightDark := lipgloss.LightDark(isDark)

	var (
		// Soft cyan for active selection / cursor — clack uses cyan as its
		// signature accent. LightDark lets terminals with light bg pick
		// the deeper shade.
		accent = lightDark(lipgloss.Color("#0E7490"), lipgloss.Color("#22D3EE"))
		// Muted gray for descriptions / placeholders / blurred state.
		muted = lightDark(lipgloss.Color("#6B7280"), lipgloss.Color("#9CA3AF"))
		// Subtle green for confirmed selections.
		success = lightDark(lipgloss.Color("#16A34A"), lipgloss.Color("#4ADE80"))
		// Muted red for errors — not blood-red, easier on the eyes.
		danger = lightDark(lipgloss.Color("#DC2626"), lipgloss.Color("#F87171"))
		// Buttons paint their own background, so keep a matching foreground.
		buttonText = lightDark(lipgloss.Color("#111827"), lipgloss.Color("#E5E7EB"))
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
	// Keep unselected labels readable even when background detection falls
	// back to the wrong light/dark mode. The terminal owns their foreground.
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.UnsetForeground()
	t.Focused.Option = t.Focused.Option.UnsetForeground()

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
		Foreground(buttonText).
		Background(lightDark(lipgloss.Color("#E5E7EB"), lipgloss.Color("#1F2937")))
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
