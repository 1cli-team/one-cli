// Package cloudflare implements the deploy/cloudflare backend. The
// provider shells out to wrangler — Cloudflare's official CLI — rather
// than calling Cloudflare's REST API directly: wrangler handles the
// V8 isolate bundling, R2/KV/D1 binding upload, static-asset
// fingerprinting, and Worker version routing for us. Doing that work
// in Go would be net-negative effort.
//
// Layout follows the same shape as infra/vercel/ — ops.go houses argv
// builders + the actual exec call so it stays trivially testable;
// sync.go scaffolds wrangler.toml; provider.go adapts everything onto
// the deploy.Provider interface and registers at init() time.
//
// Auth threading: wrangler reads CLOUDFLARE_API_TOKEN and
// CLOUDFLARE_ACCOUNT_ID from the process environment. We inject them
// via cmd.Env rather than passing on argv so the wire-format envelope
// (and dry-run output) is naturally secret-free — no separate
// masking layer needed.
package cloudflare

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/secrets"
)

// SchemaApply is the JSON envelope schema string for a cloudflare
// deploy. Versioned per-backend to allow Cloudflare-specific shape
// changes without disturbing other envelopes.
const SchemaApply = "one-cli/deploy-apply-cloudflare/v1"

// CLIBinary is the executable name we look for on $PATH. Exported so
// tests can stub it (the test variant points at a fake binary).
var CLIBinary = "wrangler"

// envCloudflareAPIToken is the env var wrangler reads for auth. Same
// name as wrangler's documented contract.
const envCloudflareAPIToken = "CLOUDFLARE_API_TOKEN"

// envCloudflareAccountID is the optional account scope wrangler reads
// when set. Required only on multi-account tokens.
const envCloudflareAccountID = "CLOUDFLARE_ACCOUNT_ID"

// ApplyInput addresses Apply.
type ApplyInput struct {
	// ProjectDir is the absolute working directory wrangler runs in
	// (the per-project dir, not the workspace root). wrangler scans
	// this dir for wrangler.toml + entry-point auto-detection.
	ProjectDir string

	// APIToken is the Cloudflare API token (from the deploy/cloudflare
	// profile). Threaded into wrangler via CLOUDFLARE_API_TOKEN env.
	APIToken string

	// AccountID is the optional account scope. Threaded into wrangler
	// via CLOUDFLARE_ACCOUNT_ID env. Empty leaves the env var unset.
	AccountID string

	// Env names the deploy target environment (from manifest.environments.names,
	// or the deploy command's --env flag). Empty or "prod" runs a production
	// deploy (no --env flag, wrangler's implicit production environment);
	// any other value maps directly to `wrangler deploy --env=<Env>`, where
	// the value must match a [env.<Env>] section in wrangler.toml.
	Env string

	// DryRun returns the planned argv without invoking wrangler.
	DryRun bool

	// InjectedEnv is the project's user-set env vars (resolved by
	// deploycmd from dotenv / Infisical). Merged into wrangler's
	// child env before the auth env vars, so build-time code (e.g.
	// Worker bundling, static-asset transforms) can read them via
	// process.env. Auth env vars (CLOUDFLARE_API_TOKEN /
	// CLOUDFLARE_ACCOUNT_ID) always win on collision. nil = none.
	InjectedEnv map[string]string
}

// ApplyResult is the JSON envelope emitted on success.
type ApplyResult struct {
	Schema       string   `json:"schema"`
	Argv         []string `json:"argv"`
	CommandLines []string `json:"command_lines,omitempty"`
	DryRun       bool     `json:"dry_run"`

	// DeploymentURL is captured from wrangler output on success.
	// Empty in dry-run.
	DeploymentURL string `json:"deployment_url,omitempty"`
}

// Apply runs the wrangler CLI to deploy one project:
//
//	wrangler deploy [--env <name>]
//
// One step, no pull / build separation: wrangler deploy bundles +
// uploads atomically. Auth flows in via CLOUDFLARE_API_TOKEN /
// CLOUDFLARE_ACCOUNT_ID env vars, not flags, so argv stays clean.
func Apply(ctx context.Context, in ApplyInput) (*ApplyResult, error) {
	if strings.TrimSpace(in.APIToken) == "" {
		return nil, cliErrors.New(cliErrors.CLOUDFLARE_PROFILE_INVALID,
			"deploy/cloudflare profile 缺少 API token。先 `one configure add deploy/cloudflare --profile <name> --token <api-token> --use`。")
	}

	deployArgv := buildDeployArgv(in)
	commandLines := []string{argvDisplay(deployArgv)}

	if in.DryRun {
		return &ApplyResult{
			Schema:       SchemaApply,
			Argv:         deployArgv,
			CommandLines: commandLines,
			DryRun:       true,
		}, nil
	}

	if err := preflightD1DatabaseBindings(ctx, in.ProjectDir, in.APIToken, in.AccountID); err != nil {
		return nil, err
	}

	execArgv := deployArgv
	binary, err := resolveCLIBinary(in.ProjectDir)
	if err != nil {
		return nil, err
	}
	if binary != deployArgv[0] {
		execArgv = append([]string{binary}, deployArgv[1:]...)
	}

	url, err := runDeployStep(ctx, in.ProjectDir, execArgv, in.APIToken, in.AccountID, in.InjectedEnv)
	if err != nil {
		return nil, err
	}
	return &ApplyResult{
		Schema:        SchemaApply,
		Argv:          execArgv,
		CommandLines:  commandLines,
		DryRun:        false,
		DeploymentURL: url,
	}, nil
}

