// Package vercel implements the deploy/vercel backend. The provider
// shells out to the upstream `vercel` CLI rather than calling Vercel's
// REST API directly: the CLI handles project linking, build artifact
// upload, environment-variable sync, and deployment polling for us, so
// adding a Go-side reimplementation would be net-negative effort.
//
// Layout follows the same shape as other infra/<name>/ packages —
// ops.go houses argv builders + the actual exec call so it stays
// trivially testable, sync.go scaffolds vercel.json, and provider.go
// adapts everything onto the deploy.Provider interface and registers
// at init() time.
package vercel

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets"
)

// SchemaApply is the JSON envelope schema string for a vercel deploy.
// Versioned per-backend to allow Vercel-specific shape changes without
// disturbing kustomize / s3 envelopes.
const SchemaApply = "one-cli/deploy-apply-vercel/v1"

// CLIBinary is the executable name we look for on $PATH. Exported so
// tests can stub it (the test variant points at a fake binary).
var CLIBinary = "vercel"

// ApplyInput addresses Apply.
type ApplyInput struct {
	// ProjectDir is the absolute working directory `vercel` runs in
	// (the per-project dir, not the workspace root). The `vercel`
	// CLI scans this dir for vercel.json + framework auto-detection.
	ProjectDir string

	// APIToken is the Vercel API token (from the deploy/vercel profile).
	APIToken string

	// Team is the org slug passed via `--scope`. Empty = personal scope.
	Team string

	// Env names the deploy target environment (from manifest.environments.names,
	// or the deploy command's --env flag). Empty or "prod" runs a production
	// deploy (vercel build/deploy --prod, vercel pull --environment=production);
	// any other value runs a preview deploy. Vercel only has the two-state
	// production/preview distinction at the platform level — staging or
	// custom envs all collapse to preview here.
	Env string

	// DryRun returns the planned argv without invoking `vercel`.
	DryRun bool

	// InjectedEnv is the project's user-set env vars (resolved by
	// deploycmd from dotenv / Infisical). Threaded into the vercel CLI
	// subprocess via cmd.Env so user-side build scripts (Next.js,
	// Nuxt, etc.) read them via process.env during `vercel build`.
	//
	// We deliberately do NOT write these into .vercel/.env.* files —
	// those files are owned by `vercel pull` (mirror of Vercel cloud
	// env). Mixing local one-cli env with cloud-pulled env on disk
	// would create two competing sources of truth. Runtime env (i.e.
	// what the deployed function sees in production) is still
	// Vercel's job, configured via Vercel UI / `vercel env add` and
	// pulled by `vercel pull`. nil = no injection.
	InjectedEnv map[string]string
}

// ApplyResult is the JSON envelope emitted on success.
type ApplyResult struct {
	Schema       string   `json:"schema"`
	Argv         []string `json:"argv"`
	CommandLines []string `json:"command_lines,omitempty"`
	DryRun       bool     `json:"dry_run"`

	// DeploymentURL is captured from the vercel CLI output on success.
	// Empty in dry-run.
	DeploymentURL string `json:"deployment_url,omitempty"`
}

// Apply runs the vercel CLI sequence to deploy one project:
//
//	vercel pull   --yes --environment=production [--scope=<team>] --token=...
//	vercel build  [--prod]
//	vercel deploy --prebuilt [--prod] [--scope=<team>] --token=...
//
// The pull step caches project link + env vars; build does the
// platform's prebuild step locally; deploy uploads the prebuilt
// artifacts. Token is passed via --token flag (vercel CLI supports
// reading from VERCEL_TOKEN as well, but flag-passing keeps the
// command transcript explicit for dry-run output).
func Apply(ctx context.Context, in ApplyInput) (*ApplyResult, error) {
	if strings.TrimSpace(in.APIToken) == "" {
		return nil, cliErrors.New(cliErrors.VERCEL_PROFILE_INVALID,
			"deploy/vercel profile 缺少 API token。先 `one configure add deploy/vercel --profile <name> --token <api-token> --use`。")
	}

	pullArgv := buildPullArgv(in)
	buildArgv := buildBuildArgv(in)
	deployArgv := buildDeployArgv(in)
	commandLines := []string{
		argvDisplay(maskTokenInArgv(pullArgv)),
		argvDisplay(maskTokenInArgv(buildArgv)),
		argvDisplay(maskTokenInArgv(deployArgv)),
	}

	if in.DryRun {
		return &ApplyResult{
			Schema:       SchemaApply,
			Argv:         maskTokenInArgv(deployArgv),
			CommandLines: commandLines,
			DryRun:       true,
		}, nil
	}

	if _, err := exec.LookPath(CLIBinary); err != nil {
		return nil, cliErrors.New(cliErrors.VERCEL_CLI_MISSING,
			"未在 PATH 中找到 `vercel` 二进制。安装方式见错误的 remediation。").
			WithContext(map[string]any{"binary": CLIBinary})
	}

	if err := runStep(ctx, in.ProjectDir, pullArgv, "vercel pull", in.InjectedEnv); err != nil {
		return nil, err
	}
	if err := runStep(ctx, in.ProjectDir, buildArgv, "vercel build", in.InjectedEnv); err != nil {
		return nil, err
	}
	url, err := runDeployStep(ctx, in.ProjectDir, deployArgv, in.InjectedEnv)
	if err != nil {
		return nil, err
	}
	return &ApplyResult{
		Schema:        SchemaApply,
		Argv:          maskTokenInArgv(deployArgv),
		CommandLines:  commandLines,
		DryRun:        false,
		DeploymentURL: url,
	}, nil
}

