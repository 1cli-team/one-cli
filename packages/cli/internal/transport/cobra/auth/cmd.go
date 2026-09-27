package authcmd

import (
	"fmt"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
)

func Commands() []*cobra.Command {
	var site string
	login := &cobra.Command{Use: "login", Short: "在浏览器中登录 Infisical", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		a, e := session.Start(c.Context(), site)
		if e != nil {
			return e
		}
		defer a.Cancel()
		fmt.Fprintf(c.ErrOrStderr(), "请在浏览器中完成登录：\n%s\n", a.URL)
		_ = browser.OpenURL(a.URL)
		info, e := a.Wait()
		if e != nil {
			return e
		}
		output.Emit(info)
		return nil
	}}
	login.Flags().StringVar(&site, "site-url", session.DefaultSiteURL, "Infisical 实例根地址")
	who := &cobra.Command{Use: "whoami", Short: "查看当前 Infisical 登录状态（不显示令牌）", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		info, e := session.Status()
		if e != nil {
			return e
		}
		output.Emit(info)
		return nil
	}}
	logout := &cobra.Command{Use: "logout", Short: "退出本机 Infisical 会话，保留变量位置", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		if e := session.Logout(); e != nil {
			return e
		}
		output.Emit(map[string]any{"loggedIn": false})
		return nil
	}}
	return []*cobra.Command{login, who, logout}
}
