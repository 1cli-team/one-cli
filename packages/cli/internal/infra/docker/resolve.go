package docker

// resolve.go folds a machine-level container/<kind> profile + the
// per-project manifest overrides into a single container.Registry the
// build / push helpers can consume. This is the only file in the
// docker package that knows the four kinds (docker / dockerhub / ghcr
// / acr) differ at all — Build / Push / Info treat the resolved
// Registry uniformly.

import (
	"fmt"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/infra/container"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// ResolveRegistryInput addresses ResolveRegistry. RequireRegistry asks
// for an error when no profile is configured (push / kustomize need
// the registry; bare `one container build` for local-only use can
// tolerate a nil result and skip the prefix).
type ResolveRegistryInput struct {
	ProjectRoot     string
	Kind            string
	ProfileFlag     string
	Subproject      string
	RequireRegistry bool
	SkipDefault     bool
}

// ResolveRegistry returns the populated container.Registry for one
// (kind, subproject) pair, or nil when RequireRegistry=false and no
// profile is configured (legacy `one container build` falls back to
// local-only mode in that case). On RequireRegistry=true a missing
// profile yields cliErrors.REGISTRY_CREDENTIAL_MISSING with a
// kind-specific remediation.
//
// Each kind's host / namespace derivation is the only real difference
// between the four backends; everything else (Build / Push argv,
// docker login, image-tag composition) is shared.
func ResolveRegistry(in ResolveRegistryInput) (*container.Registry, error) {
	kind := strings.TrimSpace(in.Kind)
	if kind == "" {
		kind = "docker"
	}
	if !profile.IsContainerKind(kind) {
		return nil, cliErrors.New(cliErrors.CONTAINER_KIND_UNKNOWN,
			fmt.Sprintf("不认识的 container kind %q；支持的 kind：%s",
				kind, strings.Join(profile.ContainerKinds(), " / "))).
			WithContext(map[string]any{
				"kind":            kind,
				"supported_kinds": profile.ContainerKinds(),
			})
	}

	workspaceID := ""
	if m, err := workspace.ReadManifest(in.ProjectRoot); err == nil {
		workspaceID = workspace.WorkspaceID(m)
	}

	resolved, err := profile.Resolve(profile.ResolveInput{
		Domain:       profile.DomainContainer,
		Backend:      kind,
		FlagOverride: in.ProfileFlag,
		WorkspaceID:  workspaceID,
		ProjectName:  in.Subproject,
		SkipDefault:  in.SkipDefault,
	})
	if err != nil {
		if cliErr, ok := err.(interface{ ErrorCode() string }); ok &&
			cliErr.ErrorCode() == "PROFILE_NONE_CONFIGURED" {
			if !in.RequireRegistry {
				return nil, nil
			}
			return nil, profileNotConfiguredErr(kind, in.Subproject)
		}
		return nil, err
	}
	if resolved.Profile.Container == nil {
		return nil, profileInvalidErr(kind, "container profile missing")
	}
	cp := resolved.Profile.Container

	host, err := hostForKind(kind, cp)
	if err != nil {
		return nil, err
	}

	username := ""
	password := ""
	if cp.Credentials != nil {
		username = cp.Credentials.Username
		password = cp.Credentials.Password
	}
	namespace := strings.TrimSpace(cp.Namespace)
	if namespace == "" {
		namespace = defaultNamespaceForKind(kind, cp.Registry, username)
	}
	return &container.Registry{
		Registry:      host,
		Namespace:     namespace,
		Username:      username,
		Password:      password,
		ProfileName:   resolved.Name,
		ProfileSource: resolved.Source,
	}, nil
}

// hostForKind derives the registry host string for one kind. docker
// reads it straight from the profile; dockerhub / ghcr have fixed
// hosts; acr (Aliyun) derives from Region.
func hostForKind(kind string, cp *profile.ContainerProfile) (string, error) {
	switch kind {
	case "docker":
		host := strings.TrimSpace(cp.Registry)
		if host == "" {
			return "", profileInvalidErr(kind, "container/docker profile 缺 registry 字段")
		}
		// Strip an accidental scheme — docker push wants bare host.
		host = strings.TrimPrefix(host, "https://")
		host = strings.TrimPrefix(host, "http://")
		return host, nil
	case "dockerhub":
		return "index.docker.io", nil
	case "ghcr":
		return "ghcr.io", nil
	case "acr":
		region := strings.TrimSpace(cp.Region)
		if region == "" {
			return "", profileInvalidErr(kind, "container/acr profile 缺 region 字段（host 派生自 registry.<region>.aliyuncs.com）")
		}
		return "registry." + region + ".aliyuncs.com", nil
	}
	return "", cliErrors.New(cliErrors.CONTAINER_KIND_UNKNOWN,
		fmt.Sprintf("hostForKind: unhandled kind %q", kind))
}

// defaultNamespaceForKind returns the namespace fallback when neither
// the manifest nor the profile set one. dockerhub / ghcr conventionally
// use username as the namespace prefix; acr requires explicit; the
// docker (generic) kind reuses the legacy host-based heuristic from
// configurecmd / kustomize.
func defaultNamespaceForKind(kind, registry, username string) string {
	switch kind {
	case "dockerhub", "ghcr":
		return strings.TrimSpace(username)
	case "acr":
		return ""
	case "docker":
		return defaultRegistryNamespace(registry, username)
	}
	return ""
}

// defaultRegistryNamespace is the historical heuristic for the docker
// (generic) kind: when the user pointed the profile at a known shared
// registry that uses owner/image naming (ghcr.io / docker.io), default
// the namespace to the configured username. Anything else gets an
// empty namespace so the image lives at <host>/<workload>:<tag>.
//
// Living here so containercmd / kustomize can delete their duplicate
// copies in Phase F / G; the canonical source of truth is now
// docker.ResolveRegistry.
func defaultRegistryNamespace(registry, username string) string {
	registry = strings.TrimSpace(strings.TrimPrefix(registry, "https://"))
	registry = strings.TrimPrefix(registry, "http://")
	username = strings.TrimSpace(username)
	switch registry {
	case "ghcr.io", "docker.io", "index.docker.io":
		return username
	}
	return ""
}

func profileNotConfiguredErr(kind, subproject string) error {
	setupCommand := "one configure add container/" + kind + " --profile <name> --use"
	return cliErrors.New(cliErrors.REGISTRY_CREDENTIAL_MISSING,
		fmt.Sprintf("container/%s 还没有配置 profile。先执行 `%s`。", kind, setupCommand)).
		WithContext(map[string]any{
			"kind":       kind,
			"subproject": strings.TrimSpace(subproject),
		}).
		WithRemediation(output.Remediation{
			Action:  "setup-registry",
			Hint:    "配置镜像仓库后再构建或推送",
			Command: setupCommand,
		})
}

func profileInvalidErr(kind, detail string) error {
	setupCommand := "one configure add container/" + kind + " --profile <name> --use"
	return cliErrors.New(cliErrors.CONTAINER_PROFILE_INVALID,
		fmt.Sprintf("container/%s profile 不完整：%s。请重新执行 `%s`。", kind, detail, setupCommand)).
		WithContext(map[string]any{
			"kind":   kind,
			"detail": detail,
		}).
		WithRemediation(output.Remediation{
			Action:  "reconfigure-container",
			Hint:    "重新配置 container profile",
			Command: setupCommand,
		})
}
