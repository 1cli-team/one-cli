// Package execcmd executes commands with optional Infisical injection.
package execcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/toolenv"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	platformprocess "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

func Commands(loaders *secrets.Registry, provider runtimeport.Provider) []*cobra.Command {
	return []*cobra.Command{newRunCmd(loaders, provider)}
}

type runFlags struct {
	global       bool
	globalPath   string
	globalKeys   []string
	project      string
	envName      string
	runtime      string
	provider     runtimeport.Provider
	dryRun       bool
	outputFormat string
}

func newRunCmd(loaders *secrets.Registry, provider runtimeport.Provider) *cobra.Command {
	flags := &runFlags{provider: provider}
	cmd := &cobra.Command{
		Use:                   "exec [project] [-p <name|path>] [--env <name>] -- <cmd> [args...]",
		DisableFlagsInUseLine: true,
		Long:                  i18n.T("exec.tip"),
		Args:                  cobra.ArbitraryArgs,
		DisableFlagParsing:    false,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags.outputFormat, _ = cmd.Flags().GetString("output")
			commandArgs, err := parseRunArgs(cmd, flags, args)
			if err != nil {
				return err
			}
			if len(commandArgs) == 0 {
				// 用户没传命令视为请求帮助：走 stdout（方便 `one exec | less`），
				// 父命令空参那条路径在 cobra 里默认走 stderr，这里显式纠偏。
				cmd.SetOut(os.Stdout)
				return cmd.Help()
			}
			if flags.global {
				return runGlobal(cmd.Context(), flags, commandArgs)
			}
			if cmd.Flags().Changed("path") || cmd.Flags().Changed("keys") {
				return i18n.Errorf("exec.global_flags_required")
			}
			return runRun(cmd.Context(), loaders, flags, commandArgs)
		},
	}
	i18n.MarkLong(cmd, "exec.tip")
	cmd.Flags().StringVarP(&flags.project, "project", "p", "", i18n.T("exec.flag.project"))
	i18n.MarkFlagUsage(cmd, "project", "exec.flag.project")
	cmd.Flags().StringVar(&flags.envName, "env", "", i18n.T("exec.flag.env"))
	i18n.MarkFlagUsage(cmd, "env", "exec.flag.env")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, i18n.T("exec.flag.dry_run"))
	i18n.MarkFlagUsage(cmd, "dry-run", "exec.flag.dry_run")
	cmd.Flags().BoolVar(&flags.global, "global", false, i18n.T("exec.flag.global"))
	i18n.MarkFlagUsage(cmd, "global", "exec.flag.global")
	cmd.Flags().StringVar(&flags.globalPath, "path", "", i18n.T("exec.flag.path"))
	i18n.MarkFlagUsage(cmd, "path", "exec.flag.path")
	cmd.Flags().StringSliceVar(&flags.globalKeys, "keys", nil, i18n.T("exec.flag.keys"))
	i18n.MarkFlagUsage(cmd, "keys", "exec.flag.keys")
	i18n.MarkShort(cmd, "exec.short")
	return cmd
}

func parseRunArgs(cmd *cobra.Command, flags *runFlags, args []string) ([]string, error) {
	dashIndex := cmd.ArgsLenAtDash()
	if dashIndex < 0 {
		if len(args) == 0 {
			return nil, nil
		}
		return nil, cliErrors.New(cliErrors.RUN_USAGE_INVALID,
			i18n.T("exec.separator_required")).
			WithContext(map[string]any{"reason": "separator-required"})
	}
	if dashIndex > 1 {
		return nil, cliErrors.New(cliErrors.RUN_USAGE_INVALID,
			i18n.T("exec.project_args_invalid")).
			WithContext(map[string]any{
				"reason":        "too-many-projects",
				"project_args":  args[:dashIndex],
				"project_count": dashIndex,
			})
	}
	if dashIndex == 1 {
		positional := args[0]
		if flags.project != "" {
			normalize := func(v string) string {
				v = strings.TrimSpace(v)
				v = strings.TrimSuffix(strings.TrimPrefix(v, "./"), "/")
				return workspace.ToPosixPath(v)
			}
			if normalize(positional) != normalize(flags.project) {
				return nil, cliErrors.New(cliErrors.RUN_USAGE_INVALID,
					i18n.T("exec.selector_conflict")).
					WithContext(map[string]any{
						"reason":             "selector-conflict",
						"positional_project": positional,
						"flag_project":       flags.project,
					})
			}
		}
		flags.project = positional
	}

	commandArgs := args[dashIndex:]
	if len(commandArgs) == 0 {
		return nil, cliErrors.New(cliErrors.RUN_USAGE_INVALID,
			i18n.T("exec.command_required")).
			WithContext(map[string]any{"reason": "command-required"})
	}
	return commandArgs, nil
}

