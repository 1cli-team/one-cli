// Package configurecmd contributes the top-level `one configure`
// command via cliexts. configure is the only entry point for the
// profile lifecycle: add (upsert) / list / current / show / use /
// remove, plus a no-arg interactive wizard that selects a (domain,
// backend) pair and dispatches to the matching add command.
//
// Naming note: the *command* is `configure` to align with industry
// standard CLIs (aws / gcloud / azure). The *data object* is still a
// "profile" — that survives in the --profile flag, local workspace
// bindings, and the internal/profile Go package.
//
// Tree shape (verb-first, v0.7+):
//
//	configure
//	├── add [pair] [--profile <name>]  # bare → interactive wizard
//	│   ├── env/infisical [--profile <name>]    # backend-specific flags
//	│   ├── deploy/aliyun-oss [--profile <name>]
//	│   ├── deploy/tencent-cos [--profile <name>]
//	│   ├── deploy/aws-s3 [--profile <name>]
//	│   ├── deploy/minio  [--profile <name>]
//	│   ├── deploy/rustfs [--profile <name>]
//	│   ├── deploy/r2     [--profile <name>]
//	│   ├── deploy/kustomize [--profile <name>]
//	│   └── container/docker [--profile <name>]
//	├── list    [pair]                 # no pair → aggregate all sections
//	├── current [pair]                 # no pair → aggregate all sections
//	├── show    <pair> --profile <name>
//	├── use     <pair> --profile <name> [--workspace] [--project <name|path>]
//	└── remove  <pair> --profile <name>
//
// Storage is the two-file split: ~/.config/one/config.json (non-
// sensitive) + ~/.config/one/credentials.json (secrets), both 0600.
// The (domain, backend) section split is unchanged. Profile resolution
// chain is --profile flag → local workspace binding → section.default.
package configurecmd

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/cliexts"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/preferences"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/prompt"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func init() {
	cliexts.Register("configure", buildContributions)
}

// allPairs lists every supported (domain, backend) pair in canonical
// display order — same order as the fields on profile.Config so list
// / current aggregations and help text read consistently.
var allPairs = []struct {
	Domain  profile.Domain
	Backend string
}{
	{profile.DomainEnv, "infisical"},
	{profile.DomainDeploy, "aliyun-oss"},
	{profile.DomainDeploy, "tencent-cos"},
	{profile.DomainDeploy, "aws-s3"},
	{profile.DomainDeploy, "minio"},
	{profile.DomainDeploy, "rustfs"},
	{profile.DomainDeploy, "r2"},
	{profile.DomainDeploy, "kustomize"},
	{profile.DomainDeploy, "vercel"},
	{profile.DomainDeploy, "cloudflare"},
	{profile.DomainDeploy, "edgeone"},
	{profile.DomainContainer, "docker"},
	{profile.DomainContainer, "dockerhub"},
	{profile.DomainContainer, "ghcr"},
	{profile.DomainContainer, "acr"},
}

func buildContributions() []*cobra.Command {
	parent := &cobra.Command{
		Use: "configure",
		Long: `配置一组 endpoint profile（每个 (domain, backend) 一组），跨工作区共享。
每个 profile 描述一个 endpoint（一个 Infisical 实例 / 一个 k8s context /
一个 S3 endpoint / 一个 container registry），并自带凭据。

存储位置（AWS-CLI 风格双文件）：
  ~/.config/one/config.json       非敏感字段：endpoint / region / default 指针 / workspace 绑定 / credentialSource
  ~/.config/one/credentials.json  敏感字段：clientSecret / accessKeySecret / registry password
两个文件都是 mode 0600，仅本人可读；不入 git；不要复制到团队共享 dotfile 仓库。

无参快捷入口（推荐首次使用）：
  one configure                   交互式向导：选 (domain, backend) 再走对应 add 流程
  one configure add               同上（add 子命令的快捷入口）

子命令（verb-first 风格）：
  one configure add <pair> [--profile <name>]     新增 / 更新一个 profile
  one configure list [pair]                       列出 profile（无 pair 时聚合所有 section）
  one configure current [pair]                    打印 default profile（无 pair 时聚合所有 section）
  one configure show <pair> --profile <name>      打印 profile 全文（凭据默认掩码）
  one configure use <pair> --profile <name>       切换 default profile
  one configure use <pair> --profile <name> --workspace
                                                    绑定当前 workspace 的本机 profile（不写 manifest）
  one configure remove <pair> --profile <name>    删除一个 profile

支持的 (domain, backend) pair：
  env/infisical     Infisical 机器身份（siteUrl + clientId + clientSecret）
                    dotenv 不需要 profile（无凭据，工作区直接落本地 .env）
  deploy/aliyun-oss 阿里云 OSS（S3 协议；endpoint + region + AK/SK）
  deploy/tencent-cos 腾讯云 COS（S3 协议；endpoint + region + AK/SK）
  deploy/aws-s3     AWS S3（region + AK/SK；endpoint 留空走 SDK 默认）
  deploy/minio      MinIO 自部署对象存储（endpoint + AK/SK；默认 path-style）
  deploy/rustfs     RustFS 自部署对象存储（同 MinIO，默认 path-style）
  deploy/r2         Cloudflare R2（endpoint + AK/SK；region 通常为 auto）
  deploy/kustomize  Kubernetes 部署（kubeconfig + context）
  deploy/vercel     Vercel 部署（API token + 可选 team scope）
  deploy/cloudflare Cloudflare Workers 部署（API token + 可选 account scope）
  deploy/edgeone    Tencent EdgeOne Pages 部署（API token）
  container/docker  通用容器镜像仓库登录（自建 Harbor / Quay / 任何标准 docker registry）
  container/dockerhub Docker Hub（host 固定为 index.docker.io）
  container/ghcr    GitHub Container Registry（host 固定为 ghcr.io）
  container/acr     阿里云 Aliyun Container Registry（host 派生自 region）

profile 解析优先级（每次 one <domain> <verb>，每个 (domain, backend) 各走一遍）：
  1. --profile <name>
  2. ~/.config/one/config.json#workspaces[workspaceId].projects[projectName].profiles[domain/backend]
  3. ~/.config/one/config.json#workspaces[workspaceId].profiles[domain/backend]
  4. ~/.config/one/config.json#<domain>/<backend>.default`,
		RunE: runConfigureWizard,
	}
	parent.AddCommand(
		buildAddCmd(),
		buildListCmd(),
		buildCurrentCmd(),
		buildShowCmd(),
		buildUseCmd(),
		buildRemoveCmd(),
		buildLocaleCmd(),
	)
	i18n.MarkShort(parent, "configure.short")
	return []*cobra.Command{parent}
}

// ───────────────────── locale ─────────────────────
//
// `one configure locale` is the only user-global preference today
// (everything else under `configure` is per-(domain, backend)
// profile state). Lives here rather than as its own top-level
// command because that's what the project plan settled on
// ("用户可以通过 one configure 设置展示语言").

type localeResult struct {
	Schema       string `json:"schema"`
	StoredLocale string `json:"stored_locale"`
	Resolved     string `json:"resolved"`
	Detected     string `json:"detected,omitempty"`
	ConfigPath   string `json:"config_path"`
	Updated      bool   `json:"updated"`
}

