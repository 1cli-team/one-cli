package execcmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	remote "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	process "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

func runGlobal(ctx context.Context, f *runFlags, args []string) error {
	if f.project != "" {
		return i18n.Errorf("exec.global_conflict")
	}
	if f.envName == "" || f.globalPath == "" {
		return i18n.Errorf("global.scope_required")
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
		return i18n.Errorf("exec.global_command_missing", args[0])
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
			return i18n.Errorf("exec.control_variable", key)
		}
	}
	child := process.Command(binary, args[1:]...)
	child.Dir = cwd
	child.Env = secrets.MergeIntoEnviron(env, vars, true)
	child.Stdin = os.Stdin
	child.Stdout, child.Stderr = os.Stdout, os.Stderr
	return process.RunForwarded(ctx, child)
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
