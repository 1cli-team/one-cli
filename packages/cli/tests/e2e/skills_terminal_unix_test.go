//go:build unix

package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

func TestE2E_AutomaticSkillsKeepLocalizedSpinnerClean(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Init(i18n.DefaultLocale) })
	for _, locale := range []struct{ id, env string }{{"en-US", "en_US.UTF-8"}, {"zh-CN", "zh_CN.UTF-8"}} {
		t.Run(locale.id, func(t *testing.T) {
			parent := t.TempDir()
			isolateHome(t, t.TempDir())
			t.Setenv("LC_ALL", locale.env)
			if err := i18n.Init(locale.id); err != nil {
				t.Fatal(err)
			}
			tools := t.TempDir()
			script := `#!/bin/sh
set -eu
printf 'SKILLS_UPSTREAM_BANNER\nTip: use --global\n'
printf 'UPSTREAM_STDERR_NOTICE\n' >&2
selecting=no
for arg in "$@"; do
  case "$arg" in
    --skill) selecting=yes ;;
    --agent) selecting=no ;;
    *) if [ "$selecting" = yes ]; then
         mkdir -p ".agents/skills/$arg"
         printf 'fixture skill\n' > ".agents/skills/$arg/SKILL.md"
       fi ;;
  esac
done
sleep 0.3
`
			if err := os.WriteFile(filepath.Join(tools, "npx"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
			created := startTaskTerminal(t, parent, "create", "demo", "--yes")
			created.wait(t, 0)
			assertSkillsSpinner(t, created.out.String(), "one-cli")
			assertSkillsSpinner(t, created.out.String(), "find-skills")
			root := filepath.Join(parent, "demo")
			added := startTaskTerminal(t, root, "add", "go-lib", "--name", "shared", "--yes")
			added.wait(t, 0)
			assertSkillsSpinner(t, added.out.String(), "golang-patterns, golang-testing")
			mobile := startTaskTerminal(t, root, "add", "expo-mobile", "--name", "mobile", "--yes")
			mobile.wait(t, 0)
			assertSkillsSpinner(t, mobile.out.String(), "expo-overview")
			truncated := false
			for _, fragment := range strings.Split(mobile.out.String(), "\r") {
				line := ansi.Strip(fragment)
				characters := []rune(line)
				if len(characters) == 0 || characters[0] < 0x2800 || characters[0] > 0x28ff {
					continue
				}
				if ansi.StringWidth(line) >= 110 {
					t.Fatalf("spinner would wrap in the 110-column terminal: %q", line)
				}
				truncated = truncated || strings.HasSuffix(line, "…")
			}
			if !truncated {
				t.Fatalf("long skill names were not truncated: %q", mobile.out.String())
			}
		})
	}
}

func assertSkillsSpinner(t *testing.T, got, names string) {
	t.Helper()
	for _, hidden := range []string{"SKILLS_UPSTREAM_BANNER", "Tip: use --global", "UPSTREAM_STDERR_NOTICE"} {
		if strings.Contains(got, hidden) {
			t.Fatalf("automatic install leaked upstream UI: %q", got)
		}
	}
	message := i18n.Tf("skills.installing", names)
	at := strings.Index(got, message)
	if at < 0 {
		t.Fatalf("missing localized skills progress %q: %q", message, got)
	}
	start := strings.LastIndex(got[:at], "\r")
	if start < 0 || !strings.HasPrefix(got[start:at], "\r\x1b[2K") || strings.Contains(got[start:at], "\n") {
		t.Fatalf("skills status did not replace the spinner on a cleared line: %q", got)
	}
}