func (r *localeResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	if r.Updated {
		fmt.Fprintf(w, "✓ 显示语言已设置为 %s\n", r.StoredLocale)
	}
	fmt.Fprintf(w, "stored:   %s\n", r.StoredLocale)
	fmt.Fprintf(w, "resolved: %s", r.Resolved)
	if r.StoredLocale == preferences.LocaleAuto && r.Detected != "" {
		fmt.Fprintf(w, "  (from $LANG / $LC_*)")
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "path:     %s\n", r.ConfigPath)
}

func buildLocaleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "locale [auto|zh-CN|en-US]",
		Long: `查看或设置 one CLI 的显示语言。

无参形式：打印当前生效的语言（preferences.json 中存储的值 + 实际解析结果）。
带参形式：把 preferences.json 中的 locale 字段写为指定值。

可选值：
  auto    跟随机器语言（解析 LC_ALL / LC_MESSAGES / LANG，识别 zh* → zh-CN，其它 → en-US）
  zh-CN   强制中文
  en-US   强制英文

dashboard（` + "`one serve`" + ` 起的本地 UI）共享这份 preferences.json，
所以在 dashboard 里切换语言也会写到这里；反之亦然。`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prefs, err := preferences.Load()
			if err != nil {
				return cliErrors.New(cliErrors.PROFILE_FILE_INVALID,
					"~/.config/one/preferences.json 读取失败："+err.Error())
			}
			path, _ := preferences.Path()

			if len(args) == 1 {
				newLocale := strings.TrimSpace(args[0])
				if !preferences.IsValidLocale(newLocale) {
					return cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
						fmt.Sprintf("未知 locale %q；可选 auto / zh-CN / en-US。", newLocale))
				}
				prefs.Locale = newLocale
				if err := preferences.Save(prefs); err != nil {
					return err
				}
				output.Emit(&localeResult{
					Schema:       "one-cli/configure-locale/v1",
					StoredLocale: newLocale,
					Resolved:     i18n.Resolve(newLocale),
					Detected:     i18n.DetectFromEnv(),
					ConfigPath:   path,
					Updated:      true,
				})
				return nil
			}

			output.Emit(&localeResult{
				Schema:       "one-cli/configure-locale/v1",
				StoredLocale: prefs.Locale,
				Resolved:     i18n.Resolve(prefs.Locale),
				Detected:     i18n.DetectFromEnv(),
				ConfigPath:   path,
				Updated:      false,
			})
			return nil
		},
	}
	i18n.MarkShort(cmd, "configure.locale.short")
	return cmd
}

// runConfigureWizard handles bare `one configure` and bare `one
// configure add`. In an interactive TTY it prompts the user for a
// (domain, backend) pair and then runs that pair's add flow. In a
// non-TTY shell (CI / -y mode) it errors with guidance to use the
// explicit path so scripts never accidentally hang on a prompt.
func runConfigureWizard(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return cliErrors.New(cliErrors.UNKNOWN_COMMAND,
			fmt.Sprintf("`one configure %s` 不是已知子命令；可用 (domain, backend) 见 `one configure --help`", strings.Join(args, " ")))
	}
	if !output.IsTTY() {
		return cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			"非交互式调用 `one configure [add]` 不支持；请显式指定 `one configure add <domain>/<backend> --profile <name>`，"+
				"如 `one configure add env/infisical --profile work --client-id ... --client-secret ...`。")
	}
	pair, err := pickPair()
	if err != nil {
		return err
	}
	domain, backend := splitPair(pair)
	return runSelectedAddBackend(cmd, domain, backend)
}

func runSelectedAddBackend(cmd *cobra.Command, domain profile.Domain, backend string) error {
	addCmd := newAddBackendCmd(domain, backend)
	addCmd.SetIn(cmd.InOrStdin())
	addCmd.SetOut(cmd.OutOrStdout())
	addCmd.SetErr(cmd.ErrOrStderr())
	// Cobra treats nil args as "read os.Args[1:]" on Execute. The
	// wizard is dispatching a fresh leaf command, so pass a real empty
	// slice to avoid replaying the original "configure add" args into
	// the selected backend.
	addCmd.SetArgs([]string{})
	return addCmd.Execute()
}

// pickPair prompts the user to choose one (domain, backend) pair from
// the five supported options. Returns the SectionKey ("env/infisical"
// etc.) so callers can split it back into the typed pieces.
func pickPair() (string, error) {
	type pairChoice struct {
		key   string
		label string
	}
	choices := []pairChoice{
		{"env/infisical", "env/infisical     Infisical 机器身份（机器级，跨工作区共享）"},
		{"deploy/aliyun-oss", "deploy/aliyun-oss 阿里云 OSS（S3 协议）"},
		{"deploy/tencent-cos", "deploy/tencent-cos 腾讯云 COS（S3 协议）"},
		{"deploy/aws-s3", "deploy/aws-s3     AWS S3"},
		{"deploy/minio", "deploy/minio      MinIO 自部署对象存储"},
		{"deploy/rustfs", "deploy/rustfs     RustFS 自部署对象存储"},
		{"deploy/r2", "deploy/r2         Cloudflare R2"},
		{"deploy/kustomize", "deploy/kustomize  Kubernetes 部署（kubeconfig + context）"},
		{"deploy/vercel", "deploy/vercel     Vercel 部署（API token + 可选 team scope）"},
		{"deploy/cloudflare", "deploy/cloudflare Cloudflare Workers 部署（API token + 可选 account scope）"},
		{"deploy/edgeone", "deploy/edgeone    Tencent EdgeOne Pages 部署（API token）"},
		{"container/docker", "container/docker  通用容器镜像仓库（自建 Harbor / Quay / 任何标准 docker registry）"},
		{"container/dockerhub", "container/dockerhub Docker Hub（host 固定 index.docker.io）"},
		{"container/ghcr", "container/ghcr    GitHub Container Registry（host 固定 ghcr.io）"},
		{"container/acr", "container/acr     阿里云 Aliyun Container Registry（host 派生自 region）"},
	}
	options := make([]prompt.Option[string], 0, len(choices))
	for _, c := range choices {
		options = append(options, prompt.Option[string]{Label: c.label, Value: c.key})
	}
	return prompt.Select("配置哪一类?", options)
}

// splitPair turns "env/infisical" back into (DomainEnv, "infisical").
// Caller has already validated the pair string came from pickPair so a
// missing slash is treated as a programmer bug, not user input.
func splitPair(pair string) (profile.Domain, string) {
	parts := strings.SplitN(pair, "/", 2)
	return profile.Domain(parts[0]), parts[1]
}

// parsePair turns a positional CLI arg ("env/infisical") into the
// typed pieces, or returns PROFILE_BACKEND_INVALID with the list of
// valid pairs when the input doesn't match.
func parsePair(arg string) (profile.Domain, string, error) {
	arg = strings.TrimSpace(arg)
	for _, p := range allPairs {
		if fmt.Sprintf("%s/%s", p.Domain, p.Backend) == arg {
			return p.Domain, p.Backend, nil
		}
	}
	valid := make([]string, 0, len(allPairs))
	for _, p := range allPairs {
		valid = append(valid, fmt.Sprintf("%s/%s", p.Domain, p.Backend))
	}
	return "", "", cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
		fmt.Sprintf("未知 (domain, backend) pair %q；可选：%s。", arg, strings.Join(valid, " / ")))
}

