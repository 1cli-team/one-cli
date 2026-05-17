package configurecmd

// builders.go houses the per-backend profile-construction helpers used
// by `one configure add <domain>/<backend>`. Each Build* function returns
// a typed sub-profile, optionally prompting the user for missing fields
// when interactive=true. They share the same flag-or-prompt pattern:
// any field already supplied via flag stays; blank fields fall back to
// a prompt; non-interactive callers must supply every required field
// via flag or get PROFILE_BACKEND_INVALID.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/prompt"
	"gopkg.in/yaml.v3"
)

// BuildInfisicalProfile fills an InfisicalProfile from flags + optional
// interactive prompts. Machine-level fields only (SiteURL + Universal
// Auth credentials); workspace-level fields (projectId, environments,
// rootPath) live in one.manifest.json#env.
func BuildInfisicalProfile(
	interactive bool,
	siteURL, clientID, clientSecret string,
) (*profile.InfisicalProfile, error) {
	if interactive {
		if siteURL == "" {
			v, err := prompt.Text("Infisical site URL", "https://app.infisical.com", nil)
			if err != nil {
				return nil, err
			}
			siteURL = strings.TrimSpace(v)
		}
		if clientID == "" {
			v, err := prompt.Text("Universal Auth client ID", "", requireNonEmpty)
			if err != nil {
				return nil, err
			}
			clientID = strings.TrimSpace(v)
		}
		if clientSecret == "" {
			v, err := prompt.Password("Universal Auth client secret", requireNonEmpty)
			if err != nil {
				return nil, err
			}
			clientSecret = strings.TrimSpace(v)
		}
	}
	if clientID == "" || clientSecret == "" {
		return nil, cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			"非交互模式下 --client-id / --client-secret 都必须提供。")
	}
	if siteURL == "" {
		siteURL = "https://app.infisical.com"
	}
	return &profile.InfisicalProfile{
		SiteURL: siteURL,
		Credentials: &profile.InfisicalCredentials{
			ClientID:     clientID,
			ClientSecret: clientSecret,
		},
	}, nil
}

// BuildKustomizeProfile fills the kubeconfig path + context used by
// deploy/kustomize. The kubeconfig file carries cluster endpoint +
// credentials; the profile only points at that file and selects one
// context from it. Workspace-scoped fields (explicit k8s namespace override,
// kustomization base path) live in manifest.deploy; empty namespace falls
// back to project.id.
func BuildKustomizeProfile(interactive bool, kubeconfigPath, kubeconfigContext string) (*profile.KustomizeProfile, error) {
	kp := &profile.KustomizeProfile{
		KubeconfigPath:    strings.TrimSpace(kubeconfigPath),
		KubeconfigContext: strings.TrimSpace(kubeconfigContext),
	}
	if interactive {
		if err := PromptKustomize(kp); err != nil {
			return nil, err
		}
		return kp, nil
	}
	if kp.KubeconfigPath == "" {
		kp.KubeconfigPath = defaultKubeconfigPath()
	}
	kp.KubeconfigPath = expandPath(kp.KubeconfigPath)
	if kp.KubeconfigPath == "" {
		return nil, cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			"非交互模式下 kubeconfig path 不能为空。")
	}
	ctx, err := resolveKubeconfigContext(kp.KubeconfigPath, kp.KubeconfigContext)
	if err != nil {
		return nil, err
	}
	kp.KubeconfigContext = ctx
	return kp, nil
}

