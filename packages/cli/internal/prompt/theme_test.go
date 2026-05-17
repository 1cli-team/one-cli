package prompt

import "testing"

func TestDefaultTheme_NonNilFields(t *testing.T) {
	// Smoke check that defaultTheme constructs without panic and that
	// the fields huh's renderer dereferences are populated. Visual
	// correctness isn't testable without a PTY; this just guards
	// against regressions where a theme refactor leaves a nil Style.
	tm := defaultTheme()
	if tm == nil {
		t.Fatal("defaultTheme returned nil")
	}
	// Render with an empty string — lipgloss styles with nil internal
	// state would panic here.
	_ = tm.Focused.Title.Render("title")
	_ = tm.Focused.Description.Render("desc")
	_ = tm.Focused.SelectSelector.Render()
	_ = tm.Focused.SelectedPrefix.Render()
	_ = tm.Focused.UnselectedPrefix.Render()
	_ = tm.Focused.ErrorMessage.Render(" oops")
	_ = tm.Focused.TextInput.Prompt.Render(">")
}

func TestDefaultTheme_DistinctFromHuhDefault(t *testing.T) {
	// Sanity check that we're actually overriding huh's defaults — if
	// the override breaks (e.g. someone refactors theme.go to a no-op),
	// SelectSelector should differ from huh.ThemeBase()'s "> ".
	tm := defaultTheme()
	rendered := tm.Focused.SelectSelector.Render()
	if rendered == "> " {
		t.Errorf("SelectSelector reverted to huh's default %q; theme override is no-op", rendered)
	}
}