// pairCompletion gives shell completion the list of valid pairs as
// the first positional. After that the verb decides what comes next
// (a profile name or nothing); we don't try to autocomplete profile
// names because that would require loading config from disk.
func pairCompletion(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	out := make([]string, 0, len(allPairs))
	for _, p := range allPairs {
		out = append(out, fmt.Sprintf("%s/%s", p.Domain, p.Backend))
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// ───────────────────── add (upsert) ─────────────────────

func buildAddCmd() *cobra.Command {
	add := &cobra.Command{
		Use:   "add [pair] [--profile <name>]",
		Short: "新增 / 更新一个 profile",
		Long: `新增或更新一个 profile。每个 (domain, backend) 一棵 sub-subcommand,
分别拥有该 backend 的专属 flag(--site-url / --endpoint / --kubeconfig / ...)。

无参形式(TTY): 进入交互式向导,先选 (domain, backend) 再走对应流程。
非交互模式(CI / -y): 必须指定 pair 和 --profile,如:

  one configure add env/infisical --profile work --client-id $CID --client-secret $CSEC --use
  one configure add deploy/aliyun-oss --profile web-prod \
      --endpoint https://oss-<region>.aliyuncs.com \
      --region <region> --access-key-id $AK --access-key-secret $SK --use
  one configure add container/docker --profile prod \
      --registry <your-registry-host> --namespace <your-namespace> \
      --username $REGISTRY_USER --password $REGISTRY_TOKEN --use

行为:
  - 第一次配某个 name → status=completed,自动设为 default
  - 同名再跑一次     → status=updated(覆盖凭据)
  - 加 --use         → 显式切到这个 profile`,
		RunE: runConfigureWizard,
	}
	for _, p := range allPairs {
		add.AddCommand(newAddBackendCmd(p.Domain, p.Backend))
	}
	return add
}

type addResult struct {
	Schema          string `json:"schema"`
	Status          string `json:"status"`
	Domain          string `json:"domain"`
	Backend         string `json:"backend"`
	Name            string `json:"name"`
	Default         bool   `json:"default"`
	ConfigPath      string `json:"config_path"`
	CredentialsPath string `json:"credentials_path,omitempty"`
}

func (r *addResult) RenderTTY(w io.Writer) {
	if r == nil {
		return
	}
	verb := "configured"
	if r.Status == "updated" {
		verb = "updated"
	}
	suffix := ""
	if r.Default {
		suffix = "  [default]"
	}
	fmt.Fprintf(w, "✓ %s %s/%s profile %q%s\n",
		verb, r.Domain, r.Backend, r.Name, suffix)
	fmt.Fprintf(w, "  · config:      %s\n", r.ConfigPath)
	if r.CredentialsPath != "" {
		fmt.Fprintf(w, "  · credentials: %s\n", r.CredentialsPath)
	}
}

// newAddBackendCmd builds one of the five backend-specific
// sub-subcommands hung off `one configure add`. The Use field is
// the pair string (e.g. "env/infisical") so the user types
// `one configure add env/infisical [name]`.
func newAddBackendCmd(domain profile.Domain, backend string) *cobra.Command {
	var (
		setDefault  bool
		profileName string
		// infisical
		siteURL, clientID, clientSecret string
		// kustomize
		kubeconfigPath, kubeconfigContext string
		// s3
		s3Endpoint, s3Region             string
		s3AccessKeyID, s3AccessKeySecret string
		s3ForcePathStyle                 bool
		// container (docker / dockerhub / ghcr / acr)
		dockerRegistry, dockerRegion, dockerNamespace, dockerUser, dockerPwd string
		// vercel
		vercelToken, vercelTeam string
		// cloudflare
		cloudflareToken, cloudflareAccountID string
		// edgeone
		edgeoneToken, edgeoneRegion string
	)
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s/%s [--profile <name>]", domain, backend),
		Short: fmt.Sprintf("新增 / 更新一个 %s/%s profile", domain, backend),
		Long:  addLong(domain, backend),
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			interactive := output.IsTTY()
			name, err := resolveProfileName(profileName, interactive)
			if err != nil {
				return err
			}
			p := profile.Profile{Backend: backend}
			switch backend {
			case "infisical":
				infi, err := BuildInfisicalProfile(interactive,
					siteURL, clientID, clientSecret)
				if err != nil {
					return err
				}
				p.Infisical = infi
			case "kustomize":
				kp, err := BuildKustomizeProfile(interactive, kubeconfigPath, kubeconfigContext)
				if err != nil {
					return err
				}
				p.Kustomize = kp
			case "aliyun-oss", "tencent-cos", "aws-s3", "minio", "rustfs", "r2":
				s3p, err := BuildS3Profile(interactive, backend,
					s3Endpoint, s3Region, s3ForcePathStyle, s3AccessKeyID, s3AccessKeySecret)
				if err != nil {
					return err
				}
				p.S3 = s3p
			case "docker", "dockerhub", "ghcr", "acr":
				dp, err := BuildContainerProfile(interactive,
					backend, dockerRegistry, dockerRegion, dockerNamespace, dockerUser, dockerPwd)
				if err != nil {
					return err
				}
				p.Container = dp
			case "vercel":
				vp, err := BuildVercelProfile(interactive, vercelToken, vercelTeam)
				if err != nil {
					return err
				}
				p.Vercel = vp
			case "cloudflare":
				cp, err := BuildCloudflareProfile(interactive, cloudflareToken, cloudflareAccountID)
				if err != nil {
					return err
				}
				p.Cloudflare = cp
			case "edgeone":
				ep, err := BuildEdgeOneProfile(interactive, edgeoneToken, edgeoneRegion)
				if err != nil {
					return err
				}
				p.EdgeOne = ep
			default:
				return cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
					fmt.Sprintf("backend %q 暂不支持 profile 配置。", backend))
			}
			updated, err := profile.Upsert(domain, backend, name, p, setDefault)
			if err != nil {
				return err
			}
			output.Emit(buildAddResult(domain, backend, name, updated))
			return nil
		},
	}
	cmd.Flags().StringVar(&profileName, "profile", "", "profile 名（非交互模式必传；交互模式留空则 prompt）")
	cmd.Flags().BoolVar(&setDefault, "use", false, "把此 profile 设为 default")
	switch backend {
	case "infisical":
		cmd.Flags().StringVar(&siteURL, "site-url", "", "Infisical site URL（默认 https://app.infisical.com）")
		cmd.Flags().StringVar(&clientID, "client-id", "", "Universal Auth client ID")
		cmd.Flags().StringVar(&clientSecret, "client-secret", "", "Universal Auth client secret")
	case "kustomize":
		cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", "", "kubeconfig 文件路径（默认 ~/.kube/config）")
		cmd.Flags().StringVar(&kubeconfigContext, "kubeconfig-context", "", "kubeconfig context name")
	case "aliyun-oss", "tencent-cos", "aws-s3", "minio", "rustfs", "r2":
		cmd.Flags().StringVar(&s3Endpoint, "endpoint", "",
			s3EndpointFlagDesc(backend))
		cmd.Flags().StringVar(&s3Region, "region", "", s3RegionFlagDesc(backend))
		cmd.Flags().BoolVar(&s3ForcePathStyle, "force-path-style", s3ForcePathStyleDefault(backend),
			"用 path-style 寻址（MinIO / RustFS 默认开启；AWS / Aliyun / 腾讯 / R2 通常关闭）")
		cmd.Flags().StringVar(&s3AccessKeyID, "access-key-id", "", "AccessKey ID")
		cmd.Flags().StringVar(&s3AccessKeySecret, "access-key-secret", "", "AccessKey Secret")
	case "docker":
		cmd.Flags().StringVar(&dockerRegistry, "registry", "",
			"Registry host（如 your-harbor.example.com；docker kind 必填）")
		cmd.Flags().StringVar(&dockerNamespace, "namespace", "",
			"Registry namespace / owner（如组织名）")
		cmd.Flags().StringVar(&dockerUser, "username", "",
			"Registry 登录名（账号 / robot 账号 / PAT 持有者）")
		cmd.Flags().StringVar(&dockerPwd, "password", "",
			"Registry 登录凭据（PAT / token / 密码）")
	case "dockerhub":
		cmd.Flags().StringVar(&dockerNamespace, "namespace", "",
			"Docker Hub namespace（个人账号名 / 组织名；空则默认等于 username）")
		cmd.Flags().StringVar(&dockerUser, "username", "",
			"Docker Hub username")
		cmd.Flags().StringVar(&dockerPwd, "password", "",
			"Docker Hub access token (PAT)")
	case "ghcr":
		cmd.Flags().StringVar(&dockerNamespace, "namespace", "",
			"GHCR namespace（GitHub user / org；空则默认等于 username）")
		cmd.Flags().StringVar(&dockerUser, "username", "",
			"GitHub username")
		cmd.Flags().StringVar(&dockerPwd, "password", "",
			"GitHub Personal Access Token（需 write:packages 权限）")
	case "acr":
		cmd.Flags().StringVar(&dockerRegion, "region", "",
			"阿里云 ACR region（如 cn-hangzhou / cn-shanghai；host 会派生为 registry.<region>.aliyuncs.com）")
		cmd.Flags().StringVar(&dockerNamespace, "namespace", "",
			"Aliyun ACR namespace（必填，由你在阿里云控制台创建）")
		cmd.Flags().StringVar(&dockerUser, "username", "",
			"Aliyun ACR 登录账号（个人版用户名 / RAM AKID）")
		cmd.Flags().StringVar(&dockerPwd, "password", "",
			"Aliyun ACR 登录密码（仓库密码 / RAM AccessKey Secret）")
	case "vercel":
		cmd.Flags().StringVar(&vercelToken, "token", "",
			"Vercel API token（vercel.com → Account Settings → Tokens）")
		cmd.Flags().StringVar(&vercelTeam, "team", "",
			"Vercel team / org slug（可选，留空 = personal scope）")
	case "cloudflare":
		cmd.Flags().StringVar(&cloudflareToken, "token", "",
			"Cloudflare API token（dash.cloudflare.com → My Profile → API Tokens，需 Edit Workers 权限）")
		cmd.Flags().StringVar(&cloudflareAccountID, "account-id", "",
			"Cloudflare Account ID（可选，多账号 token 必填；dash 右侧栏 Account ID）")
	case "edgeone":
		cmd.Flags().StringVar(&edgeoneToken, "token", "",
			"EdgeOne API token（edgeone pages deploy --token 使用）")
		cmd.Flags().StringVar(&edgeoneRegion, "region", "",
			"Tencent 区域（可选，如 ap-guangzhou / ap-shanghai；留空默认）")
	}
	return cmd
}