// PromptKustomize fills the kubeconfig path and context in interactive
// mode, defaulting the path to ~/.kube/config and preferring the
// kubeconfig current-context.
func PromptKustomize(p *profile.KustomizeProfile) error {
	if p.KubeconfigPath == "" {
		p.KubeconfigPath = defaultKubeconfigPath()
	}
	if err := prompt.NewForm().
		Text(&p.KubeconfigPath, "kubeconfig 文件路径", defaultKubeconfigPath(), requireNonEmpty).
		Run(); err != nil {
		return err
	}
	p.KubeconfigPath = expandPath(p.KubeconfigPath)
	contexts, current, err := readKubeconfigContexts(p.KubeconfigPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(p.KubeconfigContext) == "" {
		switch len(contexts) {
		case 0:
			return cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
				fmt.Sprintf("kubeconfig %s 没有 contexts。", p.KubeconfigPath))
		case 1:
			p.KubeconfigContext = contexts[0]
		default:
			ordered := contextsWithCurrentFirst(contexts, current)
			options := make([]prompt.Option[string], 0, len(ordered))
			for _, ctx := range ordered {
				label := ctx
				if ctx == current {
					label += " (current)"
				}
				options = append(options, prompt.Option[string]{Label: label, Value: ctx})
			}
			selected, err := prompt.Select("选择 Kubernetes context", options)
			if err != nil {
				return err
			}
			p.KubeconfigContext = selected
		}
	}
	ctx, err := resolveKubeconfigContext(p.KubeconfigPath, p.KubeconfigContext)
	if err != nil {
		return err
	}
	p.KubeconfigContext = ctx
	return nil
}

// BuildS3Profile fills an S3Profile from flags + optional interactive
// prompts. Single helper covers all six S3-API-compatible deploy
// backends (deploy/aliyun-oss / tencent-cos / aws-s3 / minio / rustfs /
// r2) — they share the same profile schema, only differing in defaults
// and prompt copy. `kind` selects those defaults; an empty kind falls
// back to generic prompts.
//
// The bucket is per-project (projects[i].deploy.bucket); see
// `one add` for where it gets prompted.
func BuildS3Profile(
	interactive bool, kind string,
	endpoint, region string, forcePathStyle bool,
	accessKeyID, accessKeySecret string,
) (*profile.S3Profile, error) {
	endpointPrompt, regionPrompt, regionDefault := s3CompatPrompts(kind)
	if interactive {
		if endpoint == "" {
			v, err := prompt.Text(endpointPrompt, "", nil)
			if err != nil {
				return nil, err
			}
			endpoint = strings.TrimSpace(v)
		}
		if region == "" {
			v, err := prompt.Text(regionPrompt, regionDefault, nil)
			if err != nil {
				return nil, err
			}
			region = strings.TrimSpace(v)
			if region == "" {
				region = regionDefault
			}
		}
		if accessKeyID == "" {
			v, err := prompt.Text("AccessKey ID", "", requireNonEmpty)
			if err != nil {
				return nil, err
			}
			accessKeyID = strings.TrimSpace(v)
		}
		if accessKeySecret == "" {
			v, err := prompt.Password("AccessKey Secret", requireNonEmpty)
			if err != nil {
				return nil, err
			}
			accessKeySecret = strings.TrimSpace(v)
		}
	}
	if accessKeyID == "" || accessKeySecret == "" {
		return nil, cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			"非交互模式下 --access-key-id / --access-key-secret 必须提供。")
	}
	return &profile.S3Profile{
		Endpoint:       endpoint,
		Region:         region,
		ForcePathStyle: forcePathStyle,
		Credentials: &profile.S3Credentials{
			AccessKeyID:     accessKeyID,
			AccessKeySecret: accessKeySecret,
		},
	}, nil
}

