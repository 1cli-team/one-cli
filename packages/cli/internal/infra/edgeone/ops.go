// Package edgeone implements the deploy/edgeone backend. The provider
// shells out to Tencent Cloud's `edgeone` CLI rather than reimplementing
// the EdgeOne Pages REST API in Go: the CLI handles project linking,
// asset upload, deployment polling, and version routing for us.
//
// Layout follows the same shape as infra/cloudflare/ — ops.go houses
// argv builders + the actual exec call so it stays trivially testable;
// sync.go scaffolds an edgeone.json hint; provider.go adapts everything
// onto the deploy.Provider interface and registers at init() time.
//
// Auth threading: the current edgeone CLI authenticates non-interactive
// deploys via `edgeone pages deploy --token`. Dry-run and result
// envelopes redact the token before printing.
//
// NOTE: the edgeone CLI is less stable than wrangler — flag set,
// sub-commands, and even the auth env var names have changed across
// 2024–2026. The argv builder below targets `edgeone pages deploy`
// against the asset directory; if your edgeone CLI version differs,
// adjust ops.go and re-run the unit tests. The schema version below
// gives us a clean break point if we need to ship a v2.
package edgeone

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

// SchemaApply is the JSON envelope schema string for an edgeone deploy.
const SchemaApply = "one-cli/deploy-apply-edgeone/v1"

// CLIBinary is the executable name we look for on $PATH. Exported so
// tests can stub it.
var CLIBinary = "edgeone"

const redactedToken = "<redacted>"

// ApplyInput addresses Apply.
type ApplyInput struct {
	// ProjectDir is the absolute working directory edgeone runs in
	// (the per-project dir, not the workspace root). Used both as
	// `--directory` for the deploy upload and as the cwd for the
	// CLI invocation.
	ProjectDir string

	// AssetDir is the optional sub-directory containing the build
	// output (e.g. "dist", ".output/public"). Empty defaults to
	// ProjectDir; non-empty is joined under ProjectDir.
	AssetDir string

	// APIToken is the EdgeOne Pages API token. The upstream CLI accepts
	// it as --token; dry-run / JSON output always redact it.
	APIToken string

	// Region is the optional Tencent region slug. Retained in the
	// profile for future upstream CLI support.
	Region string

	// ProjectName is the EdgeOne Pages project slug (--project-name).
	// When empty, edgeone CLI falls back to edgeone.json or prompts.
	ProjectName string

	// Env names the deploy target environment (from manifest.environments.names,
	// or the deploy command's --env flag). Empty or "prod" runs a production
	// deploy; any other value runs a preview deploy (--env=preview on the
	// upstream CLI). EdgeOne Pages only exposes the two-state production/
	// preview distinction, so staging or custom envs collapse to preview.
	Env string

	// DryRun returns the planned argv without invoking edgeone.
	DryRun bool

	// InjectedEnv is the project's user-set env vars (resolved by
	// deploycmd from dotenv / Infisical). Merged into edgeone's child
	// env before the auth env vars; the static-asset build typically
	// runs upstream of `edgeone pages deploy`, but any in-CLI bundling
	// step still sees them via process.env. nil = none.
	InjectedEnv map[string]string
}

// ApplyResult is the JSON envelope emitted on success.
type ApplyResult struct {
	Schema       string   `json:"schema"`
	Argv         []string `json:"argv"`
	CommandLines []string `json:"command_lines,omitempty"`
	DryRun       bool     `json:"dry_run"`

	// DeploymentURL is captured from the edgeone CLI output on
	// success. Empty in dry-run.
	DeploymentURL string `json:"deployment_url,omitempty"`
}

