// Package prompt is a thin façade over charmbracelet/huh that maps
// huh-specific outcomes (ErrUserAborted, validation errors) into the
// CLI's central error contract.
//
// Callers should:
//  1. gate any prompt behind output.IsTTY() && !flags.yes
//  2. surface returned errors as-is (they already carry PROMPT_CANCELLED
//     or other structured codes)
//
// huh runs its own bubbletea program per Run(), which means each helper
// is a self-contained UI invocation — no global state, easy to test by
// stubbing at the call site.
package prompt

import (
	stderrors "errors"
	"fmt"

	"github.com/charmbracelet/huh"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
)

// Option mirrors huh.Option but keeps callers from importing huh directly.
// Generic over the value type so Select returns a strongly-typed result.
type Option[T comparable] struct {
	Label       string
	Description string
	Value       T
}

// Text shows a single-line input prompt. validate may be nil; if non-nil,
// a non-nil return is shown to the user and the prompt repeats.
func Text(title, placeholder string, validate func(string) error) (string, error) {
	var value string
	field := huh.NewInput().
		Title(title).
		Placeholder(placeholder).
		Value(&value)
	if validate != nil {
		field = field.Validate(validate)
	}
	if err := field.WithTheme(defaultTheme()).Run(); err != nil {
		return "", mapErr(err)
	}
	return value, nil
}

// Password shows a single-line input with masked echo. Used for secret
// values that shouldn't appear in scrollback.
func Password(title string, validate func(string) error) (string, error) {
	var value string
	field := huh.NewInput().
		Title(title).
		EchoMode(huh.EchoModePassword).
		Value(&value)
	if validate != nil {
		field = field.Validate(validate)
	}
	if err := field.WithTheme(defaultTheme()).Run(); err != nil {
		return "", mapErr(err)
	}
	return value, nil
}

// Confirm shows a yes/no prompt. defaultValue seeds the cursor; affirmative
// and negative override the default "Yes" / "No" labels (set to "" for
// huh's defaults). On Ctrl-C returns PROMPT_CANCELLED.
func Confirm(title string, defaultValue bool, affirmative, negative string) (bool, error) {
	value := defaultValue
	field := huh.NewConfirm().Title(title).Value(&value)
	if affirmative != "" {
		field = field.Affirmative(affirmative)
	}
	if negative != "" {
		field = field.Negative(negative)
	}
	if err := field.WithTheme(defaultTheme()).Run(); err != nil {
		return false, mapErr(err)
	}
	return value, nil
}

// Select shows a vertical menu of options. T is typically string but can
// be any comparable — callers wrap a label string + a typed value (e.g.
// existingDirMode constants).
func Select[T comparable](title string, options []Option[T]) (T, error) {
	var value T
	huhOpts := make([]huh.Option[T], 0, len(options))
	for _, opt := range options {
		huhOpts = append(huhOpts, huh.NewOption(opt.Label, opt.Value))
	}
	field := huh.NewSelect[T]().
		Title(title).
		Options(huhOpts...).
		Value(&value)
	if err := field.WithTheme(defaultTheme()).Run(); err != nil {
		return value, mapErr(err)
	}
	return value, nil
}

// MultiSelect lets the user toggle multiple options. preSelected values
// are checked when the prompt opens; pass nil for "nothing pre-checked".
//
// Description, when present on an option, is appended to the label as
// "<label> — <description>". Same encoding rationale as
// SelectWithDescriptions: keep accessible-mode output coherent.
func MultiSelect[T comparable](title string, options []Option[T], preSelected []T) ([]T, error) {
	values := append([]T{}, preSelected...)
	huhOpts := make([]huh.Option[T], 0, len(options))
	for _, opt := range options {
		label := opt.Label
		if opt.Description != "" {
			label = fmt.Sprintf("%s — %s", opt.Label, opt.Description)
		}
		huhOpts = append(huhOpts, huh.NewOption(label, opt.Value))
	}
	field := huh.NewMultiSelect[T]().
		Title(title).
		Options(huhOpts...).
		Value(&values)
	if err := field.WithTheme(defaultTheme()).Run(); err != nil {
		return nil, mapErr(err)
	}
	return values, nil
}

// SelectWithDescriptions is like Select but renders a per-option description
// line below each entry. Useful for template pickers where the user needs
// to read what each option actually does.
//
// huh's Option doesn't carry a description natively, so we encode it into
// the visible label as "<label> — <description>". This keeps a11y mode
// (screen readers / accessible terminals) coherent without extra setup.
func SelectWithDescriptions[T comparable](title string, options []Option[T]) (T, error) {
	flat := make([]Option[T], 0, len(options))
	for _, opt := range options {
		label := opt.Label
		if opt.Description != "" {
			label = fmt.Sprintf("%s — %s", opt.Label, opt.Description)
		}
		flat = append(flat, Option[T]{Label: label, Value: opt.Value})
	}
	return Select(title, flat)
}

// mapErr translates huh sentinels into our central error catalogue. Anything
// we don't recognise becomes a generic ONE_CLI_ERROR so it still goes through
// the JSON envelope.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if stderrors.Is(err, huh.ErrUserAborted) {
		// PROMPT_CANCELLED is a cooperative cancel — emit the envelope,
		// but exit 0 so scripts treat Ctrl-C the same way TS did.
		return cliErrors.New(cliErrors.PROMPT_CANCELLED, "操作已取消。").
			WithExit0().
			WithRemediation(output.Remediation{
				Action: "rerun-with-yes",
				Hint:   "如果你只是想跳过提问，可以加 --yes 走纯非交互模式。",
			})
	}
	return cliErrors.New(cliErrors.ONE_CLI_ERROR, err.Error())
}
