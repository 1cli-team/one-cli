// Package redact removes known secret values from process output and errors.
// It is an accidental-disclosure guard, not a sandbox for untrusted programs.
package redact

import (
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

const Marker = "[REDACTED]"

type Filter struct {
	values  []string
	writeMu sync.Mutex
}

func New(vars map[string]string) *Filter {
	unique := map[string]bool{}
	for _, value := range vars {
		if value == "" {
			continue
		}
		unique[value] = true
		// JSON-escaped multiline credentials are common in structured application logs.
		encoded, _ := json.Marshal(value)
		unique[string(encoded[1:len(encoded)-1])] = true
	}
	f := &Filter{}
	for value := range unique {
		f.values = append(f.values, value)
	}
	sort.Slice(f.values, func(i, j int) bool { return len(f.values[i]) > len(f.values[j]) })
	return f
}

// Writer leaves streams untouched when no secrets were injected, preserving TTYs.
// Flush must be called once the child has stopped writing.
func (f *Filter) Writer(out io.Writer) (io.Writer, func() error) {
	if len(f.values) == 0 || out == nil {
		return out, func() error { return nil }
	}
	w := &writer{out: out, filter: f}
	return w, w.flush
}
func (f *Filter) Text(text string) string {
	var out strings.Builder
	w, flush := f.Writer(&out)
	_, _ = io.WriteString(w, text)
	_ = flush()
	return out.String()
}

type writer struct {
	mu      sync.Mutex
	out     io.Writer
	filter  *Filter
	pending []byte
	failure error
}

func (w *writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.failure != nil {
		return 0, w.failure
	}
	w.pending = append(w.pending, p...)
	w.failure = w.drain(false)
	return len(p), w.failure
}
func (w *writer) flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.failure != nil {
		return w.failure
	}
	w.failure = w.drain(true)
	return w.failure
}
func (w *writer) drain(final bool) error {
	var out strings.Builder
	offset := 0
	buffer := string(w.pending)
scan:
	for offset < len(w.pending) {
		pending := buffer[offset:]
		for _, value := range w.filter.values {
			if !final && len(pending) < len(value) && strings.HasPrefix(value, pending) {
				break scan
			}
			if strings.HasPrefix(pending, value) {
				out.WriteString(Marker)
				offset += len(value)
				continue scan
			}
		}
		out.WriteByte(w.pending[offset])
		offset++
	}
	w.pending = w.pending[offset:]
	if out.Len() == 0 {
		return nil
	}
	w.filter.writeMu.Lock()
	defer w.filter.writeMu.Unlock()
	n, err := io.WriteString(w.out, out.String())
	if err == nil && n != out.Len() {
		err = io.ErrShortWrite
	}
	return err
}

// Error preserves the original chain and sanitizes structured CLI diagnostics too.
func (f *Filter) Error(err error) error {
	if err == nil || len(f.values) == 0 {
		return err
	}
	e := &filteredError{message: f.Text(err.Error()), cause: err}
	var original *output.Error
	if errors.As(err, &original) {
		safe := *original
		safe.Message = f.Text(safe.Message)
		if original.Context != nil {
			safe.Context = f.context(original.Context).(map[string]any)
		}
		safe.Remediation = append([]output.Remediation(nil), original.Remediation...)
		for i := range safe.Remediation {
			safe.Remediation[i].Hint = f.Text(safe.Remediation[i].Hint)
			safe.Remediation[i].Command = f.Text(safe.Remediation[i].Command)
		}
		e.structured = &safe
	}
	return e
}
func (f *Filter) context(value any) any {
	switch v := value.(type) {
	case string:
		return f.Text(v)
	case map[string]any:
		safe := make(map[string]any, len(v))
		for key, item := range v {
			safe[f.Text(key)] = f.context(item)
		}
		return safe
	case map[string]string:
		safe := make(map[string]string, len(v))
		for key, item := range v {
			safe[f.Text(key)] = f.Text(item)
		}
		return safe
	case []any:
		safe := make([]any, len(v))
		for i, item := range v {
			safe[i] = f.context(item)
		}
		return safe
	case []string:
		safe := make([]string, len(v))
		for i, item := range v {
			safe[i] = f.Text(item)
		}
		return safe
	default:
		return value
	}
}

type filteredError struct {
	message    string
	cause      error
	structured *output.Error
}

func (e *filteredError) Error() string { return e.message }
func (e *filteredError) Unwrap() error { return e.cause }
func (e *filteredError) As(target any) bool {
	if p, ok := target.(**output.Error); ok && e.structured != nil {
		*p = e.structured
		return true
	}
	return false
}
