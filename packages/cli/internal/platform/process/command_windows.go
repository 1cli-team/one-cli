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
	if strings.EqualFold(filepath.Base(resolved), "cmd.exe") && len(args) >= 2 {
		switch strings.ToLower(args[len(args)-2]) {
		case "/c", "/k":
			// cmd.exe parses the final argument as shell source, rather than
			// CommandLineToArgvW argv. Go's default escaping turns quotes in
			// that source into \" and breaks quoted executable paths. /s strips
			// the outer quote pair; preserve every quote inside the command.
			cmd := exec.CommandContext(ctx, name)
			line := syscall.EscapeArg(cmd.Path)
			for _, arg := range args[:len(args)-1] {
				line += " " + syscall.EscapeArg(arg)
			}
			cmd.SysProcAttr = &syscall.SysProcAttr{
				CmdLine: line + ` "` + args[len(args)-1] + `"`,
			}
			return cmd
		}
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
