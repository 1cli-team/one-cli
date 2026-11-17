package prompt

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/terminalui"
)

// defaultTheme keeps every prompt aligned with One's result and progress output.
// Body text and status colors follow the terminal; accents use One's brand color.
func defaultTheme() huh.Theme {
	return huh.ThemeFunc(themeStyles)
}

func themeStyles(isDark bool) *huh.Styles {
	t := huh.ThemeBase(isDark)
	s := terminalui.NewStyles()

	t.Focused.Base = lipgloss.NewStyle().
		MarginLeft(2).
		PaddingLeft(1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderLeft(true)
	if !terminalui.ColorDisabled() {
		t.Focused.Base = t.Focused.Base.BorderForeground(s.Accent.GetForeground())
	}
	t.Focused.Card = t.Focused.Base
	t.Focused.Title = s.Title
	t.Focused.NoteTitle = s.Title.MarginBottom(1)
	t.Focused.Directory = s.Accent
	t.Focused.Description = s.Muted

	t.Focused.SelectSelector = s.Accent.SetString("● ")
	t.Focused.MultiSelectSelector = s.Accent.SetString("› ")
	t.Focused.SelectedPrefix = s.Success.SetString("◆ ")
	t.Focused.UnselectedPrefix = s.Muted.SetString("◇ ")
	t.Focused.SelectedOption = s.Accent.Bold(true)
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.UnsetForeground()
	t.Focused.Option = t.Focused.Option.UnsetForeground()
	t.Focused.NextIndicator = s.Accent.MarginLeft(1).SetString("→")
	t.Focused.PrevIndicator = s.Accent.MarginRight(1).SetString("←")
	t.Focused.ErrorIndicator = s.Error.SetString(" ✗")
	t.Focused.ErrorMessage = s.Error.SetString(" ✗")
	t.Focused.TextInput.Prompt = s.Accent
	t.Focused.TextInput.Cursor = s.Accent
	t.Focused.TextInput.Placeholder = s.Muted

	// An underline and weight communicate focus without solid button fills.
	t.Focused.FocusedButton = s.Accent.Bold(true).Padding(0, 1).MarginRight(1).Underline(true)
	t.Focused.BlurredButton = lipgloss.NewStyle().Padding(0, 1).MarginRight(1)
	t.Focused.Next = t.Focused.FocusedButton
	t.Help.ShortKey, t.Help.FullKey = s.Accent, s.Accent
	t.Help.ShortDesc, t.Help.FullDesc = s.Muted, s.Muted
	t.Help.ShortSeparator, t.Help.FullSeparator, t.Help.Ellipsis = s.Muted, s.Muted, s.Muted

	t.Blurred = t.Focused
	t.Blurred.Base = t.Blurred.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.Card = t.Blurred.Base
	t.Blurred.MultiSelectSelector = lipgloss.NewStyle().SetString("  ")
	t.Blurred.NextIndicator = lipgloss.NewStyle()
	t.Blurred.PrevIndicator = lipgloss.NewStyle()
	t.Group.Base = lipgloss.NewStyle().PaddingLeft(2)
	t.Group.Title = t.Focused.Title
	t.Group.Description = t.Focused.Description
	return t
}
