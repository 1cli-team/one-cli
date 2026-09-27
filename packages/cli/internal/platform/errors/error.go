package errors

import (
	"fmt"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

// New constructs an *output.Error pre-populated with the registered default
// remediation for the given code. Callers can extend it with WithContext /
// WithRemediation; the defaults are not destructive — they're only suggestions.
func New(code Code, message string) *output.Error {
	def := Codes[code]
	err := output.NewError(string(code), message)
	if len(def.Remediation) > 0 {
		steps := append([]output.Remediation(nil), def.Remediation...)
		for i := range steps {
			if steps[i].Hint != "" {
				key := fmt.Sprintf("error.%s.hint.%d", code, i)
				if hint := i18n.T(key); hint != key {
					steps[i].Hint = hint
				}
			}
		}
		err = err.WithRemediation(steps...)
	}
	return err
}
