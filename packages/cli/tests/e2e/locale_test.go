package cli_test

import (
	"strings"
	"testing"
	"unicode"

	"github.com/spf13/pflag"
	cli "github.com/torchstellar-team/one-cli/packages/cli/internal/bootstrap/cli"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

func TestAllHelpRefreshesBetweenLocales(t *testing.T) {
	root := cli.RootCmd()
	t.Cleanup(func() { _ = i18n.Init(i18n.DefaultLocale); i18n.RefreshTree(root) })
	for _, locale := range []string{"en-US", "zh-CN", "en-US"} {
		_ = i18n.Init(locale)
		i18n.RefreshTree(root)
		for _, walk := range walkTree(root) {
			cmd := walk.cmd
			for annotation, value := range map[string]string{i18n.AnnotationShort: cmd.Short, i18n.AnnotationLong: cmd.Long} {
				if value == "" {
					continue
				}
				key := cmd.Annotations[annotation]
				if key == "" || i18n.T(key) != value {
					t.Errorf("%s %s not registered/refreshed: %q", cmd.CommandPath(), annotation, value)
				}
			}
			checkFlag := func(flag *pflag.Flag) {
				if flag.Hidden || flag.Name == "help" {
					return
				}
				keys := flag.Annotations[i18n.AnnotationFlag]
				if len(keys) == 0 || flag.Usage != i18n.T(keys[0]) {
					t.Errorf("%s --%s not registered/refreshed", cmd.CommandPath(), flag.Name)
				}
			}
			cmd.LocalNonPersistentFlags().VisitAll(checkFlag)
			cmd.PersistentFlags().VisitAll(checkFlag)
			help := renderHelp(t, cmd)
			if locale == "en-US" && strings.ContainsFunc(help, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
				t.Errorf("English help contains Chinese: %s\n%s", cmd.CommandPath(), help)
			}
		}
	}
}

func TestE2E_LocalePreferenceAndImmediateConfirmation(t *testing.T) {
	root := t.TempDir()
	isolateHome(t, root)
	t.Setenv("LC_ALL", "en_US.UTF-8")
	zh, stderr, code := runBinaryIn(t, root, "locale", "zh-CN", "-o", "text")
	if code != 0 || !strings.Contains(zh, "显示语言已设置为 zh-CN") {
		t.Fatalf("switch to Chinese: %s %s", zh, stderr)
	}
	for _, args := range [][]string{{"run", "--help"}, {"login", "--help"}, {"templates", "-o", "text"}} {
		out, errText, code := runBinaryIn(t, root, args...)
		if code != 0 || !strings.ContainsFunc(out, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
			t.Fatalf("Chinese %v: %s %s", args, out, errText)
		}
	}
	_, stderr, code = runBinaryIn(t, root, "env", "get", "-o", "text")
	if code == 0 || !strings.Contains(stderr, "参数") || !strings.Contains(stderr, "one env get") {
		t.Fatalf("Chinese argument error: %s", stderr)
	}
	_, stderr, code = runBinaryIn(t, root, "serve", "--port=invalid", "-o", "text")
	if code == 0 || !strings.Contains(stderr, "无效") || !strings.Contains(stderr, "invalid") {
		t.Fatalf("Chinese flag error: %s", stderr)
	}
	out, stderr, code := runBinaryIn(t, root, "locale", "en-US", "-o", "text")
	if code != 0 || !strings.Contains(out, "en-US") || strings.ContainsFunc(out, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
		t.Fatalf("switch to English: %s %s", out, stderr)
	}
	for _, args := range [][]string{{"run", "--help"}, {"templates", "-o", "text"}} {
		out, errText, code := runBinaryIn(t, root, args...)
		if code != 0 || strings.ContainsFunc(out, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
			t.Fatalf("English %v: %s %s", args, out, errText)
		}
	}
	t.Setenv("LC_ALL", "zh_CN.UTF-8")
	out, stderr, code = runBinaryIn(t, root, "locale", "auto", "-o", "json")
	if code != 0 {
		t.Fatalf("auto: %s %s", out, stderr)
	}
	result := mustParseJSON(t, out)
	if result["schema"] != "one-cli/locale/v1" || result["stored_locale"] != "auto" || result["resolved"] != "zh-CN" {
		t.Fatalf("locale protocol changed: %v", result)
	}
}
