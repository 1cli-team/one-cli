// Package taskrun runs independent terminal sessions with a shared scheduler.
// It has no knowledge of manifests, secrets, or runtime providers.
package taskrun

import (
	"fmt"
	"os"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

type Mode string

const (
	Auto   Mode = "auto"
	Raw    Mode = "raw"
	TUI    Mode = "tui"
	Stream Mode = "stream"
)

func ResolveMode(value string, count int) (Mode, error) {
	mode := Mode(value)
	if mode == "" {
		mode = Auto
	}
	interactive := output.CanPrompt() && os.Getenv("CI") == "" && os.Getenv("TERM") != "dumb"
	switch mode {
	case Auto:
		if !interactive {
			return Stream, nil
		}
		if count == 1 {
			return Raw, nil
		}
		if supportsPTY {
			return TUI, nil
		}
		fmt.Fprintln(os.Stderr, "Interactive task terminals are unavailable on this platform; using streaming output.")
		return Stream, nil
	case Raw:
		if count != 1 {
			return "", fmt.Errorf("--ui=raw requires exactly one task")
		}
		if !interactive {
			return "", fmt.Errorf("--ui=raw requires an interactive terminal and text output")
		}
	case TUI:
		if !interactive || !supportsPTY {
			return "", fmt.Errorf("--ui=tui requires a supported interactive terminal and text output")
		}
	case Stream:
	default:
		return "", fmt.Errorf("unknown UI %q; use auto, raw, tui, or stream", value)
	}
	return mode, nil
}
