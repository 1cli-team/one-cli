package prompt

import "testing"

func TestNewForm_AppendsFields(t *testing.T) {
	var dir, name, choice string
	var confirmed bool

	f := NewForm().
		Text(&dir, "dir", "./my-app", nil).
		Text(&name, "name", "billing", nil).
		Select(&choice, "mode", []Option[string]{
			{Label: "fast", Value: "fast"},
			{Label: "slow", Value: "slow"},
		}).
		Confirm(&confirmed, "ok?")

	if got := len(f.fields); got != 4 {
		t.Errorf("fields = %d, want 4", got)
	}
}

func TestNewForm_EmptyRunIsNoop(t *testing.T) {
	// An empty form should return nil without trying to start a
	// bubbletea program (which would block on stdin in CI).
	if err := NewForm().Run(); err != nil {
		t.Errorf("empty Run() = %v, want nil", err)
	}
}
