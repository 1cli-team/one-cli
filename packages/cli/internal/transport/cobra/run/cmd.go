// Package runcmd contributes `one run` to the explicit root command.
// Executes a passthrough command with the resolved subproject's secrets
// injected into the child environment. Spirit follows `infisical run` /
// `dotenv run`.
package runcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	platformprocess "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

func Commands(loaders *secrets.Registry, provider runtimeport.Provider) []*cobra.Command {
	return []*cobra.Command{newRunCmd(loaders, provider), newExecCmd(loaders)}
}

// newRunCmd wires `one run` — exec a passthrough command with the resolved
// subproject's secrets injected into the child environment. Spirit follows
// `infisical run -- <cmd>` / `dotenv -- <cmd>`.
//
// Provider resolution (--env-provider):
//  1. Default = workspace's recorded env provider (manifest.domains.env.kind),
//     set at `one create --env-provider` time.
//  2. --env-provider dotenv: read <project>/.env files.
//  3. --env-provider infisical: live fetch from Infisical.
//
// Working directory: always the resolved subproject's TargetDir, so commands
// like `npm start` find their package.json regardless of cwd.
//
// Process model: child stdin/stdout/stderr are wired straight to the parent.
// SIGINT/SIGTERM are forwarded so Ctrl-C kills the child first; we exit with
// the child's exit code so scripts and CI can branch normally.
type runFlags struct {
	global       bool
	globalPath   string
	globalKeys   []string
	project      string
	envName      string
	envProvider  string
	runtime      string
	prepared     bool
	provider     runtimeport.Provider
	dryRun       bool
	outputFormat string
}

func newRunCmd(loaders *secrets.Registry, provider runtimeport.Provider) *cobra.Command {
	flags := &runFlags{provider: provider}
	cmd := &cobra.Command{
		Use:                   "run [project] [-p <name|path>] [--env-provider dotenv|infisical] [--env <name>] -- <cmd> [args...]",
		DisableFlagsInUseLine: true,
		Long:                  i18n.T("run.tip"),
		Args:                  cobra.ArbitraryArgs,
		DisableFlagParsing:    false,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags.outputFormat, _ = cmd.Flags().GetString("output")
			commandArgs, err := parseRunArgs(cmd, flags, args)
			if err != nil {
				return err
			}
			if len(commandArgs) == 0 {
				// 用户没传命令视为请求帮助：走 stdout（方便 `one run | less`），
				// 父命令空参那条路径在 cobra 里默认走 stderr，这里显式纠偏。
				cmd.SetOut(os.Stdout)
				return cmd.Help()
			}
			if flags.global {
				return runGlobal(cmd.Context(), flags, commandArgs)
			}
			if cmd.Flags().Changed("path") || cmd.Flags().Changed("keys") {
				return i18n.Errorf("run.global_flags_required")
			}
			return runRun(cmd.Context(), loaders, flags, commandArgs)
		},
	}
	i18n.MarkLong(cmd, "run.tip")
	cmd.Flags().StringVarP(&flags.project, "project", "p", "", i18n.T("run.flag.project"))
	i18n.MarkFlagUsage(cmd, "project", "run.flag.project")
	cmd.Flags().StringVar(&flags.envName, "env", "", i18n.T("run.flag.env"))
	i18n.MarkFlagUsage(cmd, "env", "run.flag.env")
	cmd.Flags().StringVar(&flags.envProvider, "env-provider", "", i18n.T("run.flag.provider"))
	i18n.MarkFlagUsage(cmd, "env-provider", "run.flag.provider")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, i18n.T("run.flag.dry_run"))
	i18n.MarkFlagUsage(cmd, "dry-run", "run.flag.dry_run")
	cmd.Flags().BoolVar(&flags.global, "global", false, i18n.T("run.flag.global"))
	i18n.MarkFlagUsage(cmd, "global", "run.flag.global")
	cmd.Flags().StringVar(&flags.globalPath, "path", "", i18n.T("run.flag.path"))
	i18n.MarkFlagUsage(cmd, "path", "run.flag.path")
	cmd.Flags().StringSliceVar(&flags.globalKeys, "keys", nil, i18n.T("run.flag.keys"))
	i18n.MarkFlagUsage(cmd, "keys", "run.flag.keys")
	i18n.MarkShort(cmd, "run.short")
	return cmd
}

func parseRunArgs(cmd *cobra.Command, flags *runFlags, args []string) ([]string, error) {
	dashIndex := cmd.ArgsLenAtDash()
	if dashIndex < 0 {
		if len(args) == 0 {
			return nil, nil
		}
		return nil, cliErrors.New(cliErrors.RUN_USAGE_INVALID,
			i18n.T("run.separator_required")).
			WithContext(map[string]any{"reason": "separator-required"})
	}
	if dashIndex > 1 {
		return nil, cliErrors.New(cliErrors.RUN_USAGE_INVALID,
			i18n.T("run.project_args_invalid")).
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
					i18n.T("run.selector_conflict")).
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
			i18n.T("run.command_required")).
			WithContext(map[string]any{"reason": "command-required"})
	}
	return commandArgs, nil
}

