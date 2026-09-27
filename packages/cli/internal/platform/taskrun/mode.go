// Package taskrun runs independent terminal sessions with a shared scheduler.
// It has no knowledge of manifests, secrets, or runtime providers.
package taskrun

import (
	"fmt"
	"os"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
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
		fmt.Fprintln(os.Stderr, i18n.T("task.stream_fallback"))
		return Stream, nil
	case Raw:
		if count != 1 {
			return "", i18n.Errorf("task.raw_single")
		}
		if !interactive {
			return "", i18n.Errorf("task.raw_terminal_required")
		}
	case TUI:
		if !interactive || !supportsPTY {
			return "", i18n.Errorf("task.tui_terminal_required")
		}
	case Stream:
	default:
		return "", i18n.Errorf("task.ui_unknown", value)
	}
	return mode, nil
}
