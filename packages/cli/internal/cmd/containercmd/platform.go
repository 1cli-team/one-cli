package containercmd

// platform.go derives the --platform value for `docker build`.
// Two-step fallback:
//
//  1. workspace.ContainerPlatform(m) — the cached value persisted by a
//     previous successful build / deploy.
//  2. K8s node sniff via the resolved kustomize profile's kubeconfig —
//     when the workspace declares deploy/kustomize and `kubectl get
//     nodes` returns a uniform architecture, derive
//     linux/amd64 / linux/arm64 from it.
//
// This is the one place containercmd reaches across domains (into
// deploy/kustomize) by design — building for the wrong arch silently
// produces unrunnable images, and the kustomize-targeted deploy is
// the typical motivator for caring about platform at all.

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func resolveBuildPlatform(projectRoot string) string {
	m, err := workspace.ReadManifest(projectRoot)
	if err != nil {
		return ""
	}
	if platform := strings.TrimSpace(workspace.ContainerPlatform(m)); platform != "" {
		return platform
	}
	return detectK8sBuildPlatform(projectRoot, m)
}

func detectK8sBuildPlatform(projectRoot string, m *workspace.Manifest) string {
	if m == nil || m.Domains == nil || m.Domains.Deploy == nil {
		return ""
	}
	resolved, err := profile.Resolve(profile.ResolveInput{
		Domain:      profile.DomainDeploy,
		Backend:     workspace.DeployBackendKustomize,
		WorkspaceID: workspace.WorkspaceID(m),
	})
	if err != nil || resolved.Profile.Kustomize == nil {
		return ""
	}
	kp := resolved.Profile.Kustomize
	if strings.TrimSpace(kp.KubeconfigPath) == "" {
		return ""
	}
	return detectKubeNodePlatform(kp.KubeconfigPath, kp.KubeconfigContext)
}

func detectKubeNodePlatform(kubeconfigPath, kubeconfigContext string) string {
	args := []string{}
	if kubeconfigPath = strings.TrimSpace(kubeconfigPath); kubeconfigPath != "" {
		args = append(args, "--kubeconfig", kubeconfigPath)
	}
	if ctx := strings.TrimSpace(kubeconfigContext); ctx != "" {
		args = append(args, "--context", ctx)
	}
	args = append(args, "get", "nodes", "-o", `jsonpath={range .items[*]}{.status.nodeInfo.architecture}{"\n"}{end}`)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "kubectl", args...).Output()
	if err != nil {
		return ""
	}
	architectures := map[string]struct{}{}
	for _, line := range strings.Fields(string(out)) {
		architectures[strings.TrimSpace(line)] = struct{}{}
	}
	if len(architectures) != 1 {
		return ""
	}
	for arch := range architectures {
		switch arch {
		case "amd64":
			return "linux/amd64"
		case "arm64":
			return "linux/arm64"
		}
	}
	return ""
}
