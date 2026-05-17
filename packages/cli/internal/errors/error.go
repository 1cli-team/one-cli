package errors

import (
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
)

// New constructs an *output.Error pre-populated with the registered default
// remediation for the given code. Callers can extend it with WithContext /
// WithRemediation; the defaults are not destructive — they're only suggestions.
func New(code Code, message string) *output.Error {
	def := Codes[code]
	err := output.NewError(string(code), message)
	if len(def.Remediation) > 0 {
		err = err.WithRemediation(def.Remediation...)
	}
	return err
}