// s3CompatPrompts returns kind-specific (endpoint prompt, region
// prompt, region default) hints. No concrete vendor hostnames or
// account ids — just placeholders the user fills in.
func s3CompatPrompts(kind string) (endpointPrompt, regionPrompt, regionDefault string) {
	switch kind {
	case "aliyun-oss":
		return "S3 endpoint URL（如 https://oss-<region>.aliyuncs.com）",
			"Region（如 cn-hangzhou / cn-shanghai）", "cn-hangzhou"
	case "tencent-cos":
		return "S3 endpoint URL（如 https://cos.<region>.myqcloud.com）",
			"Region（如 ap-guangzhou / ap-shanghai）", "ap-guangzhou"
	case "aws-s3":
		return "S3 endpoint URL（留空 = AWS SDK 默认）",
			"Region（如 us-east-1 / ap-northeast-1）", "us-east-1"
	case "minio":
		return "MinIO endpoint URL（如 http://<your-minio-host>:9000）",
			"Region（任意值，常用 us-east-1 占位）", "us-east-1"
	case "rustfs":
		return "RustFS endpoint URL（如 http://<your-rustfs-host>:9000）",
			"Region（任意值，常用 us-east-1 占位）", "us-east-1"
	case "r2":
		return "R2 endpoint URL（如 https://<account-id>.r2.cloudflarestorage.com）",
			"Region（通常 auto）", "auto"
	}
	return "S3 endpoint URL", "Region", "us-east-1"
}

// BuildContainerProfile fills a ContainerProfile for one of the four
// container backend kinds (docker / dockerhub / ghcr / acr) from flags
// + optional interactive prompts. All four kinds share the same
// `ContainerProfile` shape — host is either user-supplied (docker) or
// derived by infra/docker.ResolveRegistry from the Region (acr) or a
// fixed constant (dockerhub / ghcr). Region is only meaningful for
// kind=="acr" (Aliyun ACR).
//
// Namespace is the default registry namespace / owner for this
// machine-level profile. A subproject may still override it with
// projects[i].container.namespace.
func BuildContainerProfile(
	interactive bool,
	kind, registry, region, namespace, username, password string,
) (*profile.ContainerProfile, error) {
	if interactive {
		switch kind {
		case "docker":
			if registry == "" {
				v, err := prompt.Text(
					"Registry host（如 your-harbor.example.com / Aliyun ACR / TCR / SWR / Quay 私有地址）",
					"", requireNonEmpty)
				if err != nil {
					return nil, err
				}
				registry = strings.TrimSpace(v)
			}
		case "acr":
			if region == "" {
				v, err := prompt.Text(
					"Aliyun ACR region（如 cn-hangzhou / cn-shanghai；host 会派生为 registry.<region>.aliyuncs.com）",
					"cn-hangzhou", requireNonEmpty)
				if err != nil {
					return nil, err
				}
				region = strings.TrimSpace(v)
			}
		}
		if username == "" {
			v, err := prompt.Text("Username（登录名 / RAM AKID / robot 账号 / GitHub username）", "", requireNonEmpty)
			if err != nil {
				return nil, err
			}
			username = strings.TrimSpace(v)
		}
		if namespace == "" {
			defaultNamespace := defaultContainerNamespace(kind, registry, username)
			v, err := prompt.Text("Registry namespace（org / team / GitHub user，可空）", defaultNamespace, nil)
			if err != nil {
				return nil, err
			}
			namespace = strings.TrimSpace(v)
			if namespace == "" {
				namespace = defaultNamespace
			}
		}
		if password == "" {
			v, err := prompt.Password("Password / token / PAT", requireNonEmpty)
			if err != nil {
				return nil, err
			}
			password = strings.TrimSpace(v)
		}
	}
	// Per-kind required-field validation. Errors here surface in
	// non-interactive (CI) mode where the user passed --kind X but
	// forgot the kind's mandatory fields.
	switch kind {
	case "docker":
		if registry == "" {
			return nil, cliErrors.New(cliErrors.CONTAINER_PROFILE_INVALID,
				"container/docker 需要 --registry / --username / --password 都填好。")
		}
	case "acr":
		if region == "" {
			return nil, cliErrors.New(cliErrors.CONTAINER_PROFILE_INVALID,
				"container/acr 需要 --region 指定 Aliyun ACR 区域（host 会自动派生为 registry.<region>.aliyuncs.com）。")
		}
	}
	if username == "" || password == "" {
		return nil, cliErrors.New(cliErrors.CONTAINER_PROFILE_INVALID,
			fmt.Sprintf("container/%s 需要 --username / --password 都填好。", kind))
	}
	if namespace == "" {
		namespace = defaultContainerNamespace(kind, registry, username)
	}
	return &profile.ContainerProfile{
		Registry:  registry,
		Region:    region,
		Namespace: namespace,
		Credentials: &profile.ContainerCredentials{
			Username: username,
			Password: password,
		},
	}, nil
}

