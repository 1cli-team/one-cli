package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"gopkg.in/yaml.v3"
)

// TTYRenderer is implemented by result payloads that want a
// human-friendly rendering when stdout is a terminal. Emit dispatches
// to RenderTTY in TTY mode; payloads that don't implement it produce
// no TTY output (silent — back-compat with pre-rendering era).
//
// Implementations should write idiomatic short-form output: tabular
// for lists (text/tabwriter is enough), key-value for single objects.
// Don't include ANSI colours — keep stdlib-only and Windows-friendly.
type TTYRenderer interface {
	RenderTTY(w io.Writer)
}

// Emit writes a result payload to stdout in the active output mode.
//
// JSON mode → indented JSON (2-space). YAML mode → YAML. TTY mode →
// calls RenderTTY if the payload implements TTYRenderer, else silent.
//
// The payload MUST already include `schema: "one-cli/<cmd>/v1"`. We don't
// inject it because the schema is part of the type identity, not a runtime
// decoration.
func Emit(payload any) {
	switch resolve() {
	case resolvedJSON:
		emitJSON(os.Stdout, payload)
	case resolvedYAML:
		emitYAML(os.Stdout, payload)
	default:
		if r, ok := payload.(TTYRenderer); ok {
			r.RenderTTY(os.Stdout)
		}
	}
}

// EmitError writes the error envelope on stderr. Structured modes (JSON / YAML)
// emit the corresponding format; TTY mode emits a short human line (in
// addition, callers usually print their own clack `outro` style failure
// message).
func EmitError(err *Error) {
	emitErrorTo(os.Stderr, err)
}

func emitErrorTo(w io.Writer, err *Error) {
	// Cooperative cancellation is a valid user choice. Keep the stable error
	// code internally for control flow, but do not paint it as a failure or
	// emit a machine error envelope.
	if err == nil || err.Exit0 {
		return
	}
	switch resolve() {
	case resolvedJSON:
		emitJSON(w, err.envelope())
	case resolvedYAML:
		emitYAML(w, err.envelope())
	default:
		message := err.Error()
		if translated := i18n.T("error." + err.Code + ".message"); translated != "error."+err.Code+".message" {
			message = translated
		}
		fmt.Fprintf(w, "✗ %s\n", message)
		fmt.Fprintf(w, "  %s%s\n", i18n.T("error.code_label"), err.Code)
		printedHeader := false
		for index, step := range err.Remediation {
			if step.Hint == "" && step.Command == "" {
				continue
			}
			if !printedHeader {
				fmt.Fprintln(w)
				fmt.Fprintln(w, i18n.T("error.try_label"))
				printedHeader = true
			}
			if step.Hint != "" {
				hint := step.Hint
				key := fmt.Sprintf("error.%s.hint.%d", err.Code, index)
				if translated := i18n.T(key); translated != key {
					hint = translated
				}
				fmt.Fprintf(w, "  %s\n", hint)
			}
			if step.Command != "" {
				fmt.Fprintf(w, "    %s\n", step.Command)
			}
		}
	}
}

// emitTo is kept for tests that exercise the JSON path directly.
func emitTo(w io.Writer, v any) { emitJSON(w, v) }

func emitJSON(w io.Writer, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		emitFallback(w, err)
		return
	}
	_, _ = w.Write(b)
	_, _ = w.Write([]byte{'\n'})
}

// emitYAML round-trips through JSON so the existing `json:"..."` field tags
// drive the wire keys (yaml.v3 doesn't honour json tags). Every payload type
// in this package is JSON-clean by construction, so the round-trip is safe.
func emitYAML(w io.Writer, v any) {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		emitFallback(w, err)
		return
	}
	var generic any
	if err := json.Unmarshal(jsonBytes, &generic); err != nil {
		emitFallback(w, err)
		return
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(generic); err != nil {
		emitFallback(w, err)
		return
	}
	_ = enc.Close()
	_, _ = w.Write(buf.Bytes())
}

// emitFallback writes a synthetic JSON error envelope so the caller never
// sees a silent drop. This branch should never fire in practice — every
// payload type in this package is JSON-clean by construction.
func emitFallback(w io.Writer, err error) {
	fmt.Fprintf(w, `{"schema":"one-cli/error/v1","error":{"code":"OUTPUT_MARSHAL_FAILED","message":%q,"context":{},"remediation":[]}}`+"\n", err.Error())
}
