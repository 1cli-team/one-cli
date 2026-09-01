package processorch

// supervisor.go holds the platform-agnostic pieces of the built-in
// process supervisor: public types and prefix-formatting helpers. The
// actual exec / pgid / signal logic lives in supervisor_unix.go (real
// implementation) and supervisor_other.go (stub) behind build tags
// because pgid handling and SIGTERM forwarding are Unix-specific.

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

// ProcEntry is one workload to run under the supervisor. Callers
// (ops.go) build the slice directly from the manifest — there is no
// longer any text-file parsing step.
type ProcEntry struct {
	// Name is the workload identifier ("api", "web", ...). Sourced
	// from manifest.projects[].name.
	Name string
	// Cmd is the full command line, executed by the platform shell. ops.go
	// wraps the manifest's dev.command with `one run -p <reldir> -- ...`
	// so per-project secrets injection still runs.
	Cmd string
}

// BuiltinOpts tunes the built-in supervisor.
type BuiltinOpts struct {
	// Out is where prefixed child output goes. Typically os.Stdout. A
	// single mutex serialises writes so lines from concurrent children
	// never tear.
	Out io.Writer
	// GracePeriod is the SIGTERM → SIGKILL window for each child on
	// shutdown. Zero defaults to 5 seconds.
	GracePeriod time.Duration
}

// padName right-pads name with spaces so prefixed output aligns
// across workloads with different name lengths.
func padName(name string, width int) string {
	if len(name) >= width {
		return name
	}
	return name + strings.Repeat(" ", width-len(name))
}

// maxNameLen returns the longest entry name in the slice. Empty slice
// returns 0.
func maxNameLen(entries []ProcEntry) int {
	n := 0
	for _, e := range entries {
		if len(e.Name) > n {
			n = len(e.Name)
		}
	}
	return n
}

// prefixPalette rotates ANSI foreground colors to visually distinguish
// concurrent workloads in TTY output. Indexed modulo length so any
// number of workloads gets a stable color assignment by position.
var prefixPalette = []string{
	"\x1b[36m", // cyan
	"\x1b[35m", // magenta
	"\x1b[33m", // yellow
	"\x1b[32m", // green
	"\x1b[34m", // blue
	"\x1b[31m", // red
}

// defaultGracePeriod is the graceful-stop window when callers don't
// override it. Unix uses it between SIGTERM and SIGKILL. Windows process
// trees are terminated atomically through a Job Object.
const defaultGracePeriod = 5 * time.Second

// isStdoutTTY returns true when stdout is a real terminal AND the active
// output mode is human-friendly (not JSON/YAML).
func isStdoutTTY() bool { return output.IsTTY() }

// ansiReset closes a color span.
const ansiReset = "\x1b[0m"

// decoratePrefix returns the workload name wrapped in an ANSI color
// when colored is true; otherwise the bare padded name. Index decides
// which color in the palette is used (modulo).
func decoratePrefix(padded string, idx int, colored bool) string {
	if !colored {
		return padded
	}
	return prefixPalette[idx%len(prefixPalette)] + padded + ansiReset
}

// prefixLineWriter collects arbitrary Write chunks into log lines, prefixes
// each complete line, and flushes the final unterminated line after Wait.
// It avoids bufio.Scanner's token limit and lets exec.Cmd own pipe draining.
type prefixLineWriter struct {
	prefix    string
	writeLine func(string, string)
	pending   []byte
}

func newPrefixLineWriter(prefix string, writeLine func(string, string)) *prefixLineWriter {
	return &prefixLineWriter{prefix: prefix, writeLine: writeLine}
}

func (w *prefixLineWriter) Write(p []byte) (int, error) {
	start := 0
	for i, b := range p {
		if b != '\n' {
			continue
		}
		w.pending = append(w.pending, p[start:i]...)
		w.emit()
		start = i + 1
	}
	if start < len(p) {
		w.pending = append(w.pending, p[start:]...)
	}
	return len(p), nil
}

func (w *prefixLineWriter) Flush() {
	if len(w.pending) == 0 {
		return
	}
	w.emit()
}

func (w *prefixLineWriter) emit() {
	line := strings.TrimSuffix(string(w.pending), "\r")
	w.writeLine(w.prefix, line)
	w.pending = w.pending[:0]
}

func writePrefixedLine(mu interface {
	Lock()
	Unlock()
}, out io.Writer, prefix, line string) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Fprintf(out, "%s | %s\n", prefix, line)
}

func normalizeBuiltinOpts(opts *BuiltinOpts) {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.GracePeriod <= 0 {
		opts.GracePeriod = defaultGracePeriod
	}
}
