// Package toolenv resolves mise's environment without a recursive One process.
package toolenv

import (
	"bytes"
	"context"
	"os"
	"runtime"
	"slices"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	process "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

// Environment captures assignments and removals in memory. mise env --json
// omits removals; pwsh output preserves them and native Windows PATH spelling.
// This is a deliberately narrow data parser: no shell or PowerShell is executed.
func Environment(ctx context.Context, provider runtimeport.Provider, dir string, base []string) ([]string, error) {
	command, err := provider.PrepareCLI(ctx, runtimeport.Command{Directory: dir, Argv: []string{"env", "--shell", "pwsh"}, Env: base})
	if err != nil {
		return nil, err
	}
	child := process.CommandContext(ctx, command.Argv[0], command.Argv[1:]...)
	child.Dir, child.Env = command.Directory, command.Env
	process.CancelProcessTree(child)
	var data bytes.Buffer
	child.Stdout, child.Stderr = &data, os.Stderr
	if err := child.Run(); err != nil {
		return nil, i18n.Errorf("exec.environment_failed", err)
	}
	return parse(data.String(), command.Env)
}

func parse(data string, base []string) ([]string, error) {
	env := slices.Clone(base)
	invalid := func() ([]string, error) { return nil, i18n.Errorf("exec.environment_invalid") }
	for data != "" {
		var key, value string
		remove := false
		switch {
		case strings.HasPrefix(data, "${Env:"):
			data = data[len("${Env:"):]
			var b strings.Builder
			closed := false
			for len(data) > 0 {
				c := data[0]
				data = data[1:]
				if c == '`' {
					if len(data) == 0 {
						return invalid()
					}
					b.WriteByte(data[0])
					data = data[1:]
					continue
				}
				if c == '}' {
					closed = true
					break
				}
				b.WriteByte(c)
			}
			if !closed || !strings.HasPrefix(data, "=") {
				return invalid()
			}
			key = b.String()
			data = data[1:]
		case strings.HasPrefix(data, "Remove-Item -ErrorAction SilentlyContinue -LiteralPath "):
			data = data[len("Remove-Item -ErrorAction SilentlyContinue -LiteralPath "):]
			remove = true
		default:
			return invalid()
		}
		var ok bool
		value, data, ok = quoted(data)
		if !ok {
			return invalid()
		}
		if strings.HasPrefix(data, "\r\n") {
			data = data[2:]
		} else if strings.HasPrefix(data, "\n") {
			data = data[1:]
		} else {
			return invalid()
		}
		if remove {
			if !strings.HasPrefix(value, "Env:/") {
				return invalid()
			}
			key = strings.TrimPrefix(value, "Env:/")
		}
		if key == "" || strings.ContainsAny(key, "=\x00") || strings.ContainsRune(value, 0) {
			return invalid()
		}
		if remove {
			env = slices.DeleteFunc(env, func(entry string) bool {
				k, _, _ := strings.Cut(entry, "=")
				return k == key || runtime.GOOS == "windows" && strings.EqualFold(k, key)
			})
		} else {
			env = secrets.MergeIntoEnviron(env, map[string]string{key: value}, true)
		}
	}
	return env, nil
}

func quoted(s string) (string, string, bool) {
	if !strings.HasPrefix(s, "'") {
		return "", s, false
	}
	s = s[1:]
	var b strings.Builder
	for len(s) > 0 {
		n := strings.IndexByte(s, '\'')
		if n < 0 {
			return "", s, false
		}
		b.WriteString(s[:n])
		s = s[n+1:]
		if !strings.HasPrefix(s, "'") {
			return b.String(), s, true
		}
		b.WriteByte('\'')
		s = s[1:]
	}
	return "", s, false
}