func resolveCLIBinary(projectDir string) (string, error) {
	if path, err := exec.LookPath(CLIBinary); err == nil {
		return path, nil
	}
	for _, candidate := range localCLICandidates(projectDir) {
		if isExecutableFile(candidate) {
			return candidate, nil
		}
	}
	return "", cliErrors.New(cliErrors.CLOUDFLARE_CLI_MISSING,
		"未在 PATH 或项目 node_modules/.bin 中找到 `wrangler` 二进制。安装方式见错误的 remediation。").
		WithContext(map[string]any{"binary": CLIBinary, "project_dir": projectDir})
}

func localCLICandidates(projectDir string) []string {
	projectDir = strings.TrimSpace(projectDir)
	if projectDir == "" {
		return nil
	}
	name := filepath.Base(CLIBinary)
	if name == "." || name == string(filepath.Separator) {
		name = CLIBinary
	}
	candidates := []string{filepath.Join(projectDir, "node_modules", ".bin", name)}
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".cmd") {
		candidates = append(candidates, filepath.Join(projectDir, "node_modules", ".bin", name+".cmd"))
	}
	return candidates
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode().Perm()&0o111 != 0
}

func buildDeployArgv(in ApplyInput) []string {
	argv := []string{CLIBinary, "deploy"}
	if envName := resolveEnvName(in); envName != "" {
		argv = append(argv, "--env="+envName)
	}
	return argv
}

// resolveEnvName maps the user-facing Env onto wrangler's --env flag.
// Empty or "prod" → no --env (wrangler's implicit production environment).
// Any other value → that value, passed verbatim to `--env=<value>`.
func resolveEnvName(in ApplyInput) string {
	env := strings.TrimSpace(in.Env)
	if env == "" || env == "prod" {
		return ""
	}
	return env
}

// argvDisplay joins argv into a copy-pasteable single-line string for
// dry-run / command_lines output.
func argvDisplay(argv []string) string {
	return strings.Join(argv, " ")
}

// runDeployStep streams wrangler stdout/stderr to the user's terminal
// while sniffing for the deployment URL. Wrangler prints the deploy
// URL on its own line, e.g. "Published demo (1.2 sec) https://demo.<acct>.workers.dev".
func runDeployStep(ctx context.Context, dir string, argv []string, apiToken, accountID string, injected map[string]string) (string, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	cmd.Env = wranglerEnv(os.Environ(), injected, apiToken, accountID)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	url := captureDeploymentURL(stdout)
	if err := cmd.Wait(); err != nil {
		d1Configured := hasD1DatabaseBinding(dir)
		remediation := []output.Remediation{
			{
				Action: "rerun-step",
				Hint:   "在项目目录手动跑一次 `wrangler deploy`，看完整 wrangler CLI 输出",
			},
		}
		if d1Configured {
			remediation = append(remediation, output.Remediation{
				Action: "verify-d1-binding",
				Hint:   "wrangler.toml 配置了 D1；确认 database_id 对应的数据库已在当前 Cloudflare account 中创建，且 token 有 D1 Edit 权限",
			})
		}
		return "", cliErrors.New(cliErrors.CLOUDFLARE_DEPLOY_FAILED,
			fmt.Sprintf("`wrangler deploy` 失败：%v", err)).
			WithContext(map[string]any{"argv": argv, "d1_binding_configured": d1Configured}).
			WithRemediation(remediation...)
	}
	return url, nil
}

func hasD1DatabaseBinding(projectDir string) bool {
	raw, err := os.ReadFile(filepath.Join(projectDir, "wrangler.toml"))
	if err != nil {
		return false
	}
	return strings.Contains(string(raw), "[[d1_databases]]")
}

// wranglerEnv builds the env wrangler runs under. Order matters:
//
//  1. Merge user-injected env vars on top of the parent shell (override
//     mode) so e.g. an .env API_URL beats a stale shell API_URL.
//  2. Strip + re-append the auth env vars so the profile token /
//     account always wins, even if a user accidentally set
//     CLOUDFLARE_API_TOKEN in their .env.
func wranglerEnv(parent []string, injected map[string]string, apiToken, accountID string) []string {
	base := secrets.MergeIntoEnviron(parent, injected, true)
	out := make([]string, 0, len(base)+2)
	for _, kv := range base {
		if strings.HasPrefix(kv, envCloudflareAPIToken+"=") {
			continue
		}
		if strings.HasPrefix(kv, envCloudflareAccountID+"=") {
			continue
		}
		out = append(out, kv)
	}
	out = append(out, envCloudflareAPIToken+"="+apiToken)
	if strings.TrimSpace(accountID) != "" {
		out = append(out, envCloudflareAccountID+"="+accountID)
	}
	return out
}

// captureDeploymentURL streams the CLI stdout to the user's terminal
// while sniffing for the first https://*.workers.dev or *.pages.dev URL.
// Wrangler emits the deployment URL on a dedicated line at the end of
// `wrangler deploy`.
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
		// Wrangler prefixes URLs with markers like "  https://" — find
		// the first https:// substring on each line.
		idx := strings.Index(line, "https://")
		if idx < 0 {
			continue
		}
		candidate := strings.TrimSpace(line[idx:])
		// Trim trailing punctuation wrangler sometimes appends.
		candidate = strings.TrimRight(candidate, ".,;)")
		if strings.Contains(candidate, ".workers.dev") || strings.Contains(candidate, ".pages.dev") {
			return candidate
		}
	}
	return ""
}