func buildAddResult(domain profile.Domain, backend, name string, updated bool) *addResult {
	status := "completed"
	if updated {
		status = "updated"
	}
	isDefault := false
	if cfg, _, err := profile.Load(); err == nil {
		_, defaultName := listSection(cfg, domain, backend)
		isDefault = defaultName == name
	}
	cfgPath, _ := profile.ConfigPath()
	credPath, _ := profile.CredentialsPath()
	if !backendHasCredentials(backend) {
		credPath = ""
	}
	return &addResult{
		Schema:          "one-cli/configure-add/v1",
		Status:          status,
		Domain:          string(domain),
		Backend:         backend,
		Name:            name,
		Default:         isDefault,
		ConfigPath:      cfgPath,
		CredentialsPath: credPath,
	}
}

// backendHasCredentials reports whether the backend's profile schema
// includes a Credentials sub-struct (i.e. credentials.json sees
// entries for it). Dotenv / Kustomize have nothing in
// credentials.json, so we omit that path from the user-facing add
// result to avoid pointing them at an empty file.
func backendHasCredentials(backend string) bool {
	switch backend {
	case "infisical", "vercel", "cloudflare", "edgeone":
		return true
	}
	if profile.IsS3Compatible(backend) {
		return true
	}
	return profile.IsContainerKind(backend)
}

func resolveProfileName(flag string, interactive bool) (string, error) {
	name := strings.TrimSpace(flag)
	if name != "" {
		return name, nil
	}
	if !interactive {
		return "", cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
			"非交互模式必须通过 --profile 指定 profile 名。")
	}
	v, err := prompt.Text("profile 名（如 work / prod）", "", func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("不能为空")
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(v), nil
}

