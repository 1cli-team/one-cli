package prompt

import (
	"github.com/charmbracelet/huh"
)

// Form is a fluent builder for multi-step prompts that share one
// screen — the user can shift+tab back to revise an earlier answer
// before hitting enter on the last field. Use this when 2+ prompts
// would otherwise run sequentially and they're independent or only
// depend on prior fields in straightforward ways.
//
// All methods return the Form so callers can chain. Run() submits
// the whole form; if any field has Validate set, it gates advance.
//
// Currently supports string-valued fields only. If callers need
// typed select inside a Form (e.g. an enum), add a typed builder
// rather than introducing a generic Form[T] (Go disallows generic
// methods, so the chain wouldn't compose cleanly).
type Form struct {
	fields []huh.Field
}

// NewForm returns an empty Form. Add fields via Text/Select/Confirm,
// then call Run.
func NewForm() *Form { return &Form{} }

// Text adds a single-line text field bound to *out. validate may be nil.
// placeholder is shown grey when *out is empty.
func (f *Form) Text(out *string, title, placeholder string, validate func(string) error) *Form {
	field := huh.NewInput().
		Title(title).
		Placeholder(placeholder).
		Value(out)
	if validate != nil {
		field = field.Validate(validate)
	}
	f.fields = append(f.fields, field)
	return f
}

// Select adds a single-pick menu bound to *out. options use the same
// Option[string] shape as the standalone Select helper.
func (f *Form) Select(out *string, title string, options []Option[string]) *Form {
	huhOpts := make([]huh.Option[string], 0, len(options))
	for _, opt := range options {
		label := opt.Label
		if opt.Description != "" {
			label = opt.Label + " — " + opt.Description
		}
		huhOpts = append(huhOpts, huh.NewOption(label, opt.Value))
	}
	field := huh.NewSelect[string]().
		Title(title).
		Options(huhOpts...).
		Value(out)
	f.fields = append(f.fields, field)
	return f
}

// Confirm adds a yes/no field bound to *out.
func (f *Form) Confirm(out *bool, title string) *Form {
	field := huh.NewConfirm().Title(title).Value(out)
	f.fields = append(f.fields, field)
	return f
}

// Run renders the entire form. Returns mapped errors (PROMPT_CANCELLED
// on Ctrl-C; ONE_CLI_ERROR otherwise). After Run returns nil, the *out
// pointers passed earlier hold the user's answers.
func (f *Form) Run() error {
	if len(f.fields) == 0 {
		return nil
	}
	form := huh.NewForm(huh.NewGroup(f.fields...)).WithTheme(defaultTheme())
	if err := form.Run(); err != nil {
		return mapErr(err)
	}
	return nil
}
