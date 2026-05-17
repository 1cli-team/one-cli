package kustomize

// ops.go exposes the Render / Apply operations as ordinary package
// functions with package-local types. The plugin wrapper in verbs.go
// adapts the pkg/plugin shapes onto these.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
)

// Endpoint is the kustomize-relevant slice of a deploy profile:
// kubeconfig path/context + namespace + (optional) kustomization path
// override. Empty zero value is valid (defaults apply).
type Endpoint struct {
	KubeconfigPath    string
	KubeconfigContext string
	Namespace         string
	KustomizationPath string
}

// Stable JSON envelope schema string for the kustomize apply operation.
const SchemaApply = "one-cli/deploy-apply/v1"

// ApplyInput addresses Apply.
type ApplyInput struct {
	ProjectRoot string
	Endpoint    Endpoint
	DryRun      bool
}

// ApplyResult is the Apply envelope.
type ApplyResult struct {
	Schema       string   `json:"schema"`
	Argv         []string `json:"argv"`
	CommandLines []string `json:"command_lines,omitempty"`
	DryRun       bool     `json:"dry_run"`
}

// Apply runs `kubectl apply -k <overlay>` with --context / --namespace
// from the resolved profile. DryRun returns the argv without executing
// kubectl, so it is safe on machines without cluster access.
func Apply(ctx context.Context, in ApplyInput) (*ApplyResult, error) {
	overlay := overlayPath(in.Endpoint, in.ProjectRoot)
	argv := []string{"kubectl", "apply", "-k", overlay}
	argv = append(argv, kubectlGlobalArgs(in.Endpoint)...)
	if ns := strings.TrimSpace(in.Endpoint.Namespace); ns != "" {
		argv = append(argv, "--namespace", ns)
	}
	if in.DryRun {
		argv = append(argv, "--dry-run=client")
		return &ApplyResult{
			Schema:       SchemaApply,
			Argv:         argv,
			CommandLines: dryRunCommandLines(in.Endpoint, argv),
			DryRun:       true,
		}, nil
	}
	if _, err := exec.LookPath("kubectl"); err != nil {
		return nil, missingKubectlErr()
	}
	if err := ensureNamespace(ctx, in.Endpoint); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, cliErrors.New(cliErrors.ONE_CLI_ERROR,
			fmt.Sprintf("`%s` 失败：%v", strings.Join(argv, " "), err))
	}
	return &ApplyResult{Schema: SchemaApply, Argv: argv, DryRun: in.DryRun}, nil
}

func dryRunCommandLines(ep Endpoint, applyArgs []string) []string {
	lines := []string{}
	if ns := strings.TrimSpace(ep.Namespace); ns != "" {
		lines = append(lines,
			kubectlCommandLine(namespaceCreateArgs(ep, ns))+" | "+kubectlCommandLine(namespaceApplyArgs(ep)),
		)
	}
	lines = append(lines, strings.Join(applyArgs, " "))
	return lines
}

func kubectlCommandLine(args []string) string {
	if len(args) == 0 {
		return "kubectl"
	}
	return "kubectl " + strings.Join(args, " ")
}

func kubectlGlobalArgs(ep Endpoint) []string {
	args := []string{}
	if p := strings.TrimSpace(ep.KubeconfigPath); p != "" {
		args = append(args, "--kubeconfig", p)
	}
	if c := strings.TrimSpace(ep.KubeconfigContext); c != "" {
		args = append(args, "--context", c)
	}
	return args
}

func ensureNamespace(ctx context.Context, ep Endpoint) error {
	ns := strings.TrimSpace(ep.Namespace)
	if ns == "" {
		return nil
	}
	createArgs := namespaceCreateArgs(ep, ns)
	createCmd := exec.CommandContext(ctx, "kubectl", createArgs...)
	var manifest bytes.Buffer
	var createErr bytes.Buffer
	createCmd.Stdout = &manifest
	createCmd.Stderr = &createErr
	if err := createCmd.Run(); err != nil {
		msg := strings.TrimSpace(createErr.String())
		if msg == "" {
			msg = err.Error()
		}
		return cliErrors.New(cliErrors.ONE_CLI_ERROR,
			fmt.Sprintf("`kubectl %s` 失败：%s", strings.Join(createArgs, " "), msg))
	}

	applyArgs := namespaceApplyArgs(ep)
	applyCmd := exec.CommandContext(ctx, "kubectl", applyArgs...)
	applyCmd.Stdin = &manifest
	applyCmd.Stdout = os.Stdout
	var applyErr bytes.Buffer
	applyCmd.Stderr = &applyErr
	if err := applyCmd.Run(); err != nil {
		msg := strings.TrimSpace(applyErr.String())
		if msg == "" {
			msg = err.Error()
		}
		return cliErrors.New(cliErrors.ONE_CLI_ERROR,
			fmt.Sprintf("`kubectl %s` 失败：%s", strings.Join(applyArgs, " "), msg))
	}
	return nil
}

func namespaceCreateArgs(ep Endpoint, ns string) []string {
	args := append([]string{}, kubectlGlobalArgs(ep)...)
	return append(args, "create", "namespace", ns, "--dry-run=client", "-o", "yaml")
}

func namespaceApplyArgs(ep Endpoint) []string {
	args := append([]string{}, kubectlGlobalArgs(ep)...)
	return append(args, "apply", "-f", "-")
}

// overlayPath resolves which kustomize overlay to apply. Profile's
// KustomizationPath wins; otherwise the prod overlay convention.
// Path is interpreted relative to projectRoot when not absolute.
func overlayPath(ep Endpoint, projectRoot string) string {
	path := defaultOverlay
	if strings.TrimSpace(ep.KustomizationPath) != "" {
		path = ep.KustomizationPath
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(projectRoot, filepath.FromSlash(path))
}

func missingKubectlErr() error {
	return cliErrors.New(cliErrors.RUN_COMMAND_NOT_FOUND,
		"未在 PATH 中找到 kubectl。装一个：brew install kubectl（macOS），或参考 https://kubernetes.io/docs/tasks/tools/")
}