func addLong(domain profile.Domain, backend string) string {
	switch backend {
	case "infisical":
		return fmt.Sprintf(`新增或更新 %s/infisical profile。

凭据获取（Infisical 后台）：
  Organization → Access Control → Identities → 新建 Universal Auth
  identity，记下 client id 与 client secret。

行为：
  - 第一次配某个 name → status=completed，自动设为 default
  - 同名再跑一次     → status=updated（覆盖凭据）
  - 加 --use         → 显式切到这个 profile

示例：
  # 交互（推荐）
  one configure add %s/infisical

  # 非交互（CI / 脚本）
  one configure add %s/infisical --profile work \
      --site-url https://infisical.company.com \
      --client-id $CID --client-secret $CSEC \
      --use`, domain, domain, domain)
	case "aliyun-oss", "tencent-cos", "aws-s3", "minio", "rustfs", "r2":
		return s3CompatAddLong(domain, backend)
	case "kustomize":
		return fmt.Sprintf(`新增或更新 %s/kustomize profile。

profile 只决定要用 kubeconfig 里的哪个 context；k8s 认证还是走本地
~/.kube/config，或 --kubeconfig 指定的文件。kustomization overlay path 是 workspace 级
字段，写在 manifest.deploy；namespace 默认用 project.id，只有需要覆盖时才写 manifest.deploy.namespace。

示例：
  # 交互（推荐）
  one configure add %s/kustomize --profile prod-k8s --use

  # 非交互（CI / 脚本）
  one configure add %s/kustomize --profile prod-k8s \
      --kubeconfig ~/.kube/config \
      --kubeconfig-context prod \
      --use`, domain, domain, domain)
	case "vercel":
		return fmt.Sprintf(`新增或更新 %s/vercel profile。

凭据获取（Vercel 后台）：
  Account Settings → Tokens → Create Token
  可选 scope：personal 或某个 team。

team 字段对应 vercel CLI 的 --scope；留空 = personal account。

示例：
  # 交互（推荐）
  one configure add %s/vercel

  # 非交互（CI / 脚本）
  one configure add %s/vercel --profile work \
      --token $VERCEL_TOKEN \
      --team my-team \
      --use`, domain, domain, domain)
	case "cloudflare":
		return fmt.Sprintf(`新增或更新 %s/cloudflare profile。

凭据获取（Cloudflare 后台）：
  My Profile → API Tokens → Create Token
  最小权限：Account → Workers Scripts: Edit。覆盖 Pages 静态资源时同时
  勾选 Account → Cloudflare Pages: Edit。

account-id 字段对应 wrangler 的 CLOUDFLARE_ACCOUNT_ID；多账号 token 必填，
单账号 token 留空即可（wrangler 会从 token 推断）。

部署时凭据通过 CLOUDFLARE_API_TOKEN / CLOUDFLARE_ACCOUNT_ID 环境变量
传给 wrangler，**不会**出现在 argv 里。

示例：
  # 交互（推荐）
  one configure add %s/cloudflare

  # 非交互（CI / 脚本）
  one configure add %s/cloudflare --profile work \
      --token $CLOUDFLARE_API_TOKEN \
      --account-id $CLOUDFLARE_ACCOUNT_ID \
      --use`, domain, domain, domain)
	case "edgeone":
		return fmt.Sprintf(`新增或更新 %s/edgeone profile。

凭据使用 EdgeOne Pages API token，和上游命令：
  edgeone pages deploy <dir> --token <token>
保持一致。

部署时如果 profile 有 token，会以 --token 传给 edgeone CLI；One CLI 的 dry-run 输出会隐藏真实 token。
如果不配置 token，则使用本机 edgeone login 的登录态。

region 字段对应腾讯云区域 slug（ap-guangzhou / ap-shanghai 等），
留空让 CLI 默认。

示例：
  # 交互（本地可留空 token，改用 edgeone login）
  one configure add %s/edgeone

  # 非交互（CI / 脚本）
  one configure add %s/edgeone --profile work \
      --token $EDGEONE_API_TOKEN \
      --use`, domain, domain, domain)
	case "docker":
		return fmt.Sprintf(`新增或更新 %s/docker profile（通用 Docker registry 协议）。

适配任何走标准 docker registry 协议 + HTTP Basic auth 的镜像仓库：
自建 Harbor / Quay / 任意私有 registry / Cloudflare R2 / GitLab Container
Registry / 任何接受 "docker login --password-stdin" 的服务。

如果你用的是 Docker Hub / GHCR / 阿里云 ACR，请改用对应的 kind
(%s/dockerhub / %s/ghcr / %s/acr)，host 会自动派生，少配一个字段。

示例：
  # 交互（推荐）
  one configure add %s/docker

  # 非交互（CI / 脚本，地址按你的 registry 填）
  one configure add %s/docker --profile prod \
      --registry <your-registry-host> \
      --namespace <your-namespace> \
      --username $REGISTRY_USER --password $REGISTRY_TOKEN --use`, domain, domain, domain, domain, domain, domain)
	case "dockerhub":
		return fmt.Sprintf(`新增或更新 %s/dockerhub profile（Docker Hub）。

host 固定为 index.docker.io —— 不需要 --registry。namespace 留空时
默认等于 username（Docker Hub 的个人 / 组织目录通常等于账号名）。

凭据建议用 Docker Hub 的 Personal Access Token（Account Settings →
Security → Personal access tokens）而不是登录密码。

示例：
  # 交互（推荐）
  one configure add %s/dockerhub

  # 非交互（CI / 脚本）
  one configure add %s/dockerhub --profile prod \
      --username $DOCKER_HUB_USER \
      --password $DOCKER_HUB_PAT \
      --namespace my-org --use`, domain, domain, domain)
	case "ghcr":
		return fmt.Sprintf(`新增或更新 %s/ghcr profile（GitHub Container Registry）。

host 固定为 ghcr.io —— 不需要 --registry。namespace 留空时默认等于
username（GHCR 的 image path 通常是 ghcr.io/<user-or-org>/<image>）。

凭据必须用 GitHub Personal Access Token，且勾选 write:packages 权限
（PAT classic：Settings → Developer settings → Personal access tokens
→ Generate new token (classic) → 勾 write:packages）。

示例：
  # 交互（推荐）
  one configure add %s/ghcr

  # 非交互（CI / 脚本）
  one configure add %s/ghcr --profile prod \
      --username $GITHUB_USER \
      --password $GITHUB_PAT_WRITE_PACKAGES \
      --namespace my-org --use`, domain, domain, domain)
	case "acr":
		return fmt.Sprintf(`新增或更新 %s/acr profile（阿里云 Aliyun Container Registry）。

host 由 --region 派生为 registry.<region>.aliyuncs.com —— 不需要
--registry。namespace 必须用户自己在阿里云控制台创建后再填进来
（个人版的 namespace 是仓库前缀，不会自动取自用户名）。

凭据可以是：
  - 仓库登录账号 + 仓库登录密码（控制台「访问凭证」里设置）
  - RAM 子账号 AccessKey ID + AccessKey Secret

本次只支持「个人版」host 格式。企业版 instance 名（<instance>-registry.<region>.cr.aliyuncs.com）
的支持要等后续 PR。

示例：
  # 交互（推荐）
  one configure add %s/acr

  # 非交互（CI / 脚本）
  one configure add %s/acr --profile prod \
      --region cn-hangzhou \
      --namespace my-team \
      --username $ACR_USER --password $ACR_TOKEN --use`, domain, domain, domain)
	}
	return fmt.Sprintf("新增或更新 %s/%s profile。", domain, backend)
}

// s3CompatAddLong renders the `add` Long text for one of the six
// S3-compatible deploy backends. Each kind gets a matching pair of
// (endpoint placeholder, region default) hints so the user sees the
// shape of what to type without any concrete vendor address.
func s3CompatAddLong(domain profile.Domain, kind string) string {
	var vendorBlurb, endpointHint, regionHint, forcePathNote string
	switch kind {
	case "aliyun-oss":
		vendorBlurb = "阿里云 OSS（S3 协议）"
		endpointHint = "https://oss-<region>.aliyuncs.com"
		regionHint = "<region>（如 cn-hangzhou / cn-shanghai）"
		forcePathNote = "通常不需要 --force-path-style。"
	case "tencent-cos":
		vendorBlurb = "腾讯云 COS（S3 协议）"
		endpointHint = "https://cos.<region>.myqcloud.com"
		regionHint = "<region>（如 ap-guangzhou / ap-shanghai）"
		forcePathNote = "通常不需要 --force-path-style。"
	case "aws-s3":
		vendorBlurb = "AWS S3"
		endpointHint = "（留空 = AWS SDK 默认）"
		regionHint = "<region>（如 us-east-1 / ap-northeast-1）"
		forcePathNote = "通常不需要 --force-path-style。"
	case "minio":
		vendorBlurb = "MinIO 自部署对象存储"
		endpointHint = "http://<your-minio-host>:9000"
		regionHint = "<region>（任意值，常用 us-east-1 占位）"
		forcePathNote = "默认开启 --force-path-style。"
	case "rustfs":
		vendorBlurb = "RustFS 自部署对象存储"
		endpointHint = "http://<your-rustfs-host>:9000"
		regionHint = "<region>（任意值，常用 us-east-1 占位）"
		forcePathNote = "默认开启 --force-path-style。"
	case "r2":
		vendorBlurb = "Cloudflare R2"
		endpointHint = "https://<account-id>.r2.cloudflarestorage.com"
		regionHint = "auto"
		forcePathNote = "通常不需要 --force-path-style。"
	}
	return fmt.Sprintf(`新增或更新 %s/%s profile。

%s。所有 S3-protocol 兼容后端共享同一 profile 结构（endpoint + region +
AccessKey 对）；六个 backend id 只是给同一段实现起的不同名字，方便区分
你的 endpoint 来自哪家供应商。

bucket 是 subproject 级字段，由 `+"`one add`"+` 时的 prompt 写入
projects[i].deploy.bucket，不存在 profile 里。

示例：
  # 交互（推荐）
  one configure add %s/%s

  # 非交互（CI / 脚本）
  one configure add %s/%s --profile prod \
      --endpoint %s \
      --region %s \
      --access-key-id $AK --access-key-secret $SK --use

%s`, domain, kind, vendorBlurb, domain, kind, domain, kind, endpointHint, regionHint, forcePathNote)
}

