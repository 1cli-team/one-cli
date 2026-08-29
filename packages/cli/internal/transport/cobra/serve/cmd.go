// Package servecmd contributes `one serve` — a local HTTP server with a web
// UI for editing ~/.config/one/config.json and credentials.json. AI agents
// read these files at their peril (they hold Infisical secrets, kubeconfig
// paths, container registry tokens), so the UI is the human-only path.
package servecmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	configureapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/configure"
	workspaceapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/workspace"
	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/transport/http"
)

type Dependencies struct {
	Catalog    *catalog.Catalog
	Profiles   *configureapp.ProfileService
	Workspaces *workspaceapp.Service
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
		Long: `启动一个本地 HTTP 服务，在浏览器里手工编辑 profile（env / deploy /
container 各 backend）。Profile 含 API key、kubeconfig path、registry
token 等敏感字段，AI 不应读写；本命令是给你（人类）的入口。

默认行为：绑定 127.0.0.1 + 内核分配空闲端口 + 自动用系统默认浏览器
打开 URL（带一次性 session token）。打印 URL 后阻塞，按 Ctrl-C 退出。

安全模型：
  - 仅绑定 127.0.0.1（loopback）
  - Host header 校验，挡 DNS rebinding
  - 全部 mutating 请求做 Origin 校验
  - 每次启动生成一次性 session token；URL 自带 ?token=...，
    首次访问后写入 cookie`,
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
			// `one serve` is happy to run outside a workspace (the
			// profile-editor surface works either way); only the new
			// Overview endpoint cares, and it returns {present: false}
			// when WorkspaceRoot == "".
			workspaceRoot, _ := workspace.WalkUpToManifest("")

			return serve.Run(ctx, serve.Opts{
				Host:             host,
				Port:             port,
				WorkspaceRoot:    workspaceRoot,
				Catalog:          deps.Catalog,
				ProfileService:   deps.Profiles,
				WorkspaceService: deps.Workspaces,
			}, func(res serve.Result) {
				output.Emit(res)
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
