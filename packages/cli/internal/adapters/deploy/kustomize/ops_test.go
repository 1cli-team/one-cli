package kustomize

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyEnsuresNamespaceBeforeKustomizeApply(t *testing.T) {
	tmp := t.TempDir()
	overlay := filepath.Join(tmp, "kustomize", "overlays", "prod")
	if err := os.MkdirAll(overlay, 0o755); err != nil {
		t.Fatalf("mkdir overlay: %v", err)
	}
	logPath := filepath.Join(tmp, "kubectl.log")
	installApplyFakeKubectl(t, tmp, logPath)

	_, err := Apply(context.Background(), ApplyInput{
		ProjectRoot: tmp,
		Endpoint: Endpoint{
			KubeconfigPath:    filepath.Join(tmp, "kubeconfig.yaml"),
			KubeconfigContext: "local",
			Namespace:         "demo-123",
			KustomizationPath: "kustomize/overlays/prod",
		},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake kubectl log: %v", err)
	}
	got := string(raw)
	wants := []string{
		"--kubeconfig " + filepath.Join(tmp, "kubeconfig.yaml") + " --context local create namespace demo-123 --dry-run=client -o yaml",
		"--kubeconfig " + filepath.Join(tmp, "kubeconfig.yaml") + " --context local apply -f -",
		"apply -k " + overlay + " --kubeconfig " + filepath.Join(tmp, "kubeconfig.yaml") + " --context local --namespace demo-123",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("kubectl log missing %q:\n%s", want, got)
		}
	}
}

func TestApplyAcceptsStagingOverlay(t *testing.T) {
	tmp := t.TempDir()
	overlay := filepath.Join(tmp, "kustomize", "overlays", "staging")
	if err := os.MkdirAll(overlay, 0o755); err != nil {
		t.Fatalf("mkdir overlay: %v", err)
	}
	logPath := filepath.Join(tmp, "kubectl.log")
	installApplyFakeKubectl(t, tmp, logPath)

	_, err := Apply(context.Background(), ApplyInput{
		ProjectRoot: tmp,
		Endpoint: Endpoint{
			KubeconfigPath:    filepath.Join(tmp, "kubeconfig.yaml"),
			KubeconfigContext: "local",
			Namespace:         "demo-123",
			KustomizationPath: "kustomize/overlays/staging",
		},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake kubectl log: %v", err)
	}
	got := string(raw)
	if !strings.Contains(got, "apply -k "+overlay) {
		t.Fatalf("kubectl log missing staging overlay apply:\n%s", got)
	}
}

func TestApplyDryRunIncludesNamespaceEnsureCommandLines(t *testing.T) {
	tmp := t.TempDir()
	overlay := filepath.Join(tmp, "kustomize", "overlays", "prod")
	if err := os.MkdirAll(overlay, 0o755); err != nil {
		t.Fatalf("mkdir overlay: %v", err)
	}
	kubeconfig := filepath.Join(tmp, "kubeconfig.yaml")

	res, err := Apply(context.Background(), ApplyInput{
		ProjectRoot: tmp,
		Endpoint: Endpoint{
			KubeconfigPath:    kubeconfig,
			KubeconfigContext: "local",
			Namespace:         "demo-123",
			KustomizationPath: "kustomize/overlays/prod",
		},
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("Apply dry-run: %v", err)
	}

	wants := []string{
		"kubectl --kubeconfig " + kubeconfig + " --context local create namespace demo-123 --dry-run=client -o yaml | kubectl --kubeconfig " + kubeconfig + " --context local apply -f -",
		"kubectl apply -k " + overlay + " --kubeconfig " + kubeconfig + " --context local --namespace demo-123 --dry-run=client",
	}
	if len(res.CommandLines) != len(wants) {
		t.Fatalf("command lines = %#v, want %d lines", res.CommandLines, len(wants))
	}
	for i, want := range wants {
		if res.CommandLines[i] != want {
			t.Fatalf("command line %d = %q, want %q", i, res.CommandLines[i], want)
		}
	}
}

func installApplyFakeKubectl(t *testing.T, dir, logPath string) {
	t.Helper()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	path := filepath.Join(bin, "kubectl")
	body := `#!/bin/sh
echo "$@" >> "$KUBECTL_LOG"
case "$*" in
  *" create namespace "*)
    printf 'apiVersion: v1\nkind: Namespace\nmetadata:\n  name: demo-123\n'
    ;;
  *" apply -f -"*)
    cat >/dev/null
    printf 'namespace/demo-123 configured\n'
    ;;
  *"apply -k "*)
    printf 'deployment.apps/api configured\n'
    ;;
  *)
    printf 'unexpected kubectl %s\n' "$*" >&2
    exit 1
    ;;
esac
`
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake kubectl: %v", err)
	}
	t.Setenv("KUBECTL_LOG", logPath)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}
