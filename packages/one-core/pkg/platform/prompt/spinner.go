package prompt

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/terminalui"
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
	return SpinContext(context.Background(), title, func(context.Context) error { return action() })
}

type spinnerContextKey struct{}

type spinnerStatus struct {
	mu    sync.Mutex
	title string
}

// SpinContext attaches a progress sink to this action's context, so its phase
// updates replace the spinner title instead of interleaving terminal output.
func SpinContext(ctx context.Context, title string, action func(context.Context) error) error {
	if !output.IsTTY() {
		return action(ctx)
	}
	if !term.IsTerminal(int(os.Stderr.Fd())) || os.Getenv("TERM") == "dumb" {
		fmt.Fprintln(os.Stderr, "  "+title)
		return action(ctx)
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	status := &spinnerStatus{title: title}
	ctx = context.WithValue(ctx, spinnerContextKey{}, status)

	go renderSpinner(os.Stderr, status, stop, done)
	defer func() {
		close(stop)
		<-done
	}()
	return action(ctx)
}

// ReportProgress updates the active action's spinner, or writes a normal status
// line when no spinner is attached (structured CLI output or Dashboard calls).
func ReportProgress(ctx context.Context, message string) {
	if status, ok := ctx.Value(spinnerContextKey{}).(*spinnerStatus); ok {
		status.mu.Lock()
		status.title = message
		status.mu.Unlock()
		return
	}
	fmt.Fprintln(os.Stderr, message)
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
func renderSpinner(w io.Writer, status *spinnerStatus, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)

	styles := terminalui.NewStyles()

	t := time.NewTicker(frameInterval)
	defer t.Stop()

	idx := 0
	// Print initial frame so the spinner appears immediately.
	render := func() {
		status.mu.Lock()
		title := status.title
		status.mu.Unlock()
		// Clear the previous title, including when a new phase is shorter or
		// contains wide characters, before drawing the next frame.
		line := fmt.Sprintf("  %s %s ", styles.Accent.Render(frames[idx]), title)
		if terminal, ok := w.(interface{ Fd() uintptr }); ok {
			if columns, _, err := term.GetSize(int(terminal.Fd())); err == nil && columns > 0 {
				// Leave the last column free to avoid terminal auto-wrap. ANSI
				// truncation preserves styles and handles Chinese display width.
				line = ansi.Truncate(line, columns-1, "…")
			}
		}
		lipgloss.Fprintf(w, "\r\033[2K%s", line)
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
	terminalui.New(os.Stderr).Success(message)
}

// Warning displays a non-fatal notice without marking it as a completed step.
func Warning(message string) {
	if output.IsTTY() {
		terminalui.New(os.Stderr).Warning(message)
	}
}