// isProduction reports whether the requested env represents Vercel's
// production tier. Empty (default) and "prod" both map to production;
// any other value (e.g. "dev", "staging", custom) maps to preview.
func isProduction(env string) bool {
	env = strings.TrimSpace(env)
	return env == "" || env == "prod"
}

func buildPullArgv(in ApplyInput) []string {
	argv := []string{CLIBinary, "pull", "--yes"}
	if isProduction(in.Env) {
		argv = append(argv, "--environment=production")
	} else {
		argv = append(argv, "--environment=preview")
	}
	if scope := strings.TrimSpace(in.Team); scope != "" {
		argv = append(argv, "--scope="+scope)
	}
	argv = append(argv, "--token="+in.APIToken)
	return argv
}

func buildBuildArgv(in ApplyInput) []string {
	argv := []string{CLIBinary, "build"}
	if isProduction(in.Env) {
		argv = append(argv, "--prod")
	}
	if scope := strings.TrimSpace(in.Team); scope != "" {
		argv = append(argv, "--scope="+scope)
	}
	argv = append(argv, "--token="+in.APIToken)
	return argv
}

func buildDeployArgv(in ApplyInput) []string {
	argv := []string{CLIBinary, "deploy", "--prebuilt"}
	if isProduction(in.Env) {
		argv = append(argv, "--prod")
	}
	if scope := strings.TrimSpace(in.Team); scope != "" {
		argv = append(argv, "--scope="+scope)
	}
	argv = append(argv, "--token="+in.APIToken)
	return argv
}

// maskTokenInArgv replaces the literal token value with `********` so
// the wire-format envelope (and dry-run output) never leaks the API
// token. The `vercel` invocation itself uses the real argv.
func maskTokenInArgv(argv []string) []string {
	out := make([]string, len(argv))
	for i, a := range argv {
		switch {
		case strings.HasPrefix(a, "--token="):
			out[i] = "--token=********"
		default:
			out[i] = a
		}
	}
	return out
}

// argvDisplay joins argv into a copy-pasteable single-line string for
// dry-run / command_lines output.
func argvDisplay(argv []string) string {
	return strings.Join(argv, " ")
}

func runStep(ctx context.Context, dir string, argv []string, label string, injected map[string]string) error {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = vercelEnv(os.Environ(), injected)
	if err := cmd.Run(); err != nil {
		return cliErrors.New(cliErrors.VERCEL_DEPLOY_FAILED,
			fmt.Sprintf("`%s` 失败：%v", label, err)).
			WithContext(map[string]any{"argv": maskTokenInArgv(argv)}).
			WithRemediation(output.Remediation{
				Action: "rerun-step",
				Hint:   "在项目目录手动跑一次该步骤,看完整 vercel CLI 输出",
			})
	}
	return nil
}

// runDeployStep is the same as runStep except it also captures stdout
// to lift the deployment URL out of the CLI output. `vercel deploy`
// prints the URL on its own line, e.g. https://my-app-abc123.vercel.app.
func runDeployStep(ctx context.Context, dir string, argv []string, injected map[string]string) (string, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	cmd.Env = vercelEnv(os.Environ(), injected)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	url := captureDeploymentURL(stdout)
	if err := cmd.Wait(); err != nil {
		return "", cliErrors.New(cliErrors.VERCEL_DEPLOY_FAILED,
			fmt.Sprintf("`vercel deploy` 失败：%v", err)).
			WithContext(map[string]any{"argv": maskTokenInArgv(argv)}).
			WithRemediation(output.Remediation{
				Action: "rerun-step",
				Hint:   "手动 `vercel deploy --prebuilt --prod` 看完整输出",
			})
	}
	return url, nil
}

// vercelEnv merges user-injected env vars on top of the parent shell
// (override=true so user .env beats stale shell vars). Unlike wrangler /
// edgeone we do NOT strip auth env vars: vercel reads its API token
// from the --token argv flag, not from VERCEL_TOKEN env, so there is
// no auth env to protect from accidental injection. nil injected = no
// merge, behaviour equals os.Environ().
func vercelEnv(parent []string, injected map[string]string) []string {
	return secrets.MergeIntoEnviron(parent, injected, true)
}

// captureDeploymentURL streams the CLI stdout to the user's terminal
// while sniffing for the first https://*.vercel.app URL. Vercel emits
// the production URL on a dedicated line at the end of `vercel deploy`.
func captureDeploymentURL(stdout interface{ Read(p []byte) (int, error) }) string {
	buf := make([]byte, 4096)
	var collected strings.Builder
	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			os.Stdout.Write(buf[:n])
			collected.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	for _, line := range strings.Split(collected.String(), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "https://") && strings.Contains(line, ".vercel.app") {
			return line
		}
	}
	return ""
}
