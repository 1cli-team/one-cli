package configurecmd

import (
	"fmt"
	"io"

	"strings"

	"github.com/spf13/cobra"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/preferences"
)

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
		fmt.Fprintf(w, i18n.T("configure.locale_success")+"\n", r.StoredLocale)
	}
	fmt.Fprintf(w, i18n.T("configure.locale_stored")+"\n", r.StoredLocale)
	fmt.Fprintf(w, i18n.T("configure.locale_resolved"), r.Resolved)
	if r.StoredLocale == preferences.LocaleAuto && r.Detected != "" {
		fmt.Fprint(w, i18n.T("configure.locale_from_env"))
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, i18n.T("configure.locale_path")+"\n", r.ConfigPath)
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
