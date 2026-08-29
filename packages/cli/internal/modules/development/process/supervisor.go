package processorch

// supervisor.go holds the platform-agnostic pieces of the built-in
// process supervisor: public types and prefix-formatting helpers. The
// actual exec / pgid / signal logic lives in supervisor_unix.go (real
// implementation) and supervisor_other.go (stub) behind build tags
// because pgid handling and SIGTERM forwarding are Unix-specific.

import (
	"io"
	"strings"
	"time"
)

// ProcEntry is one workload to run under the supervisor. Callers
// (ops.go) build the slice directly from the manifest — there is no
// longer any text-file parsing step.
type ProcEntry struct {
	// Name is the workload identifier ("api", "web", ...). Sourced
	// from manifest.projects[].name.
	Name string
	// Cmd is the full command line, executed via `sh -c <Cmd>`. ops.go
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
