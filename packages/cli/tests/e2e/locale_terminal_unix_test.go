//go:build unix

package cli_test

import (
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

func TestE2E_AddPromptAndBuiltInTemplatesFollowLocale(t *testing.T) {
	for _, tc := range []struct{ locale, title, template string }{
		{"en_US.UTF-8", "What would you like to add?", "React single-page application"},
		{"zh_CN.UTF-8", "想添加什么？", "React 单页应用"},
	} {
		t.Run(tc.locale, func(t *testing.T) {
			tmp := t.TempDir()
			isolateHome(t, tmp)
			root := bootstrapWorkspace(t, tmp, "demo")
			t.Setenv("LC_ALL", tc.locale)
			terminal := startTaskTerminal(t, root, "add")
			waitForTTYOutput(t, terminal.out, tc.title, 8*time.Second)
			_, _ = terminal.pty.Write([]byte("\r"))
			waitForTTYOutput(t, terminal.out, tc.template, 8*time.Second)
			_, _ = terminal.pty.Write([]byte{3})
			terminal.wait(t, 0)
			if tc.locale == "en_US.UTF-8" && strings.ContainsFunc(ansi.Strip(terminal.out.String()), func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
				t.Fatalf("English prompt contains Chinese: %s", terminal.out.String())
			}
		})
	}
}
