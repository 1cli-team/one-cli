// Package output centralises every result payload (one-cli/<cmd>/v1) and
// error envelope (one-cli/error/v1) the CLI emits. All structured output
// MUST flow through Emit/EmitError so the wire shape stays uniform across
// commands. See testdata/reference/ for the snapshot fixtures that lock
// the structural contract; comparison is structural (json.Unmarshal +
// reflect.DeepEqual) so whitespace and key ordering are flexible.
package output

import (
	"os"

	"golang.org/x/term"
)

// Mode controls whether the CLI renders human-friendly TTY UI (intro, spinner,
// coloured logs) or a structured machine format (JSON or YAML) on every emit.
type Mode int

const (
	// ModeAuto defers to runtime TTY detection (TTY → human, pipe → JSON).
	ModeAuto Mode = iota
	// ModeJSON forces JSON output regardless of TTY status.
	ModeJSON
	// ModeTTY forces human-friendly output regardless of TTY status.
	ModeTTY
	// ModeYAML forces YAML output regardless of TTY status.
	ModeYAML
)

var globalMode = ModeAuto

// SetMode overrides auto-detection. Should be called once during startup,
// before any subcommand runs (after parsing -o / --output).
func SetMode(m Mode) { globalMode = m }

// resolved is the mode after auto-detection collapses to a concrete format.
type resolved int

const (
	resolvedTTY resolved = iota
	resolvedJSON
	resolvedYAML
)

func resolve() resolved {
	switch globalMode {
	case ModeJSON:
		return resolvedJSON
	case ModeYAML:
		return resolvedYAML
	case ModeTTY:
		return resolvedTTY
	default: // ModeAuto: pipe → JSON, terminal → human. YAML is opt-in only.
		if stdoutIsTTY() {
			return resolvedTTY
		}
		return resolvedJSON
	}
}

// IsJSON reports whether the active output is JSON specifically.
func IsJSON() bool { return resolve() == resolvedJSON }

// IsYAML reports whether the active output is YAML.
func IsYAML() bool { return resolve() == resolvedYAML }

// IsTTY reports whether the active output is human-friendly TTY.
func IsTTY() bool { return resolve() == resolvedTTY }

// CanPrompt reports whether it is safe to open an interactive prompt.
// Forced text output still is not enough: CI and tests can request text
// while stdin/stdout are pipes and /dev/tty is unavailable.
func CanPrompt() bool { return IsTTY() && stdinIsTTY() && stdoutIsTTY() }

// IsStructured reports whether the active output is a machine format
// (JSON or YAML). Use this when deciding between emitting an envelope
// vs a bare scalar shortcut for shell consumption.
func IsStructured() bool { return !IsTTY() }

func stdinIsTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func stdoutIsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}
