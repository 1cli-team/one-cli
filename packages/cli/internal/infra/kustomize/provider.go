package kustomize

// provider.go is the kustomize backend's adapter to the deploy.Provider
// interface — registered at init() so deploycmd's dispatch table picks
// it up automatically. Apply orchestrates the kustomize-specific
// pre-deploy flow that used to live in deploycmd:
//   1. require a deploy/kustomize profile (kubeconfig + context)
//   2. if the project has container/docker enabled, build + push its
//      image (tag chosen interactively in TTY, --tag flag in CI)
//   3. sync the kustomize overlay so it points at the just-pushed image
//   4. run `kubectl apply -k <overlay>`
//
// The container pre-build lives here, not in deploycmd, because it's
// a kustomize concern: vercel/s3 don't need a container artifact.

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/container"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/deploy"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/docker"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/prompt"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// ProviderTag is the image tag chosen for the next deploy. deploycmd
// fills this from --tag (CI) or leaves empty (interactive prompt
// inside Apply selects it). Mutated only at command bootstrap time
// from cobra flag parsing — there is exactly one deploy job in flight
// at a time so a package-level variable is acceptable.
//
// Set/Reset are wired by deploycmd; never read this from outside.
var ProviderTag string

// providerImpl is the deploy.Provider for "kustomize". Lives here so
// kustomize's container pre-build, overlay sync, and kubectl apply all
// stay in one package.
type providerImpl struct{}

func (providerImpl) ID() string { return workspace.DeployBackendKustomize }