// defaultContainerNamespace returns the namespace prompt default per
// container kind. Docker Hub / GHCR conventionally namespace under the
// username; Aliyun ACR requires the user to fill in their own
// namespace (no sensible auto-default); generic docker reuses the old
// host-based heuristic.
func defaultContainerNamespace(kind, registry, username string) string {
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

// BuildDockerProfile is kept as a thin wrapper around BuildContainerProfile
// for the kind="docker" case so existing callers don't need to thread
// kind through. New call sites should use BuildContainerProfile with an
// explicit kind.
func BuildDockerProfile(
	interactive bool,
	registry, namespace, username, password string,
) (*profile.ContainerProfile, error) {
	return BuildContainerProfile(interactive, "docker", registry, "", namespace, username, password)
}

// BuildVercelProfile fills a VercelProfile from flags + optional
// interactive prompts. Token is the only required field; team is the
// optional org slug (corresponds to vercel CLI's --scope flag — leave
// blank to use the token's personal scope).
func BuildVercelProfile(
	interactive bool,
	apiToken, team string,
) (*profile.VercelProfile, error) {
	apiToken = strings.TrimSpace(apiToken)
	team = strings.TrimSpace(team)
	if interactive {
		if apiToken == "" {
			v, err := prompt.Password("Vercel API token（vercel.com → Account Settings → Tokens）", requireNonEmpty)
			if err != nil {
				return nil, err
			}
			apiToken = strings.TrimSpace(v)
		}
		if team == "" {
			v, err := prompt.Text("Team / org slug（可选，留空 = personal scope）", "", nil)
			if err != nil {
				return nil, err
			}
			team = strings.TrimSpace(v)
		}
	}
	if apiToken == "" {
		return nil, cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			"非交互模式下必须提供 --token。")
	}
	return &profile.VercelProfile{
		Team:        team,
		Credentials: &profile.VercelCredentials{APIToken: apiToken},
	}, nil
}

// BuildCloudflareProfile fills a CloudflareProfile from flags +
// optional interactive prompts. Token is the only required field;
// accountID is optional (wrangler can infer it from a single-account
// token, but multi-account tokens require an explicit Account ID).
func BuildCloudflareProfile(
	interactive bool,
	apiToken, accountID string,
) (*profile.CloudflareProfile, error) {
	apiToken = strings.TrimSpace(apiToken)
	accountID = strings.TrimSpace(accountID)
	if interactive {
		if apiToken == "" {
			v, err := prompt.Password("Cloudflare API token（dash.cloudflare.com → My Profile → API Tokens）", requireNonEmpty)
			if err != nil {
				return nil, err
			}
			apiToken = strings.TrimSpace(v)
		}
		if accountID == "" {
			v, err := prompt.Text("Account ID（可选，多账号 token 必填；留空 = 单账号 token）", "", nil)
			if err != nil {
				return nil, err
			}
			accountID = strings.TrimSpace(v)
		}
	}
	if apiToken == "" {
		return nil, cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			"非交互模式下必须提供 --token。")
	}
	return &profile.CloudflareProfile{
		AccountID:   accountID,
		Credentials: &profile.CloudflareCredentials{APIToken: apiToken},
	}, nil
}