// Apply runs the edgeone CLI to deploy one project:
//
//	edgeone pages deploy <asset-dir> [--name <name>] [--env preview]
//
// One step: edgeone bundles + uploads atomically.
func Apply(ctx context.Context, in ApplyInput) (*ApplyResult, error) {
	deployArgv := buildDeployArgv(in)
	displayArgv := redactedDeployArgv(in)
	commandLines := []string{argvDisplay(displayArgv)}

	if in.DryRun {
		return &ApplyResult{
			Schema:       SchemaApply,
			Argv:         displayArgv,
			CommandLines: commandLines,
			DryRun:       true,
		}, nil
	}

	if _, err := exec.LookPath(CLIBinary); err != nil {
		return nil, cliErrors.New(cliErrors.EDGEONE_CLI_MISSING,
			"未在 PATH 中找到 `edgeone` 二进制。安装方式见错误的 remediation。").
			WithContext(map[string]any{"binary": CLIBinary})
	}

	url, err := runDeployStep(ctx, in.ProjectDir, deployArgv, in)
	if err != nil {
		return nil, err
	}
	return &ApplyResult{
		Schema:        SchemaApply,
		Argv:          displayArgv,
		CommandLines:  commandLines,
		DryRun:        false,
		DeploymentURL: url,
	}, nil
}

// isProduction reports whether the requested env represents EdgeOne's
// production tier. Empty (default) and "prod" both map to production;
// any other value (e.g. "dev", "staging", custom) maps to preview.
func isProduction(env string) bool {
	env = strings.TrimSpace(env)
	return env == "" || env == "prod"
}

func buildDeployArgv(in ApplyInput) []string {
	argv := []string{CLIBinary, "pages", "deploy"}
	dir := strings.TrimSpace(in.AssetDir)
	if dir == "" {
		dir = "."
	}
	argv = append(argv, dir)
	if name := strings.TrimSpace(in.ProjectName); name != "" {
		argv = append(argv, "--name="+name)
	}
	if token := strings.TrimSpace(in.APIToken); token != "" {
		argv = append(argv, "--token", token)
	}
	if !isProduction(in.Env) {
		argv = append(argv, "--env=preview")
	}
	return argv
}

func redactedDeployArgv(in ApplyInput) []string {
	if strings.TrimSpace(in.APIToken) != "" {
		in.APIToken = redactedToken
	}
	return buildDeployArgv(in)
}

// argvDisplay joins argv into a copy-pasteable single-line string for
// dry-run / command_lines output.
func argvDisplay(argv []string) string {
	return strings.Join(argv, " ")
}

// runDeployStep streams edgeone stdout/stderr to the user's terminal
// while sniffing for the deployment URL. EdgeOne Pages prints the URL
// on its own line ending in `.pages.tencent.com` or `.eo.dev`.
func runDeployStep(ctx context.Context, dir string, argv []string, in ApplyInput) (string, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	cmd.Env = edgeoneEnv(os.Environ(), in.InjectedEnv)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	url := captureDeploymentURL(stdout)
	if err := cmd.Wait(); err != nil {
		return "", cliErrors.New(cliErrors.EDGEONE_DEPLOY_FAILED,
			fmt.Sprintf("`edgeone pages deploy` 失败：%v", err)).
			WithContext(map[string]any{"argv": argv}).
			WithRemediation(output.Remediation{
				Action: "rerun-step",
				Hint:   "在项目目录手动跑一次 `edgeone pages deploy`，看完整 edgeone CLI 输出",
			})
	}
	return url, nil
}

// edgeoneEnv builds the env edgeone runs under. User-injected env vars
// override the parent shell so e.g. an .env API_URL beats a stale shell
// API_URL.
func edgeoneEnv(parent []string, injected map[string]string) []string {
	return secrets.MergeIntoEnviron(parent, injected, true)
}

// captureDeploymentURL streams the CLI stdout to the user's terminal
// while sniffing for the first https:// URL ending with
// `.pages.tencent.com` / `.eo.dev` / `.edgeone.app` (the three known
// EdgeOne Pages domain suffixes as of writing).
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
		idx := strings.Index(line, "https://")
		if idx < 0 {
			continue
		}
		candidate := strings.TrimSpace(line[idx:])
		candidate = strings.TrimRight(candidate, ".,;)")
		if strings.Contains(candidate, ".pages.tencent.com") ||
			strings.Contains(candidate, ".eo.dev") ||
			strings.Contains(candidate, ".edgeone.app") {
			return candidate
		}
	}
	return ""
}
