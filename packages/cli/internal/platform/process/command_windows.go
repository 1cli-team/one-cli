//go:build windows

package process

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func commandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	resolved := name
	if path, err := exec.LookPath(name); err == nil {
		resolved = path
	}
	ext := strings.ToLower(filepath.Ext(resolved))
	if ext != ".cmd" && ext != ".bat" {
		return exec.CommandContext(ctx, name, args...)
	}
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = "cmd.exe"
	}
	line := quoteCmdToken(resolved)
	for _, arg := range args {
		line += " " + quoteCmdToken(arg)
	}
	cmd := exec.CommandContext(ctx, comspec)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: quoteCmdToken(comspec) + ` /d /s /c "` + line + `"`,
	}
	return cmd
}

func quoteCmdToken(value string) string {
	// Double quotes inside a quoted cmd.exe token represent a literal quote.
	// Keeping every token quoted also prevents &, |, <, >, and spaces from
	// becoming command separators.
	value = strings.NewReplacer(
		"^", "^^",
		"&", "^&",
		"|", "^|",
		"<", "^<",
		">", "^>",
		"(", "^(",
		")", "^)",
	).Replace(value)
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
