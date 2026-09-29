package taskui

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

type clipboardWriter func(context.Context, string) (remote bool, err error)

func systemClipboard(ctx context.Context, text string) (bool, error) {
	remote := os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_TTY") != ""
	var args []string
	if !remote {
		switch runtime.GOOS {
		case "darwin":
			args = []string{"pbcopy"}
		case "windows":
			args = []string{"powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "[Console]::InputEncoding = [System.Text.Encoding]::UTF8; Set-Clipboard -Value ([Console]::In.ReadToEnd())"}
		case "linux":
			if os.Getenv("WAYLAND_DISPLAY") != "" {
				if _, err := exec.LookPath("wl-copy"); err == nil {
					args = []string{"wl-copy"}
				}
			}
			if len(args) == 0 && os.Getenv("DISPLAY") != "" {
				args = []string{"xclip", "-selection", "clipboard"}
			}
		}
	}
	if len(args) > 0 {
		if _, err := exec.LookPath(args[0]); err == nil {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, args[0], args[1:]...)
			cmd.Stdin = strings.NewReader(text)
			cmd.WaitDelay = time.Second
			return false, cmd.Run()
		}
	}
	// Terminals impose different OSC 52 limits. Refuse a large request rather
	// than silently putting a truncated log into the user's clipboard.
	if len(text) > 100*1024 {
		return true, i18n.Errorf("tasks.ui.copy_limit")
	}
	return true, nil
}
