// Package servecmd contributes `one serve` — a local Dashboard for observed
// Workspaces, their Projects, and machine-level Profiles. AI agents read
// credential files at their peril, so sensitive Profile editing remains a
// human-only browser path.
package servecmd

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	configureapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/configure"
	manifestapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/manifest"
	workspaceapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/workspace"
	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/transport/http"
)

type Dependencies struct {
	Catalog      *catalog.Catalog
	Profiles     *configureapp.ProfileService
	Manifest     *manifestapp.Service
	Environments *environmentmodule.Service
	Workspaces   *workspaceapp.Service
	Registry     *workspaceapp.RegistryService
}

func Commands(deps Dependencies) []*cobra.Command {
	return []*cobra.Command{newServeCmd(deps)}
}

func newServeCmd(deps Dependencies) *cobra.Command {
	var (
		host string
		port int
		open bool
	)
	cmd := &cobra.Command{
		Use: "serve",
		Long: `启动一个本地 HTTP 服务，在浏览器里查看本机 Workspace、配置其中的
Project、审阅后保存 Manifest 配置、管理 Infisical 密钥，并管理 profile（env / deploy / container 各 backend）。Profile
含 API key、kubeconfig path、registry token 等敏感字段，AI 不应读写；
本命令是给你（人类）的入口。

默认行为：绑定 127.0.0.1 + 内核分配空闲端口 + 自动用系统默认浏览器
打开 URL。打印 URL 后阻塞，按 Ctrl-C 退出。

安全模型：
  - 仅绑定 127.0.0.1（loopback）
  - Host header 校验，挡 DNS rebinding
  - 全部 mutating 请求做 Origin 校验`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			parent := cmd.Context()
			if parent == nil {
				parent = context.Background()
			}
			ctx, cancel := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
			defer cancel()

			// WalkUpToManifest fails with NOT_ONE_PROJECT when there's no
			// one.manifest.json anywhere up the tree. That's fine here —
			// `one serve` is happy to run outside a workspace: it still
			// loads the persisted registry and machine-level Profiles.
			target, registryWarn := discoverServeWorkspaceTarget(ctx, "", deps.Registry)
			if registryWarn != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), i18n.Tf("serve.registry_warning", registryWarn))
			}

			return serve.Run(ctx, serve.Opts{
				Host:               host,
				Port:               port,
				WorkspaceRoot:      target.Root,
				Catalog:            deps.Catalog,
				ProfileService:     deps.Profiles,
				ManifestService:    deps.Manifest,
				EnvironmentService: deps.Environments,
				WorkspaceService:   deps.Workspaces,
				RegistryService:    deps.Registry,
			}, func(res serve.Result) {
				output.Emit(res)
				if cmd.Name() == "serve" {
					res.URL = workspaceDashboardURL(res.URL, target.EntryID)
				}
				maybeOpenBrowser(cmd.ErrOrStderr(), res, open)
			})
		},
	}
	cmd.Flags().StringVar(&host, "host", "127.0.0.1", i18n.T("serve.flag.host"))
	cmd.Flags().IntVar(&port, "port", 0, i18n.T("serve.flag.port"))
	cmd.Flags().BoolVar(&open, "open", true, i18n.T("serve.flag.open"))
	i18n.MarkFlagUsage(cmd, "host", "serve.flag.host")
	i18n.MarkFlagUsage(cmd, "port", "serve.flag.port")
	i18n.MarkFlagUsage(cmd, "open", "serve.flag.open")
	i18n.MarkShort(cmd, "serve.short")
	return cmd
}

type serveWorkspaceTarget struct {
	Root    string
	EntryID string
}

func discoverServeWorkspaceTarget(
	ctx context.Context,
	start string,
	registry *workspaceapp.RegistryService,
) (serveWorkspaceTarget, error) {
	root, err := workspace.WalkUpToManifest(start)
	if err != nil {
		return serveWorkspaceTarget{}, nil
	}
	target := serveWorkspaceTarget{Root: root}
	if registry == nil {
		return target, nil
	}
	registered, err := registry.Observe(ctx, root, "serve")
	if err != nil {
		return target, err
	}
	target.EntryID = registered.EntryID
	return target, nil
}

// discoverServeWorkspace keeps optional Workspace discovery and registration
// in one testable boundary. Running outside a Workspace is valid: the server
// still exposes machine-level profiles and the previously observed registry.
func discoverServeWorkspace(
	ctx context.Context,
	start string,
	registry *workspaceapp.RegistryService,
) (string, error) {
	target, err := discoverServeWorkspaceTarget(ctx, start, registry)
	return target.Root, err
}

func workspaceDashboardURL(baseURL, entryID string) string {
	if strings.TrimSpace(baseURL) == "" || strings.TrimSpace(entryID) == "" {
		return baseURL
	}
	return strings.TrimRight(baseURL, "/") + "/workspace/" + url.PathEscape(entryID)
}

// NewOpenCmd exposes the same local settings server under the user-facing
// `one configure open` path while keeping `one serve` compatible.
func NewOpenCmd(deps Dependencies) *cobra.Command {
	cmd := newServeCmd(deps)
	cmd.Use = "open"
	cmd.Example = "  one configure open"
	i18n.MarkShort(cmd, "configure.open.short")
	i18n.MarkLong(cmd, "configure.open.tip")
	return cmd
}

// maybeOpenBrowser fires `pkg/browser`'s OpenURL when it makes sense.
//
// We skip opening when:
//   - --open=false is explicitly passed,
//   - stdout is being structured (piped to JSON consumer / redirected — the
//     caller is automation; popping a window would be surprising),
//   - the URL is non-trivially missing (defensive).
//
// Open failures (no DISPLAY, WSL without wslview, headless Linux without
// xdg-open) are non-fatal — we already printed the URL via output.Emit and
// the user can copy it manually.
func maybeOpenBrowser(stderr io.Writer, res serve.Result, open bool) {
	if !open || res.URL == "" {
		return
	}
	if !output.IsTTY() {
		return
	}
	// pkg/browser logs to its own io.Writer if we don't redirect; route to
	// /dev/null so an attempted-and-failed open doesn't spam the user (we
	// keep our own friendly stderr line below).
	browser.Stderr = io.Discard
	browser.Stdout = io.Discard
	if err := browser.OpenURL(res.URL); err != nil {
		fmt.Fprintf(stderr, i18n.T("serve.browser_failed")+"\n", err)
	}
}
