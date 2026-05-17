// Package runcmd contributes `one run` to the root command via cliexts.
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
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/cliexts"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func init() {
	cliexts.Register("run", buildContributions)
}

func buildContributions() []*cobra.Command {
	return []*cobra.Command{newRunCmd()}
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
	project     string
	envName     string
	envProvider string
}

func newRunCmd() *cobra.Command {
	flags := &runFlags{}
	cmd := &cobra.Command{
		Use:                   "run [-p <name|path>] [--env-provider dotenv|infisical] [--env <name>] -- <cmd> [args...]",
		DisableFlagsInUseLine: true,
		Long: `把项目环境变量注入到任意命令，类似 infisical run。

项目解析：
  - 不传 -p：从当前目录推导（必须 cd 到某个项目目录里）
  - 传 -p <name>：按 manifest.projects[].name 选（如 -p web）
  - 传 -p <relativeDir>：按相对路径选（如 -p apps/web）

密钥来源（--env-provider，默认取 workspace 在 one create 时选择的 provider）：
  - dotenv    ：读 <project>/.env 文件
  - infisical ：联网从 Infisical 拉

环境名（--env）：
  覆盖 manifest.environments.default，比如 --env staging。

工作目录：
  子进程总在解析出的项目目录里运行（不论你从哪里 cd 进来）。

注入的密钥默认会覆盖已存在的 shell 环境变量。

示例：
  one run -- npm run dev                          # 用 workspace 默认 provider
  one run -p web -- npm start                     # 按 manifest 里的 name 选
  one run -p apps/web -- npm start                # 按相对路径选
  one run --env-provider dotenv -- npm test       # 强制走 dotenv（离线场景）
  one run --env staging -- npm run e2e            # 用 staging 环境的密钥`,
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				// 用户没传命令视为请求帮助：走 stdout（方便 `one run | less`），
				// 父命令空参那条路径在 cobra 里默认走 stderr，这里显式纠偏。
				cmd.SetOut(os.Stdout)
				return cmd.Help()
			}
			return runRun(cmd.Context(), flags, args)
		},
	}
	cmd.Flags().StringVarP(&flags.project, "project", "p", "", "项目名（manifest.projects[].name）或相对路径；默认从 cwd 推导")
	cmd.Flags().StringVar(&flags.envName, "env", "", "环境名（默认取 manifest.environments.default）")
	cmd.Flags().StringVar(&flags.envProvider, "env-provider", "", "env provider: dotenv | infisical（默认取 workspace manifest 中已选的值）")
	// SetInterspersed(false) lets users skip the `--` separator: any token
	// after the first positional is treated as a positional, including
	// things that look like flags. Both `one run -- npm start --foo` and
	// `one run npm start --foo` work; `--foo` reaches npm verbatim.
	cmd.Flags().SetInterspersed(false)
	i18n.MarkShort(cmd, "run.short")
	return cmd
}

func runRun(ctx context.Context, flags *runFlags, args []string) error {
	projectRoot, err := resolveWorkspaceRoot("")
	if err != nil {
		return err
	}

	targetDir, relativeDir, err := resolveRunSubproject(projectRoot, flags.project)
	if err != nil {
		return err
	}

	vars, source, err := loadRunSecrets(ctx, flags, projectRoot, relativeDir, targetDir)
	if err != nil {
		return err
	}
	if output.IsTTY() {
		if len(vars) == 0 && source == loaderIDDotenv {
			fmt.Fprintln(os.Stderr, "⚠ 未找到 .env 或文件为空，继续执行（如需注入变量请创建 .env）")
		} else {
			fmt.Fprintf(os.Stderr, "✓ 注入 %d 个环境变量（来源：%s）\n", len(vars), source)
		}
	}

	childEnv := secrets.MergeIntoEnviron(os.Environ(), vars, true)
	// Always inject node_modules/.bin so commands like `astro` / `next` / `vite`
	// resolve when invoked directly (and so `npm run dev` finds hoisted bins
	// in pnpm/turbo workspaces). Subproject-local first, workspace root next,
	// inherited PATH last — this is the same precedence npm/pnpm use.
	childEnv = augmentPathForRun(childEnv, projectRoot, targetDir)

	binary, err := lookPathFor(args[0], childEnv)
	if err != nil {
		return cliErrors.New(cliErrors.RUN_COMMAND_NOT_FOUND,
			fmt.Sprintf("命令未找到：%s（已在 PATH 中查过：含 %s/node_modules/.bin 与 %s/node_modules/.bin）",
				args[0], relativeDir, filepath.Base(projectRoot))).
			WithContext(map[string]any{
				"command": args[0],
			})
	}

	child := exec.Command(binary, args[1:]...)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.Env = childEnv
	child.Dir = targetDir

	if err := child.Start(); err != nil {
		return cliErrors.New(cliErrors.RUN_COMMAND_NOT_FOUND,
			fmt.Sprintf("启动 %s 失败：%v", args[0], err)).
			WithContext(map[string]any{"command": args[0]})
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case sig := <-sigCh:
				if child.Process != nil {
					_ = child.Process.Signal(sig)
				}
			case <-done:
				return
			}
		}
	}()

	waitErr := child.Wait()
	close(done)
	signal.Stop(sigCh)

	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		return cliErrors.New(cliErrors.ONE_CLI_ERROR,
			fmt.Sprintf("等待子进程失败：%v", waitErr))
	}
	return nil
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
	// (internal/secrets/infisical/loader.go, dotenv/loader.go).
	loaderIDInfisical = "infisical"
	loaderIDDotenv    = "dotenv"
)

