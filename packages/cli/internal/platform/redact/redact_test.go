package redact

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

func TestMasksSecretsAcrossWrites(t *testing.T) {
	vars := map[string]string{"A": "very-secret", "B": "short", "C": "very-secret-long", "MULTI": "line-one\nline-two", "UTF8": "机密令牌", "EMPTY": ""}
	input := "\x1b[32mbefore very-secret-long and short\x1b[0m\nline-one\nline-two / line-one\\nline-two / 机密令牌 / very-secret"
	want := "\x1b[32mbefore [REDACTED] and [REDACTED]\x1b[0m\n[REDACTED] / [REDACTED] / [REDACTED] / [REDACTED]"
	for size := 1; size <= len(input); size++ {
		var out bytes.Buffer
		writer, flush := New(vars).Writer(&out)
		for i := 0; i < len(input); i += size {
			if _, err := io.WriteString(writer, input[i:min(i+size, len(input))]); err != nil {
				t.Fatal(err)
			}
		}
		if err := flush(); err != nil {
			t.Fatal(err)
		}
		if out.String() != want {
			t.Fatalf("chunk size %d: %q", size, out.String())
		}
	}
}
func TestUnrelatedOutputIsImmediateAndEmptySecretsPreserveWriter(t *testing.T) {
	var out bytes.Buffer
	w, flush := New(map[string]string{"TOKEN": "secret-token"}).Writer(&out)
	_, _ = io.WriteString(w, "Enter choice: ")
	if out.String() != "Enter choice: " {
		t.Fatal("held an unrelated interactive prompt")
	}
	_, _ = io.WriteString(w, "sec")
	if out.String() != "Enter choice: " {
		t.Fatal("released a possible secret prefix")
	}
	_, _ = io.WriteString(w, "ret-token")
	if err := flush(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "Enter choice: [REDACTED]" {
		t.Fatal(out.String())
	}
	plain, _ := New(map[string]string{"EMPTY": ""}).Writer(&out)
	if plain != &out {
		t.Fatal("wrapped a stream without secrets")
	}
}
func TestErrorKeepsCodesAndChainsWithoutLeakingStructuredDetails(t *testing.T) {
	original := output.NewError("EXAMPLE", "failed token-value").WithContext(map[string]any{"argv": []string{"tool", "token-value"}, "nested": map[string]any{"token-value": "token-value"}, "count": 2}).WithRemediation(output.Remediation{Hint: "check token-value", Command: "tool token-value"})
	err := New(map[string]string{"TOKEN": "token-value"}).Error(fmt.Errorf("context: %w", original))
	var structured *output.Error
	if !errors.Is(err, original) || !errors.As(err, &structured) || structured.Code != "EXAMPLE" {
		t.Fatal("error identity lost")
	}
	if strings.Contains(fmt.Sprintf("%s %+v %+v", err, structured.Context, structured.Remediation), "token-value") {
		t.Fatal("secret survived redaction")
	}
	if original.Message != "failed token-value" {
		t.Fatal("mutated original error")
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }
func TestWriterPreservesOutputFailures(t *testing.T) {
	failure := errors.New("broken pipe")
	w, flush := New(map[string]string{"TOKEN": "secret"}).Writer(failingWriter{failure})
	if _, err := io.WriteString(w, "secret"); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if err := flush(); !errors.Is(err, failure) {
		t.Fatal(err)
	}
}