func runRun(ctx context.Context, loaders *secrets.Registry, flags *runFlags, args []string) error {
	activeWorkspace, err := execution.ResolveWorkspace(ctx)
	if err != nil {
		return err
	}
	projectRoot := activeWorkspace.Root()

	targetDir, relativeDir, err := resolveRunSubproject(activeWorkspace, flags.project)
	if err != nil {
		return err
	}

	if !flags.prepared {
		flags.runtime, err = execution.RuntimeKind(projectRoot)
		if err != nil {
			return err
		}
	}
	if err := runtimeport.Validate(flags.runtime); err != nil {
		return err
	}
	if flags.dryRun {
		output.Emit(map[string]any{"schema": "one-cli/run-plan/v1", "runtime": flags.runtime, "directory": targetDir, "argv": args, "environment": flags.envName, "dry_run": true})
		return nil
	}
	if flags.runtime == runtimeport.Mise {
		binary, err := os.Executable()
		if err != nil {
			return err
		}
		childArgs := []string{binary, "__exec", "--protocol", "1", "--project", relativeDir}
		if flags.outputFormat != "" {
			childArgs = append(childArgs, "--output", flags.outputFormat)
		}
		if flags.envName != "" {
			childArgs = append(childArgs, "--env", flags.envName)
		}
		if flags.envProvider != "" {
			childArgs = append(childArgs, "--env-provider", flags.envProvider)
		}
		childArgs = append(append(childArgs, "--"), args...)
		if flags.provider == nil {
			return cliErrors.New(cliErrors.ONE_CLI_ERROR, i18n.T("run.mise_missing"))
		}
		prepared, err := flags.provider.Prepare(ctx, runtimeport.Command{Directory: targetDir, Argv: childArgs, Env: os.Environ()})
		if err != nil {
			return err
		}
		child := platformprocess.Command(prepared.Argv[0], prepared.Argv[1:]...)
		child.Dir, child.Env = prepared.Directory, prepared.Env
		child.Stdin, child.Stdout, child.Stderr = os.Stdin, os.Stdout, os.Stderr
		return platformprocess.RunForwarded(ctx, child)
	}

	vars, source, err := loadRunSecrets(
		ctx, loaders, flags, projectRoot, activeWorkspace.Manifest(), relativeDir,
	)
	if err != nil {
		return err
	}
	if output.IsTTY() {
		if len(vars) == 0 && source == loaderIDDotenv {
			fmt.Fprintln(os.Stderr, i18n.T("run.env_empty"))
		} else {
			fmt.Fprintf(os.Stderr, i18n.T("run.injected")+"\n", len(vars), source)
		}
	}

	childEnv := secrets.MergeIntoEnviron(os.Environ(), vars, true)
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
			i18n.Tf("run.command_missing",
				args[0], relativeDir, filepath.Base(projectRoot))).
			WithContext(map[string]any{
				"command": args[0],
			})
	}

	child := platformprocess.Command(binary, args[1:]...)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.Env = childEnv
	child.Dir = targetDir

	err = platformprocess.RunForwarded(ctx, child)
	var exit *platformprocess.ExitStatus
	if err != nil && !errors.As(err, &exit) {
		return cliErrors.New(cliErrors.RUN_COMMAND_NOT_FOUND, i18n.Tf("run.start_failed", args[0], err))
	}
	return err
}

// loadRunSecrets resolves the secret source per --env-provider and returns
// the merged map plus the loader ID used (e.g. "infisical", "dotenv").
//
// Provider resolution:
//   - flag value wins ("dotenv" | "infisical")
//   - else read manifest.domains.env.kind (set at `one create --env-provider` time)
//   - fall back to "dotenv" if manifest somehow has no backend recorded
const (
	// loaderIDInfisical / loaderIDDotenv MUST stay in lockstep with
	// the ID() returned by their respective loader implementations
	// (internal/adapters/env/infisical/loader.go, dotenv/loader.go).
	loaderIDInfisical = "infisical"
	loaderIDDotenv    = "dotenv"
)

func loadRunSecrets(
	ctx context.Context,
	loaders *secrets.Registry,
	flags *runFlags,
	projectRoot string,
	manifest *workspace.Manifest,
	relativeDir string,
) (map[string]string, string, error) {
	providerID := strings.ToLower(strings.TrimSpace(flags.envProvider))
	if providerID == "" {
		providerID = workspace.EnvBackend(manifest)
		if providerID == "" {
			providerID = loaderIDDotenv
		}
	}

	if providerID != loaderIDDotenv && providerID != loaderIDInfisical {
		return nil, "", cliErrors.New(cliErrors.RUN_DOTENV_MISSING,
			i18n.Tf("run.provider_invalid", providerID))
	}

	loader := loaders.Find(providerID)
	if loader == nil {
		return nil, "", cliErrors.New(cliErrors.ONE_CLI_ERROR,
			i18n.Tf("run.provider_unregistered", providerID))
	}

	vars, err := loader.Load(ctx, projectRoot, relativeDir, flags.envName)
	if err != nil {
		return nil, "", err
	}
	return vars, loader.ID(), nil
}

// resolveRunSubproject picks which subproject's .env to load.
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
			hint = i18n.Tf("run.available_projects", strings.Join(names, ", "))
		}
		return "", "", cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND,
			i18n.Tf("run.project_missing", selector, hint))
	}

	project, ok := activeWorkspace.ProjectFromWorkingDirectory()
	if !ok {
		return "", "", cliErrors.New(cliErrors.RUN_DOTENV_MISSING,
			i18n.T("run.project_directory_required"))
	}
	return project.TargetDir, project.RelativeDir, nil
}

// augmentPathForRun prepends the workspace's node_modules/.bin directories
// to PATH so subprocess commands resolve like they would under `npm run`.
// Without this, `one run -- astro dev` against a subproject whose deps live
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