func runRun(ctx context.Context, loaders *secrets.Registry, flags *runFlags, args []string) (resultErr error) {
	ctx, stop := platformprocess.SignalContext(ctx)
	defer stop()
	defer func() {
		if ctx.Err() != nil {
			resultErr = &platformprocess.ExitStatus{Code: platformprocess.ExitCode(context.Cause(ctx))}
		}
	}()
	activeWorkspace, err := execution.ResolveWorkspace(ctx)
	if err != nil {
		return err
	}
	projectRoot := activeWorkspace.Root()

	targetDir, relativeDir, err := resolveRunSubproject(activeWorkspace, flags.project)
	if err != nil {
		return err
	}

	flags.runtime, err = execution.RuntimeKind(projectRoot)
	if err != nil {
		return err
	}
	if err := runtimeport.Validate(flags.runtime); err != nil {
		return err
	}
	if flags.dryRun {
		output.Emit(map[string]any{"schema": "one-cli/run-plan/v1", "runtime": flags.runtime, "directory": targetDir, "argv": args, "environment": flags.envName, "dry_run": true})
		return nil
	}
	childEnv := os.Environ()
	if flags.runtime == runtimeport.Mise {
		if flags.provider == nil {
			return i18n.Errorf("exec.mise_missing")
		}
		childEnv, err = toolenv.Environment(ctx, flags.provider, targetDir, childEnv)
		if err != nil {
			return err
		}
	}

	vars, source, err := loadRunSecrets(
		ctx, loaders, flags, projectRoot, activeWorkspace.Manifest(), relativeDir,
	)
	if err != nil {
		return err
	}
	if output.IsTTY() && source != "" {
		fmt.Fprintf(os.Stderr, i18n.T("exec.injected")+"\n", len(vars), source)
	}

	childEnv = secrets.MergeIntoEnviron(childEnv, vars, true)
	// Always inject node_modules/.bin so commands like `astro` / `next` / `vite`
	// resolve when invoked directly (and so `npm run dev` finds hoisted bins
	// in pnpm/turbo workspaces). Subproject-local first, workspace root next,
	// inherited PATH last — this is the same precedence npm/pnpm use.
	childEnv = augmentPathForRun(childEnv, projectRoot, targetDir)

	commandName := args[0]
	if !filepath.IsAbs(commandName) && strings.ContainsAny(commandName, "/\\") {
		commandName = filepath.Join(targetDir, commandName)
	}
	binary, err := lookPathFor(commandName, childEnv)
	if err != nil {
		return cliErrors.New(cliErrors.RUN_COMMAND_NOT_FOUND,
			i18n.Tf("exec.command_missing",
				args[0], relativeDir, filepath.Base(projectRoot))).
			WithContext(map[string]any{
				"command": args[0],
			})
	}

	child := platformprocess.CommandContext(ctx, binary, args[1:]...)
	child.Cancel = func() error { return platformprocess.StopTree(child.Process) }
	child.WaitDelay = 3 * time.Second
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.Env = childEnv
	child.Dir = targetDir

	err = child.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return &platformprocess.ExitStatus{Code: platformprocess.ExitCode(err)}
	}
	if err != nil {
		return cliErrors.New(cliErrors.RUN_COMMAND_NOT_FOUND, i18n.Tf("exec.start_failed", args[0], err))
	}
	return err
}