// s3EndpointFlagDesc returns the --endpoint flag description specific
// to one S3-compatible backend kind. Placeholders only — no concrete
// vendor hosts.
func s3EndpointFlagDesc(kind string) string {
	switch kind {
	case "aliyun-oss":
		return "S3 endpoint URL（如 https://oss-<region>.aliyuncs.com）"
	case "tencent-cos":
		return "S3 endpoint URL（如 https://cos.<region>.myqcloud.com）"
	case "aws-s3":
		return "S3 endpoint URL（留空 = AWS SDK 默认）"
	case "minio":
		return "MinIO endpoint URL（如 http://<your-minio-host>:9000）"
	case "rustfs":
		return "RustFS endpoint URL（如 http://<your-rustfs-host>:9000）"
	case "r2":
		return "R2 endpoint URL（如 https://<account-id>.r2.cloudflarestorage.com）"
	}
	return "S3 endpoint URL"
}

// s3RegionFlagDesc returns the --region flag description specific to
// one S3-compatible backend kind.
func s3RegionFlagDesc(kind string) string {
	switch kind {
	case "aliyun-oss":
		return "Region（如 cn-hangzhou / cn-shanghai）"
	case "tencent-cos":
		return "Region（如 ap-guangzhou / ap-shanghai）"
	case "aws-s3":
		return "Region（如 us-east-1 / ap-northeast-1）"
	case "minio", "rustfs":
		return "Region（任意值，常用 us-east-1 占位）"
	case "r2":
		return "Region（通常 auto）"
	}
	return "Region"
}

// s3ForcePathStyleDefault returns the default --force-path-style flag
// value per kind. MinIO and RustFS need path-style addressing because
// they typically run without DNS for per-bucket subdomains.
func s3ForcePathStyleDefault(kind string) bool {
	switch kind {
	case "minio", "rustfs":
		return true
	}
	return false
}

// ───────────────────── list ─────────────────────

type listResult struct {
	Schema   string         `json:"schema"`
	Domain   string         `json:"domain"`
	Backend  string         `json:"backend"`
	Default  string         `json:"default,omitempty"`
	Profiles []profileEntry `json:"profiles"`
}

type profileEntry struct {
	Name             string `json:"name"`
	Default          bool   `json:"default"`
	CredentialSource string `json:"credentialSource,omitempty"`
}

func (r listResult) RenderTTY(w io.Writer) {
	if len(r.Profiles) == 0 {
		fmt.Fprintf(w, "（%s/%s 还没有 profile。运行 `one configure add %s/%s <name>` 创建。）\n",
			r.Domain, r.Backend, r.Domain, r.Backend)
		return
	}
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "DEFAULT\tNAME\tCRED SOURCE")
	for _, p := range r.Profiles {
		marker := " "
		if p.Default {
			marker = "*"
		}
		src := p.CredentialSource
		if src == "" {
			src = profile.SourceFile
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", marker, p.Name, src)
	}
	_ = tw.Flush()
}

// listAllResult is emitted by `one configure list` (no pair). It
// rolls up every (domain, backend) section into a single envelope so
// scripts can scan one's profile state without 5 separate calls.
type listAllResult struct {
	Schema   string                `json:"schema"`
	Sections []listAllSectionEntry `json:"sections"`
}

type listAllSectionEntry struct {
	Domain   string         `json:"domain"`
	Backend  string         `json:"backend"`
	Default  string         `json:"default,omitempty"`
	Profiles []profileEntry `json:"profiles"`
}

func (r listAllResult) RenderTTY(w io.Writer) {
	any := false
	for _, s := range r.Sections {
		if len(s.Profiles) == 0 {
			continue
		}
		any = true
		header := fmt.Sprintf("%s/%s", s.Domain, s.Backend)
		if s.Default != "" {
			header += fmt.Sprintf("  (default: %s)", s.Default)
		}
		fmt.Fprintln(w, header)
		tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
		for _, p := range s.Profiles {
			marker := " "
			if p.Default {
				marker = "*"
			}
			src := p.CredentialSource
			if src == "" {
				src = profile.SourceFile
			}
			fmt.Fprintf(tw, "  %s\t%s\t%s\n", marker, p.Name, src)
		}
		_ = tw.Flush()
		fmt.Fprintln(w)
	}
	if !any {
		fmt.Fprintln(w, "（还没有任何 profile。运行 `one configure add` 进入交互式向导，或 `one configure add <pair> <name>`。）")
	}
}

func buildListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [pair]",
		Short: "列出 profile（无 pair 时聚合所有 section）",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, _, err := profile.Load()
			if err != nil {
				return err
			}
			if len(args) == 0 {
				output.Emit(collectAllSections(cfg))
				return nil
			}
			domain, backend, err := parsePair(args[0])
			if err != nil {
				return err
			}
			output.Emit(collectSection(cfg, domain, backend))
			return nil
		},
		ValidArgsFunction: pairCompletion,
	}
	return cmd
}

func collectSection(cfg *profile.Config, domain profile.Domain, backend string) listResult {
	names, defaultName := listSection(cfg, domain, backend)
	sort.Strings(names)
	entries := make([]profileEntry, 0, len(names))
	for _, n := range names {
		entries = append(entries, profileEntry{
			Name:             n,
			Default:          n == defaultName,
			CredentialSource: sectionCredentialSource(cfg, domain, backend, n),
		})
	}
	return listResult{
		Schema:   "one-cli/configure-list/v1",
		Domain:   string(domain),
		Backend:  backend,
		Default:  defaultName,
		Profiles: entries,
	}
}

func collectAllSections(cfg *profile.Config) listAllResult {
	sections := make([]listAllSectionEntry, 0, len(allPairs))
	for _, p := range allPairs {
		section := collectSection(cfg, p.Domain, p.Backend)
		sections = append(sections, listAllSectionEntry{
			Domain:   section.Domain,
			Backend:  section.Backend,
			Default:  section.Default,
			Profiles: section.Profiles,
		})
	}
	return listAllResult{
		Schema:   "one-cli/configure-list-all/v1",
		Sections: sections,
	}
}

// sectionCredentialSource returns the credentialSource string of one
// profile inside (domain, backend), or "" when the section / profile
// is missing. Used by `list` to render a per-profile source column.
// Dotenv / kustomize profiles have no credentialSource discriminator
// and always return "".
func sectionCredentialSource(cfg *profile.Config, domain profile.Domain, backend, name string) string {
	switch {
	case domain == profile.DomainEnv && backend == "infisical":
		if p, ok := cfg.EnvInfisical.Profiles[name]; ok {
			return p.CredentialSource
		}
	case domain == profile.DomainDeploy && profile.IsS3Compatible(backend):
		sec := cfg.S3CompatSection(backend)
		if p, ok := sec.Profiles[name]; ok {
			return p.CredentialSource
		}
	case domain == profile.DomainDeploy && backend == "vercel":
		if p, ok := cfg.DeployVercel.Profiles[name]; ok {
			return p.CredentialSource
		}
	case domain == profile.DomainContainer && profile.IsContainerKind(backend):
		sec := cfg.ContainerKindSection(backend)
		if p, ok := sec.Profiles[name]; ok {
			return p.CredentialSource
		}
	}
	return ""
}

// ───────────────────── current ─────────────────────

type currentResult struct {
	Schema  string `json:"schema"`
	Domain  string `json:"domain"`
	Backend string `json:"backend"`
	Default string `json:"default,omitempty"`
}