// BuildEdgeOneProfile fills an EdgeOneProfile from flags + optional
// interactive prompts. API token is optional for local machines that
// authenticate with `edgeone login`; CI should configure one.
func BuildEdgeOneProfile(
	interactive bool,
	apiToken, region string,
) (*profile.EdgeOneProfile, error) {
	apiToken = strings.TrimSpace(apiToken)
	region = strings.TrimSpace(region)
	if interactive {
		if apiToken == "" {
			v, err := prompt.Password("EdgeOne API token（可选；留空则使用 edgeone login 登录态）", nil)
			if err != nil {
				return nil, err
			}
			apiToken = strings.TrimSpace(v)
		}
		if region == "" {
			v, err := prompt.Text("Tencent 区域（可选，如 ap-guangzhou；留空 = 默认）", "", nil)
			if err != nil {
				return nil, err
			}
			region = strings.TrimSpace(v)
		}
	}
	ep := &profile.EdgeOneProfile{Region: region}
	if apiToken != "" {
		ep.Credentials = &profile.EdgeOneCredentials{APIToken: apiToken}
	}
	return ep, nil
}

func defaultRegistryNamespace(registry, username string) string {
	registry = strings.TrimSpace(strings.TrimPrefix(registry, "https://"))
	registry = strings.TrimPrefix(registry, "http://")
	username = strings.TrimSpace(username)
	switch registry {
	case "ghcr.io", "docker.io", "index.docker.io":
		return username
	default:
		return ""
	}
}

func defaultKubeconfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "~/.kube/config"
	}
	return filepath.Join(home, ".kube", "config")
}

func expandPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func contextsWithCurrentFirst(contexts []string, current string) []string {
	if strings.TrimSpace(current) == "" {
		return contexts
	}
	out := make([]string, 0, len(contexts))
	for _, ctx := range contexts {
		if ctx == current {
			out = append(out, ctx)
			break
		}
	}
	for _, ctx := range contexts {
		if ctx != current {
			out = append(out, ctx)
		}
	}
	return out
}

type kubeconfigFile struct {
	CurrentContext string `yaml:"current-context"`
	Contexts       []struct {
		Name string `yaml:"name"`
	} `yaml:"contexts"`
}

func resolveKubeconfigContext(path, requested string) (string, error) {
	contexts, current, err := readKubeconfigContexts(path)
	if err != nil {
		return "", err
	}
	requested = strings.TrimSpace(requested)
	if requested != "" {
		for _, ctx := range contexts {
			if ctx == requested {
				return requested, nil
			}
		}
		return "", cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			fmt.Sprintf("kubeconfig %s 中不存在 context %q。", path, requested)).
			WithContext(map[string]any{"requested_context": requested, "contexts": contexts})
	}
	if current != "" {
		return current, nil
	}
	if len(contexts) == 1 {
		return contexts[0], nil
	}
	if len(contexts) == 0 {
		return "", cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			fmt.Sprintf("kubeconfig %s 没有 contexts。", path))
	}
	return "", cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
		fmt.Sprintf("kubeconfig %s 有多个 context，请传 --kubeconfig-context 明确选择。", path)).
		WithContext(map[string]any{"contexts": contexts})
}

func readKubeconfigContexts(path string) ([]string, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			fmt.Sprintf("读取 kubeconfig 失败：%s", err.Error())).
			WithContext(map[string]any{"path": path})
	}
	var cfg kubeconfigFile
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, "", cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			fmt.Sprintf("解析 kubeconfig 失败：%s", err.Error())).
			WithContext(map[string]any{"path": path})
	}
	seen := map[string]struct{}{}
	contexts := make([]string, 0, len(cfg.Contexts))
	for _, c := range cfg.Contexts {
		name := strings.TrimSpace(c.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		contexts = append(contexts, name)
	}
	current := strings.TrimSpace(cfg.CurrentContext)
	if current != "" {
		if _, ok := seen[current]; !ok {
			return nil, "", cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
				fmt.Sprintf("kubeconfig current-context %q 不在 contexts 列表中。", current)).
				WithContext(map[string]any{"path": path, "current_context": current, "contexts": contexts})
		}
	}
	return contexts, current, nil
}

func requireNonEmpty(v string) error {
	if strings.TrimSpace(v) == "" {
		return fmt.Errorf("不能为空")
	}
	return nil
}