func loadRunSecrets(ctx context.Context, flags *runFlags, projectRoot, relativeDir, _ string) (map[string]string, string, error) {
	providerID := strings.ToLower(strings.TrimSpace(flags.envProvider))
	if providerID == "" {
		m, err := workspace.ReadManifest(projectRoot)
		if err != nil {
			return nil, "", err
		}
		providerID = workspace.EnvBackend(m)
		if providerID == "" {
			providerID = loaderIDDotenv
		}
	}

	if providerID != loaderIDDotenv && providerID != loaderIDInfisical {
		return nil, "", cliErrors.New(cliErrors.RUN_DOTENV_MISSING,
			"--env-provider 取值非法："+providerID+"（合法值: dotenv | infisical）")
	}

	loader := secrets.Find(providerID)
	if loader == nil {
		return nil, "", cliErrors.New(cliErrors.ONE_CLI_ERROR,
			"内部错误：env provider "+providerID+" 未注册到二进制。")
	}

	vars, err := loader.Load(ctx, projectRoot, relativeDir, flags.envName)
	if err != nil {
		return nil, "", err
	}
	return vars, loader.ID(), nil
}

// resolveWorkspaceRoot is a thin alias over workspace.WalkUpToManifest.
// Kept as a local name for readability inside this file (`one run` is
// intentionally forgiving about cwd: the natural use case is "I'm
// cd'd into a subproject").
func resolveWorkspaceRoot(dirFlag string) (string, error) {
	return workspace.WalkUpToManifest(dirFlag)
}

// resolveRunSubproject picks which subproject's .env to load.
//   - explicit -p / --project: pnpm-style selector — first by name, then by
//     relativeDir. Delegates to workspace.ResolveProjectFromSelector so
//     `one run` and `one env` agree on what a selector means.
//   - else: figure out which subproject the current cwd is inside via
//     workspace.ResolveProjectFromCWD; error out if cwd is at workspace root
//     or somewhere outside any subproject.
func resolveRunSubproject(projectRoot, selector string) (targetDir, relativeDir string, err error) {
	selector = strings.TrimSpace(selector)
	if selector != "" {
		sub, err := workspace.ResolveProjectFromSelector(projectRoot, selector)
		if err != nil {
			return "", "", err
		}
		if sub != nil {
			return sub.TargetDir, sub.RelativeDir, nil
		}
		m, _ := workspace.ReadManifest(projectRoot)
		hint := ""
		if names := workspace.ProjectNames(m); len(names) > 0 {
			hint = "；可选：" + strings.Join(names, ", ")
		}
		return "", "", cliErrors.New(cliErrors.SUBPROJECT_NOT_FOUND,
			"找不到名字或路径匹配 "+selector+" 的项目"+hint)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	sub, err := workspace.ResolveProjectFromCWD(projectRoot, cwd)
	if err != nil {
		return "", "", err
	}
	if sub == nil {
		return "", "", cliErrors.New(cliErrors.RUN_DOTENV_MISSING,
			"当前目录不在任何项目内；请 cd 进项目，或加 -p <name|path>。")
	}
	return sub.TargetDir, sub.RelativeDir, nil
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
		if !replaced && strings.HasPrefix(kv, "PATH=") {
			existing := strings.TrimPrefix(kv, "PATH=")
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
		if strings.HasPrefix(kv, "PATH=") {
			augmented = strings.TrimPrefix(kv, "PATH=")
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