func (r currentResult) RenderTTY(w io.Writer) {
	if r.Default == "" {
		fmt.Fprintf(w, "（%s/%s 当前没有 default profile。）\n", r.Domain, r.Backend)
		return
	}
	fmt.Fprintf(w, "%s\n", r.Default)
}

// currentAllResult is emitted by `one configure current` (no pair),
// listing the default profile of every section.
type currentAllResult struct {
	Schema   string                   `json:"schema"`
	Defaults []currentAllSectionEntry `json:"defaults"`
}

type currentAllSectionEntry struct {
	Domain  string `json:"domain"`
	Backend string `json:"backend"`
	Default string `json:"default,omitempty"`
}

func (r currentAllResult) RenderTTY(w io.Writer) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	for _, s := range r.Defaults {
		defaultName := s.Default
		if defaultName == "" {
			defaultName = "(none)"
		}
		fmt.Fprintf(tw, "%s/%s\t%s\n", s.Domain, s.Backend, defaultName)
	}
	_ = tw.Flush()
}

func buildCurrentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "current [pair]",
		Short: "打印 default profile（无 pair 时聚合所有 section）",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, _, err := profile.Load()
			if err != nil {
				return err
			}
			if len(args) == 0 {
				out := currentAllResult{Schema: "one-cli/configure-current-all/v1"}
				for _, p := range allPairs {
					_, defaultName := listSection(cfg, p.Domain, p.Backend)
					out.Defaults = append(out.Defaults, currentAllSectionEntry{
						Domain:  string(p.Domain),
						Backend: p.Backend,
						Default: defaultName,
					})
				}
				output.Emit(out)
				return nil
			}
			domain, backend, err := parsePair(args[0])
			if err != nil {
				return err
			}
			_, defaultName := listSection(cfg, domain, backend)
			output.Emit(currentResult{
				Schema:  "one-cli/configure-current/v1",
				Domain:  string(domain),
				Backend: backend,
				Default: defaultName,
			})
			return nil
		},
		ValidArgsFunction: pairCompletion,
	}
	return cmd
}

// ───────────────────── show ─────────────────────

type showResult struct {
	Schema           string          `json:"schema"`
	Domain           string          `json:"domain"`
	Backend          string          `json:"backend"`
	Name             string          `json:"name"`
	Profile          profile.Profile `json:"profile"`
	CredentialSource string          `json:"credentialSource"`
	Reveal           bool            `json:"reveal"`
}

func (r showResult) RenderTTY(w io.Writer) {
	fmt.Fprintf(w, "name:        %s\n", r.Name)
	fmt.Fprintf(w, "domain:      %s\n", r.Domain)
	fmt.Fprintf(w, "backend:     %s\n", r.Backend)
	src := r.CredentialSource
	if src == "" {
		src = profile.SourceFile
	}
	fmt.Fprintf(w, "cred source: %s\n", src)
	if r.Profile.Infisical != nil {
		i := r.Profile.Infisical
		fmt.Fprintln(w, "infisical:")
		fmt.Fprintf(w, "  siteUrl:     %s\n", i.SiteURL)
		if i.Credentials != nil {
			fmt.Fprintf(w, "  clientId:     %s\n", i.Credentials.ClientID)
			fmt.Fprintf(w, "  clientSecret: %s\n", i.Credentials.ClientSecret)
		}
	}
	if r.Profile.Kustomize != nil {
		k := r.Profile.Kustomize
		fmt.Fprintln(w, "kustomize:")
		if k.KubeconfigPath != "" {
			fmt.Fprintf(w, "  kubeconfig: %s\n", k.KubeconfigPath)
		}
		if k.KubeconfigContext != "" {
			fmt.Fprintf(w, "  context:    %s\n", k.KubeconfigContext)
		}
	}
	if r.Profile.S3 != nil {
		o := r.Profile.S3
		fmt.Fprintln(w, "s3:")
		if o.Endpoint != "" {
			fmt.Fprintf(w, "  endpoint:       %s\n", o.Endpoint)
		} else {
			fmt.Fprintf(w, "  endpoint:       (AWS S3 default)\n")
		}
		if o.Region != "" {
			fmt.Fprintf(w, "  region:         %s\n", o.Region)
		}
		if o.ForcePathStyle {
			fmt.Fprintln(w, "  forcePathStyle: true (MinIO / RustFS)")
		}
		if o.Credentials != nil {
			fmt.Fprintf(w, "  accessKeyId:     %s\n", o.Credentials.AccessKeyID)
			fmt.Fprintf(w, "  accessKeySecret: %s\n", o.Credentials.AccessKeySecret)
		}
	}
	if r.Profile.Vercel != nil {
		v := r.Profile.Vercel
		fmt.Fprintln(w, "vercel:")
		if v.Team != "" {
			fmt.Fprintf(w, "  team:     %s\n", v.Team)
		} else {
			fmt.Fprintf(w, "  team:     (personal scope)\n")
		}
		if v.Credentials != nil {
			fmt.Fprintf(w, "  apiToken: %s\n", v.Credentials.APIToken)
		}
	}
	if r.Profile.Container != nil {
		c := r.Profile.Container
		fmt.Fprintln(w, "container:")
		if c.Registry != "" {
			fmt.Fprintf(w, "  registry:  %s\n", c.Registry)
		}
		if c.Region != "" {
			fmt.Fprintf(w, "  region:    %s\n", c.Region)
		}
		if c.Namespace != "" {
			fmt.Fprintf(w, "  namespace: %s\n", c.Namespace)
		}
		if c.Credentials != nil {
			fmt.Fprintf(w, "  username:  %s\n", c.Credentials.Username)
			fmt.Fprintf(w, "  password:  %s\n", c.Credentials.Password)
		}
	}
	if !r.Reveal {
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "（凭据已掩码。--reveal 显示原文。）")
	}
}

func buildShowCmd() *cobra.Command {
	var (
		reveal      bool
		profileName string
	)
	cmd := &cobra.Command{
		Use:   "show <pair> --profile <name>",
		Short: "打印 profile 全文（凭据默认掩码）",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			domain, backend, err := parsePair(args[0])
			if err != nil {
				return err
			}
			name := strings.TrimSpace(profileName)
			resolved, err := profile.Resolve(profile.ResolveInput{
				Domain:       domain,
				Backend:      backend,
				FlagOverride: name,
			})
			if err != nil {
				return err
			}
			p := resolved.Profile
			if !reveal {
				p = maskCredentials(p)
			}
			output.Emit(showResult{
				Schema:           "one-cli/configure-show/v1",
				Domain:           string(domain),
				Backend:          backend,
				Name:             name,
				Profile:          p,
				CredentialSource: resolved.CredSource,
				Reveal:           reveal,
			})
			return nil
		},
		ValidArgsFunction: pairCompletion,
	}
	cmd.Flags().StringVar(&profileName, "profile", "", "profile 名（必填）")
	_ = cmd.MarkFlagRequired("profile")
	cmd.Flags().BoolVar(&reveal, "reveal", false, "显示凭据原文（默认掩码为 ********）")
	return cmd
}

