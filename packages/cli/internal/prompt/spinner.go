package prompt

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
)

// frames is the spinner glyph cycle. Braille dots are visually consistent
// with clack and render fine on every terminal we care about (renders as
// a moving dot circle). 80 ms / frame matches clack's pacing.
var frames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const frameInterval = 80 * time.Millisecond

// Spin runs action while displaying a spinner with the given title.
// On TTY: renders ⠋ → ⠙ → ⠹ ... cycle next to title in the prompt accent
// colour. When action returns: clears the line. On non-TTY (JSON mode
// or piped stdout): no UI — action runs synchronously, error returned
// as-is. Errors from action are returned unwrapped; no PROMPT_CANCELLED
// translation here since spinner doesn't accept user input.
//
// Use this for perceptible-but-bounded operations (template scaffolding,
// network calls). For long streaming operations, prefer a real bubbletea
// program with progress; for sub-100 ms operations, don't wrap — the
// spinner flash is more distracting than the work.
func Spin(title string, action func() error) error {
	if !output.IsTTY() {
		return action()
	}

	stop := make(chan struct{})
	done := make(chan struct{})

	go renderSpinner(os.Stderr, title, stop, done)

	err := action()

	close(stop)
	<-done
	return err
}

// renderSpinner writes spinner frames to w until stop is closed, then
// clears the line and signals done. Writes to stderr so the spinner
// never contaminates stdout (which carries result envelopes).
//
// We render to stderr unconditionally because:
//   - stdout is reserved for the JSON envelope / TTY result table; mixing
//     spinner frames into stdout breaks `... | jq` even in TTY mode
//   - stderr is interactive-by-default — terminals show it inline, but
//     redirected `2>` consumers don't see the carriage-return overwrites
func renderSpinner(w io.Writer, title string, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)

	accent := lipgloss.AdaptiveColor{Light: "#0E7490", Dark: "#22D3EE"}
	frameStyle := lipgloss.NewStyle().Foreground(accent)
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#374151", Dark: "#D1D5DB"})

	t := time.NewTicker(frameInterval)
	defer t.Stop()

	idx := 0
	// Print initial frame so the spinner appears immediately.
	render := func() {
		fmt.Fprintf(w, "\r%s %s ", frameStyle.Render(frames[idx]), titleStyle.Render(title))
		idx = (idx + 1) % len(frames)
	}
	render()

	for {
		select {
		case <-stop:
			// Erase the spinner line so the next output starts cleanly.
			// "\r\033[2K" = carriage return + ANSI "erase entire line".
			fmt.Fprint(w, "\r\033[2K")
			return
		case <-t.C:
			render()
		}
	}
}

// Step prints a single completed step line without occupying a spinner
// slot — for static checkpoints inside a longer flow ("✓ template
// downloaded"). Writes to stderr for the same reason as the spinner.
func Step(message string) {
	if !output.IsTTY() {
		return
	}
	check := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#16A34A", Dark: "#4ADE80"}).
		SetString("✓")
	fmt.Fprintf(os.Stderr, "%s %s\n", check, message)
}
