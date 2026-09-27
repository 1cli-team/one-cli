package prompt

import (
	"charm.land/bubbles/v2/key"
	"charm.land/huh/v2"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// defaultKeyMap keeps the library bindings and localizes only their help labels.
func defaultKeyMap() *huh.KeyMap {
	m := huh.NewDefaultKeyMap()
	set := func(id string, bindings ...*key.Binding) {
		for _, binding := range bindings {
			binding.SetHelp(binding.Help().Key, i18n.T(id))
		}
	}
	set("prompt.key.complete", &m.Input.AcceptSuggestion)
	set("prompt.key.back", &m.Input.Prev, &m.FilePicker.Back, &m.FilePicker.Prev, &m.Text.Prev, &m.Select.Prev, &m.MultiSelect.Prev, &m.Note.Prev, &m.Confirm.Prev)
	set("prompt.key.next", &m.Input.Next, &m.FilePicker.Next, &m.Text.Next, &m.Note.Next, &m.Confirm.Next)
	set("prompt.key.submit", &m.Input.Submit, &m.FilePicker.Submit, &m.Text.Submit, &m.Select.Submit, &m.MultiSelect.Submit, &m.Note.Submit, &m.Confirm.Submit)
	set("prompt.key.first", &m.FilePicker.GotoTop)
	set("prompt.key.last", &m.FilePicker.GotoBottom)
	set("prompt.key.page_up", &m.FilePicker.PageUp)
	set("prompt.key.page_down", &m.FilePicker.PageDown)
	set("prompt.key.select", &m.FilePicker.Select, &m.Select.Next)
	set("prompt.key.up", &m.FilePicker.Up, &m.Select.Up, &m.MultiSelect.Up)
	set("prompt.key.down", &m.FilePicker.Down, &m.Select.Down, &m.MultiSelect.Down)
	set("prompt.key.open", &m.FilePicker.Open)
	set("prompt.key.close", &m.FilePicker.Close)
	set("prompt.key.new_line", &m.Text.NewLine)
	set("prompt.key.open_editor", &m.Text.Editor)
	set("prompt.key.left", &m.Select.Left)
	set("prompt.key.right", &m.Select.Right)
	set("prompt.key.filter", &m.Select.Filter, &m.MultiSelect.Filter)
	set("prompt.key.set_filter", &m.Select.SetFilter, &m.MultiSelect.SetFilter)
	set("prompt.key.clear_filter", &m.Select.ClearFilter, &m.MultiSelect.ClearFilter)
	set("prompt.key.half_page_up", &m.Select.HalfPageUp, &m.MultiSelect.HalfPageUp)
	set("prompt.key.half_page_down", &m.Select.HalfPageDown, &m.MultiSelect.HalfPageDown)
	set("prompt.key.go_to_start", &m.Select.GotoTop, &m.MultiSelect.GotoTop)
	set("prompt.key.go_to_end", &m.Select.GotoBottom, &m.MultiSelect.GotoBottom)
	set("prompt.key.confirm", &m.MultiSelect.Next)
	set("prompt.key.toggle", &m.MultiSelect.Toggle, &m.Confirm.Toggle)
	set("prompt.key.select_all", &m.MultiSelect.SelectAll)
	set("prompt.key.select_none", &m.MultiSelect.SelectNone)
	set("prompt.key.yes", &m.Confirm.Accept)
	set("prompt.key.no", &m.Confirm.Reject)
	return m
}