func maskCredentials(p profile.Profile) profile.Profile {
	const masked = "********"
	if p.Infisical != nil && p.Infisical.Credentials != nil {
		c := *p.Infisical
		c.Credentials = &profile.InfisicalCredentials{ClientID: masked, ClientSecret: masked}
		p.Infisical = &c
	}
	if p.S3 != nil && p.S3.Credentials != nil {
		c := *p.S3
		c.Credentials = &profile.S3Credentials{AccessKeyID: masked, AccessKeySecret: masked}
		p.S3 = &c
	}
	if p.Vercel != nil && p.Vercel.Credentials != nil {
		c := *p.Vercel
		c.Credentials = &profile.VercelCredentials{APIToken: masked}
		p.Vercel = &c
	}
	if p.Cloudflare != nil && p.Cloudflare.Credentials != nil {
		c := *p.Cloudflare
		c.Credentials = &profile.CloudflareCredentials{APIToken: masked}
		p.Cloudflare = &c
	}
	if p.EdgeOne != nil && p.EdgeOne.Credentials != nil {
		c := *p.EdgeOne
		c.Credentials = &profile.EdgeOneCredentials{APIToken: masked}
		p.EdgeOne = &c
	}
	if p.Container != nil && p.Container.Credentials != nil {
		c := *p.Container
		c.Credentials = &profile.ContainerCredentials{Username: masked, Password: masked}
		p.Container = &c
	}
	return p
}

// ───────────────────── use ─────────────────────

type useResult struct {
	Schema      string `json:"schema"`
	Domain      string `json:"domain"`
	Backend     string `json:"backend"`
	Name        string `json:"name"`
	Scope       string `json:"scope"`
	WorkspaceID string `json:"workspaceId,omitempty"`
	Project     string `json:"project,omitempty"`
}

func (r useResult) RenderTTY(w io.Writer) {
	switch r.Scope {
	case "workspace-project":
		fmt.Fprintf(w, "✓ workspace %s project %s uses %s/%s profile: %s\n",
			r.WorkspaceID, r.Project, r.Domain, r.Backend, r.Name)
	case "workspace":
		fmt.Fprintf(w, "✓ workspace %s uses %s/%s profile: %s\n",
			r.WorkspaceID, r.Domain, r.Backend, r.Name)
	default:
		fmt.Fprintf(w, "✓ default %s/%s profile: %s\n", r.Domain, r.Backend, r.Name)
	}
}

func buildUseCmd() *cobra.Command {
	var profileName string
	var bindWorkspace bool
	var projectName string
	cmd := &cobra.Command{
		Use:   "use <pair> --profile <name> [--workspace] [--project <name|path>]",
		Short: "切换 default profile，或绑定当前 workspace 的 profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			domain, backend, err := parsePair(args[0])
			if err != nil {
				return err
			}
			name := strings.TrimSpace(profileName)
			result := useResult{
				Schema:  "one-cli/configure-use/v1",
				Domain:  string(domain),
				Backend: backend,
				Name:    name,
				Scope:   "default",
			}
			if bindWorkspace || strings.TrimSpace(projectName) != "" {
				root, err := workspace.ResolveProjectRoot("")
				if err != nil {
					return err
				}
				m, err := workspace.ReadManifest(root)
				if err != nil {
					return err
				}
				workspaceID := workspace.WorkspaceID(m)
				if strings.TrimSpace(workspaceID) == "" {
					return cliErrors.New(cliErrors.MANIFEST_INVALID,
						"当前 workspace 缺少 one.manifest.json#workspace.id，无法写入本机 workspace profile 绑定。")
				}
				project := strings.TrimSpace(projectName)
				if project != "" {
					if sub, err := workspace.ResolveProjectFromSelector(root, project); err == nil && sub != nil {
						project = sub.Name
					}
				}
				workspaceName := ""
				if m.Workspace != nil {
					workspaceName = m.Workspace.Name
				}
				if err := profile.BindWorkspaceProfile(workspaceID, workspaceName, root, project, domain, backend, name); err != nil {
					return err
				}
				result.Scope = "workspace"
				result.WorkspaceID = workspaceID
				if project != "" {
					result.Scope = "workspace-project"
					result.Project = project
				}
			} else {
				if err := profile.SetDefault(domain, backend, name); err != nil {
					return err
				}
			}
			output.Emit(result)
			return nil
		},
		ValidArgsFunction: pairCompletion,
	}
	cmd.Flags().StringVar(&profileName, "profile", "", "profile 名（必填）")
	cmd.Flags().BoolVar(&bindWorkspace, "workspace", false, "绑定到当前 workspace（写入本机 config.json，不修改 default）")
	cmd.Flags().StringVarP(&projectName, "project", "p", "", "绑定到当前 workspace 的某个 project（名称或相对路径）")
	_ = cmd.MarkFlagRequired("profile")
	return cmd
}

// ───────────────────── remove ─────────────────────

type removeResult struct {
	Schema  string `json:"schema"`
	Domain  string `json:"domain"`
	Backend string `json:"backend"`
	Name    string `json:"name"`
}

func (r removeResult) RenderTTY(w io.Writer) {
	fmt.Fprintf(w, "✓ removed %s/%s profile %q\n", r.Domain, r.Backend, r.Name)
}

func buildRemoveCmd() *cobra.Command {
	var profileName string
	cmd := &cobra.Command{
		Use:   "remove <pair> --profile <name>",
		Short: "删除一个 profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			domain, backend, err := parsePair(args[0])
			if err != nil {
				return err
			}
			name := strings.TrimSpace(profileName)
			if err := profile.Remove(domain, backend, name); err != nil {
				return err
			}
			output.Emit(removeResult{
				Schema:  "one-cli/configure-remove/v1",
				Domain:  string(domain),
				Backend: backend,
				Name:    name,
			})
			return nil
		},
		ValidArgsFunction: pairCompletion,
	}
	cmd.Flags().StringVar(&profileName, "profile", "", "profile 名（必填）")
	_ = cmd.MarkFlagRequired("profile")
	return cmd
}

// ───────────────────── shared helpers ─────────────────────

// listSection returns (names, default) for one (domain, backend)
// section. Each (domain, backend) maps to a discrete struct field on
// profile.Config; the profile package keeps the struct flat for v3
// schema readability so we mirror that here rather than going through
// a generic accessor.
func listSection(cfg *profile.Config, domain profile.Domain, backend string) ([]string, string) {
	switch {
	case domain == profile.DomainEnv && backend == "infisical":
		return mapKeys(cfg.EnvInfisical.Profiles), cfg.EnvInfisical.Default
	case domain == profile.DomainEnv && backend == "dotenv":
		return mapKeys(cfg.EnvDotenv.Profiles), cfg.EnvDotenv.Default
	case domain == profile.DomainDeploy && profile.IsS3Compatible(backend):
		sec := cfg.S3CompatSection(backend)
		return mapKeys(sec.Profiles), sec.Default
	case domain == profile.DomainDeploy && backend == "kustomize":
		return mapKeys(cfg.DeployKustomize.Profiles), cfg.DeployKustomize.Default
	case domain == profile.DomainDeploy && backend == "vercel":
		return mapKeys(cfg.DeployVercel.Profiles), cfg.DeployVercel.Default
	case domain == profile.DomainDeploy && backend == "cloudflare":
		return mapKeys(cfg.DeployCloudflare.Profiles), cfg.DeployCloudflare.Default
	case domain == profile.DomainDeploy && backend == "edgeone":
		return mapKeys(cfg.DeployEdgeOne.Profiles), cfg.DeployEdgeOne.Default
	case domain == profile.DomainContainer && profile.IsContainerKind(backend):
		sec := cfg.ContainerKindSection(backend)
		return mapKeys(sec.Profiles), sec.Default
	}
	return nil, ""
}

func mapKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