func (p providerImpl) Apply(ctx context.Context, in deploy.ApplyInput) (*deploy.ApplyResult, error) {
	if err := requireK8sDeployProfile(in.Resolved); err != nil {
		return nil, err
	}
	endpoint := endpointFromInput(in)

	// Container pre-build: only fires when the project has
	// container/docker enabled in the manifest. Sets the project's
	// container.image to the just-pushed reference so kustomize
	// overlay sync below can pick it up.
	if containerEnabledForProject(in.Manifest, in.Project.Name) {
		reg, err := resolveContainerRegistryForDeploy(in.ProjectRoot, in.Project.Name)
		if err != nil {
			return nil, err
		}
		platform, err := resolveBuildPlatformForDeploy(in.Manifest, endpoint, in.DryRun)
		if err != nil {
			return nil, err
		}
		imageTag, err := selectDeployImageTag(in.Stdout, in.Manifest, in.Project.Name, ProviderTag)
		if err != nil {
			return nil, err
		}
		if err := buildAndPushContainer(ctx, in, imageTag, reg, platform); err != nil {
			return nil, err
		}
		// Re-read manifest because buildAndPushContainer may have
		// updated projects[].container.image / .buildVersion.
		if !in.DryRun {
			m, _ := workspace.ReadManifest(in.ProjectRoot)
			in.Manifest = m
		}
	}

	if !in.DryRun {
		if err := syncOverlayTargetForApply(in.ProjectRoot, in.Manifest, endpoint); err != nil {
			return nil, err
		}
	}

	res, err := Apply(ctx, ApplyInput{
		ProjectRoot: in.ProjectRoot,
		Endpoint:    endpoint,
		DryRun:      in.DryRun,
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	return &deploy.ApplyResult{
		Schema:       res.Schema,
		Argv:         res.Argv,
		CommandLines: res.CommandLines,
		DryRun:       res.DryRun,
	}, nil
}

func init() {
	deploy.Register(providerImpl{})
}

// endpointFromInput collects the kustomize Endpoint from the resolved
// profile + manifest. The split: profile holds credentials / context
// (machine-level), manifest holds the deploy target (workspace-level
// kustomizationPath, namespace; per-project env name).
//
// Overlay path priority:
//  1. per-project `projects[i].deploy.kustomize.env` → `kustomize/overlays/<env>`
//  2. workspace `manifest.deploy.kustomizationPath`
//  3. package default (`kustomize/overlays/prod`, via ops.overlayPath)
func endpointFromInput(in deploy.ApplyInput) Endpoint {
	ep := Endpoint{
		Namespace:         workspace.DeployNamespace(in.Manifest),
		KustomizationPath: workspace.DeployKustomizationPath(in.Manifest),
	}
	if envName := envForProject(in.Manifest, in.Project.Name); envName != "" {
		ep.KustomizationPath = "kustomize/overlays/" + envName
	}
	if in.Resolved != nil && in.Resolved.Profile.Kustomize != nil {
		ep.KubeconfigPath = in.Resolved.Profile.Kustomize.KubeconfigPath
		ep.KubeconfigContext = in.Resolved.Profile.Kustomize.KubeconfigContext
	}
	return ep
}

// envForProject reads projects[i].domains.deploy.config.env. Empty when
// the manifest does not pin a value; callers then fall back to the
// workspace-level kustomizationPath or the package default.
func envForProject(m *workspace.Manifest, projectName string) string {
	cfg, _ := DecodeProjectConfig(m, projectName)
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.Env)
}

func requireK8sDeployProfile(resolved *profile.Resolved) error {
	if resolved != nil && resolved.Profile.Kustomize != nil {
		return nil
	}
	return cliErrors.New(cliErrors.PROFILE_NONE_CONFIGURED,
		"还没有配置 Kubernetes 部署目标。请先执行 `one configure add deploy/kustomize --profile <name> --use`。").
		WithRemediation(output.Remediation{
			Action:  "setup-k8s-deploy",
			Hint:    "选择 kubeconfig 和 context",
			Command: "one configure add deploy/kustomize --profile <name> --use",
		})
}

func containerEnabledForProject(m *workspace.Manifest, projectName string) bool {
	if m == nil {
		return false
	}
	enabled, _ := workspace.ContainerForProject(m, projectName)
	return enabled
}

// resolveContainerRegistryForDeploy fetches the per-project container
// registry endpoint by dispatching to docker.ResolveRegistry. The
// project's container.kind (manifest field, default "docker") picks
// which of the four kinds to resolve against.
func resolveContainerRegistryForDeploy(projectRoot, subproject string) (*container.Registry, error) {
	m, _ := workspace.ReadManifest(projectRoot)
	kind := workspace.ContainerKindForProject(m, subproject)
	return docker.ResolveRegistry(docker.ResolveRegistryInput{
		ProjectRoot:     projectRoot,
		Kind:            kind,
		Subproject:      subproject,
		RequireRegistry: true,
	})
}

func resolveBuildPlatformForDeploy(m *workspace.Manifest, endpoint Endpoint, allowCached bool) (string, error) {
	platform := detectKubeNodePlatform(endpoint.KubeconfigPath, endpoint.KubeconfigContext)
	if platform != "" {
		return platform, nil
	}
	if allowCached {
		if platform := strings.TrimSpace(workspace.ContainerPlatform(m)); platform != "" {
			return platform, nil
		}
	}
	cmdText := "kubectl get nodes -o wide"
	if endpoint.KubeconfigPath != "" {
		cmdText = "kubectl --kubeconfig " + endpoint.KubeconfigPath + " get nodes -o wide"
	}
	if endpoint.KubeconfigContext != "" {
		if endpoint.KubeconfigPath != "" {
			cmdText = "kubectl --kubeconfig " + endpoint.KubeconfigPath + " --context " + endpoint.KubeconfigContext + " get nodes -o wide"
		} else {
			cmdText = "kubectl --context " + endpoint.KubeconfigContext + " get nodes -o wide"
		}
	}
	return "", cliErrors.New(cliErrors.K8S_PLATFORM_UNDETECTED,
		"无法在构建镜像前检测 Kubernetes 节点架构。请先确认 kubeconfig/context/DNS 可用，再重新执行 `one deploy`。").
		WithContext(map[string]any{
			"kubeconfig": endpoint.KubeconfigPath,
			"context":    endpoint.KubeconfigContext,
		}).
		WithRemediation(output.Remediation{
			Action:  "check-k8s",
			Hint:    "确认当前部署目标可以访问，并能列出节点",
			Command: cmdText,
		})
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

func buildAndPushContainer(ctx context.Context, in deploy.ApplyInput, tag string, reg *container.Registry, platform string) error {
	root := in.ProjectRoot
	dryRun := in.DryRun
	stdout := in.Stdout
	if stdout == nil {
		stdout = io.Writer(os.Stdout)
	}
	kind := workspace.ContainerKindForProject(in.Manifest, in.Project.Name)
	provider, ok := container.Get(kind)
	if !ok {
		return cliErrors.New(cliErrors.CONTAINER_KIND_UNKNOWN,
			"container kind "+kind+" 未注册；检查 `projects[i].domains.container.kind` 值。")
	}
	buildRes, err := provider.Build(ctx, container.BuildInput{
		ProjectRoot: root,
		Project:     in.Project.Name,
		TargetNames: []string{in.Project.Name},
		Tag:         tag,
		Platform:    platform,
		DryRun:      dryRun,
		Registry:    reg,
	})
	if err != nil {
		return err
	}
	effectiveTag := strings.TrimSpace(tag)
	if buildRes != nil {
		for _, e := range buildRes.Built {
			if dryRun {
				fmt.Fprintln(stdout, strings.Join(e.Argv, " "))
				if effectiveTag == "" {
					effectiveTag = tagFromImageRef(e.Image)
				}
				continue
			}
			if err := workspace.SetProjectContainerImage(root, e.Project, e.Image); err != nil {
				return err
			}
			if err := workspace.SetProjectBuildVersion(root, e.Project, tagFromImageRef(e.Image)); err != nil {
				return err
			}
			if effectiveTag == "" {
				effectiveTag = tagFromImageRef(e.Image)
			}
		}
	}
	if platform != "" && !dryRun {
		if err := workspace.SetWorkspaceContainerPlatform(root, platform); err != nil {
			return err
		}
	}
	pushRes, err := provider.Push(ctx, container.PushInput{
		ProjectRoot: root,
		Project:     in.Project.Name,
		TargetNames: []string{in.Project.Name},
		Tag:         effectiveTag,
		DryRun:      dryRun,
		Registry:    reg,
	})
	if err != nil {
		return err
	}
	if pushRes != nil {
		for _, e := range pushRes.Pushed {
			if dryRun {
				fmt.Fprintln(stdout, strings.Join(e.Argv, " "))
				continue
			}
			if err := workspace.SetProjectContainerImage(root, e.Project, e.Image); err != nil {
				return err
			}
			if err := workspace.SetProjectBuildVersion(root, e.Project, tagFromImageRef(e.Image)); err != nil {
				return err
			}
		}
	}
	return nil
}

type semverTag struct {
	major int
	minor int
	patch int
}

// selectDeployImageTag picks the image tag for the next push. Honours
// the explicitTag argument first (CI / scripted callers), then prompts
// the user in TTY, then bails when neither is available.
func selectDeployImageTag(_ io.Writer, m *workspace.Manifest, subprojectName, explicitTag string) (string, error) {
	explicitTag = strings.TrimSpace(explicitTag)
	if explicitTag != "" {
		return normalizeVersionTag(explicitTag), nil
	}
	if !output.CanPrompt() {
		return "", nil
	}
	current, hasCurrent := currentImageSemverTag(m, subprojectName)
	if !hasCurrent {
		current = semverTag{}
	}
	patchTag := formatSemverTag(semverTag{major: current.major, minor: current.minor, patch: current.patch + 1})
	minorTag := formatSemverTag(semverTag{major: current.major, minor: current.minor + 1, patch: 0})
	majorTag := formatSemverTag(semverTag{major: current.major + 1, minor: 0, patch: 0})

	options := []prompt.Option[string]{}
	if hasCurrent {
		options = append(options,
			prompt.Option[string]{Label: "Current version " + formatSemverTag(current), Value: formatSemverTag(current)},
			prompt.Option[string]{Label: "Patch version " + patchTag, Value: patchTag},
			prompt.Option[string]{Label: "Minor version " + minorTag, Value: minorTag},
			prompt.Option[string]{Label: "Major version " + majorTag, Value: majorTag},
		)
	} else {
		options = append(options,
			prompt.Option[string]{Label: "Initial minor version " + minorTag, Value: minorTag},
			prompt.Option[string]{Label: "Major version " + majorTag, Value: majorTag},
			prompt.Option[string]{Label: "Patch version " + patchTag, Value: patchTag},
		)
	}
	options = append(options, prompt.Option[string]{Label: "Custom version", Value: "__custom__"})

	selected, err := prompt.Select("Select image version", options)
	if err != nil {
		return "", err
	}
	if selected != "__custom__" {
		return selected, nil
	}
	placeholder := minorTag
	if hasCurrent {
		placeholder = patchTag
	}
	custom, err := prompt.Text("Image version", placeholder, func(value string) error {
		if _, ok := parseSemverTag(value); !ok {
			return fmt.Errorf("enter a semver version, e.g. v0.1.0")
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return normalizeVersionTag(custom), nil
}

func currentImageSemverTag(m *workspace.Manifest, subprojectName string) (semverTag, bool) {
	if m == nil {
		return semverTag{}, false
	}
	if version := workspace.BuildVersionForProject(m, subprojectName); version != "" {
		if tag, ok := parseSemverTag(version); ok {
			return tag, true
		}
	}
	for _, sub := range m.Projects {
		if sub.Name != subprojectName {
			continue
		}
		if sub.Domains == nil || sub.Domains.Container == nil {
			continue
		}
		if tag, ok := parseSemverTag(tagFromImageRef(sub.Domains.Container.Image)); ok {
			return tag, true
		}
	}
	if tag, ok := parseSemverTag(workspace.DefaultBuildVersion); ok {
		return tag, true
	}
	return semverTag{}, false
}

func parseSemverTag(value string) (semverTag, bool) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(strings.TrimPrefix(value, "v"), "V")
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return semverTag{}, false
	}
	nums := make([]int, 3)
	for i, part := range parts {
		if part == "" {
			return semverTag{}, false
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return semverTag{}, false
		}
		nums[i] = n
	}
	return semverTag{major: nums[0], minor: nums[1], patch: nums[2]}, true
}

func normalizeVersionTag(value string) string {
	parsed, ok := parseSemverTag(value)
	if !ok {
		return strings.TrimSpace(value)
	}
	return formatSemverTag(parsed)
}

func formatSemverTag(v semverTag) string {
	return fmt.Sprintf("v%d.%d.%d", v.major, v.minor, v.patch)
}

func tagFromImageRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	idx := strings.LastIndex(ref, ":")
	slash := strings.LastIndex(ref, "/")
	if idx > slash {
		return ref[idx+1:]
	}
	return ""
}

// syncOverlayTargetForApply rewrites the prod overlay's image tags to
// match the just-pushed images. Called only when the workspace runs at
// least one kustomize-backed project.
func syncOverlayTargetForApply(root string, m *workspace.Manifest, endpoint Endpoint) error {
	images := map[string]string{}
	if m != nil {
		for _, sub := range m.Projects {
			if sub.Domains == nil {
				continue
			}
			if sub.Domains.Deploy == nil || sub.Domains.Deploy.Kind != workspace.DeployBackendKustomize {
				continue
			}
			if sub.Domains.Container == nil || strings.TrimSpace(sub.Domains.Container.Image) == "" {
				continue
			}
			images[sub.Name] = sub.Domains.Container.Image
		}
	}
	return SyncOverlayTarget(root, endpoint.KustomizationPath, endpoint.Namespace, images)
}