// loadRunSecrets fetches Infisical variables only when the project enables injection.
func loadRunSecrets(
	ctx context.Context,
	loaders *secrets.Registry,
	flags *runFlags,
	projectRoot string,
	manifest *workspace.Manifest,
	relativeDir string,
) (map[string]string, string, error) {
	providerID := workspace.EnvBackend(manifest)
	if !workspace.EnvironmentEnabled(manifest, relativeDir) {
		return map[string]string{}, "", nil
	}

	loader := loaders.Find(providerID)
	if loader == nil {
		return nil, "", cliErrors.New(cliErrors.ONE_CLI_ERROR,
			i18n.Tf("exec.provider_unregistered", providerID))
	}

	vars, err := loader.Load(ctx, projectRoot, relativeDir, flags.envName)
	if err != nil {
		return nil, "", err
	}
	return vars, loader.ID(), nil
}

// resolveRunSubproject selects the project for command execution.
//   - explicit -p / --project: pnpm-style selector — first by name, then by
//     relativeDir, using the command's workspace snapshot.
//   - else: figure out which subproject the current cwd is inside via
//     that same snapshot; error out if cwd is at workspace root or somewhere
//     outside any subproject.
func resolveRunSubproject(activeWorkspace execution.Workspace, selector string) (targetDir, relativeDir string, err error) {
	selector = strings.TrimSpace(selector)
	if selector != "" {
		project, ok := activeWorkspace.Project(selector)
		if ok {
			return project.TargetDir, project.RelativeDir, nil
		}
		hint := ""
		if names := activeWorkspace.ProjectNames(); len(names) > 0 {
			hint = i18n.Tf("exec.available_projects", strings.Join(names, ", "))
		}
		return "", "", cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND,
			i18n.Tf("exec.project_missing", selector, hint))
	}

	project, ok := activeWorkspace.ProjectFromWorkingDirectory()
	if !ok {
		return "", "", cliErrors.New(cliErrors.RUN_USAGE_INVALID,
			i18n.T("exec.project_directory_required"))
	}
	return project.TargetDir, project.RelativeDir, nil
}

// augmentPathForRun prepends the workspace's node_modules/.bin directories
// to PATH so subprocess commands resolve like they would under `npm run`.
// Without this, `one exec -- astro dev` against a subproject whose deps live
// in a hoisted root node_modules dies with "astro: command not found".
//
// Order (highest precedence first): subproject .bin, workspace root .bin,
// inherited PATH. Replaces any existing PATH= entry in env.
func augmentPathForRun(env []string, projectRoot, targetDir string) []string {
	binPaths := []string{
		filepath.Join(targetDir, "node_modules", ".bin"),
		filepath.Join(projectRoot, "node_modules", ".bin"),
	}
	sep := string(os.PathListSeparator)
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, kv := range env {
		key, existing, found := strings.Cut(kv, "=")
		if !replaced && found && isPathEnvironmentKey(key) {
			parts := append([]string{}, binPaths...)
			if existing != "" {
				parts = append(parts, existing)
			}
			out = append(out, "PATH="+strings.Join(parts, sep))
			replaced = true
			continue
		}
		out = append(out, kv)
	}
	if !replaced {
		out = append(out, "PATH="+strings.Join(binPaths, sep))
	}
	return out
}

// lookPathFor resolves an unqualified executable name against the PATH
// embedded in env. Wraps stdlib LookPath by temporarily swapping PATH on
// the current process — exec.LookPath ignores cmd.Env and only consults
// the parent's environment, which would defeat augmentPathForRun.
//
// runRun is single-shot per invocation, so this isn't racing any other
// goroutine that reads PATH. The defer guarantees we restore even if
// LookPath panics.
func lookPathFor(name string, env []string) (string, error) {
	if strings.ContainsAny(name, "/\\") {
		// Already path-qualified; let exec.LookPath validate executability.
		return exec.LookPath(name)
	}
	var augmented string
	for _, kv := range env {
		key, value, found := strings.Cut(kv, "=")
		if found && isPathEnvironmentKey(key) {
			augmented = value
			break
		}
	}
	if augmented == "" {
		return exec.LookPath(name)
	}
	orig, hadPath := os.LookupEnv("PATH")
	if err := os.Setenv("PATH", augmented); err != nil {
		return "", err
	}
	defer func() {
		if hadPath {
			_ = os.Setenv("PATH", orig)
		} else {
			_ = os.Unsetenv("PATH")
		}
	}()
	return exec.LookPath(name)
}

func isPathEnvironmentKey(key string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(key, "PATH")
	}
	return key == "PATH"
}
