package runcmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	remote "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	process "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

func runGlobal(ctx context.Context, f *runFlags, args []string) error {
	if f.project != "" || f.envProvider != "" {
		return fmt.Errorf("--global 不能与项目或 --env-provider 同时使用")
	}
	if f.envName == "" || f.globalPath == "" {
		return fmt.Errorf("使用全局凭据必须显式指定 --env 和 --path")
	}
	folder, e := remote.ValidateGlobalPath(f.globalPath)
	if e != nil {
		return e
	}
	cwd, e := os.Getwd()
	if e != nil {
		return e
	}
	// Do not prepend repository executables or resolve a command using injected PATH.
	env := globalCommandEnv(os.Environ())
	binary, e := lookPathFor(args[0], env)
	if e != nil {
		return fmt.Errorf("找不到命令 %s；请使用已安装的工具或显式指定可执行文件路径", args[0])
	}
	binary, e = filepath.Abs(binary)
	if e != nil {
		return e
	}
	if f.dryRun {
		output.Emit(map[string]any{"scope": "global", "environment": f.envName, "path": folder, "keys": f.globalKeys, "directory": cwd, "executable": binary, "argv": args})
		return nil
	}
	vars, e := remote.GlobalValues(ctx, f.envName, folder, f.globalKeys)
	if e != nil {
		return e
	}
	for key := range vars {
		if reservedGlobalKey(key) {
			return fmt.Errorf("全局目录包含进程控制变量 %s，拒绝注入；请通过 --keys 选择业务凭据", key)
		}
	}
	child := process.Command(binary, args[1:]...)
	child.Dir = cwd
	child.Env = secrets.MergeIntoEnviron(env, vars, true)
	child.Stdin = os.Stdin
	out := newSecretWriter(os.Stdout, vars)
	errOut := newSecretWriter(os.Stderr, vars)
	child.Stdout = out
	child.Stderr = errOut
	err := process.RunForwarded(ctx, child)
	flushOut := out.Close()
	flushErr := errOut.Close()
	if err != nil {
		return err
	}
	if flushOut != nil {
		return flushOut
	}
	return flushErr
}
func reservedGlobalKey(key string) bool {
	key = strings.ToUpper(key)
	switch key {
	case "PATH", "PATHEXT", "HOME", "USERPROFILE", "BASH_ENV", "ENV", "SHELLOPTS", "BASHOPTS", "NODE_OPTIONS", "NODE_PATH", "PYTHONPATH", "PYTHONHOME", "RUBYOPT", "PERL5OPT", "GIT_CONFIG", "GIT_CONFIG_COUNT", "GIT_SSH_COMMAND":
		return true
	}
	return strings.HasPrefix(key, "LD_") || strings.HasPrefix(key, "DYLD_")
}
func globalCommandEnv(env []string) []string {
	result := make([]string, 0, len(env))
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if strings.EqualFold(key, "PATH") {
			dirs := []string{}
			for _, dir := range filepath.SplitList(value) {
				if !filepath.IsAbs(dir) {
					continue
				}
				if strings.Contains(filepath.ToSlash(dir), "/node_modules/.bin") {
					continue
				}
				dirs = append(dirs, dir)
			}
			entry = key + "=" + strings.Join(dirs, string(os.PathListSeparator))
		}
		result = append(result, entry)
	}
	return result
}

// Exact-value output masking is best effort, not an exfiltration boundary.
// Keep enough bytes to redact secrets split across arbitrary Write calls.
type secretWriter struct {
	mu      sync.Mutex
	out     io.Writer
	pending []byte
	values  []string
	longest int
}

func newSecretWriter(out io.Writer, vars map[string]string) *secretWriter {
	w := &secretWriter{out: out, longest: 1}
	for _, v := range vars {
		if v != "" {
			w.values = append(w.values, v)
			if len(v) > w.longest {
				w.longest = len(v)
			}
		}
	}
	sort.Slice(w.values, func(i, j int) bool { return len(w.values[i]) > len(w.values[j]) })
	return w
}
func (w *secretWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending = append(w.pending, p...)
	return len(p), w.drain(false)
}
func (w *secretWriter) Close() error { w.mu.Lock(); defer w.mu.Unlock(); return w.drain(true) }
func (w *secretWriter) drain(final bool) error {
	var result strings.Builder
	for len(w.pending) > 0 && (final || len(w.pending) >= w.longest) {
		matched := false
		for _, v := range w.values {
			if strings.HasPrefix(string(w.pending), v) {
				result.WriteString("[REDACTED]")
				w.pending = w.pending[len(v):]
				matched = true
				break
			}
		}
		if !matched {
			result.WriteByte(w.pending[0])
			w.pending = w.pending[1:]
		}
	}
	if result.Len() == 0 {
		return nil
	}
	_, e := io.WriteString(w.out, result.String())
	return e
}
