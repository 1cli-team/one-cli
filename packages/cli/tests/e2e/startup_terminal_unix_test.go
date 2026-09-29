//go:build unix

package cli_test

import (
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

func TestE2E_NoninteractiveStartupDoesNotQueryTerminal(t *testing.T) {
	root := t.TempDir()
	isolateHome(t, root)
	buildWrite(t, root, "one.manifest.toml", "version = 2\n[workspace]\nid = 'startup'\nname = 'startup'\n")
	buildWrite(t, root, "mise.toml", "[tasks.dev]\nrun = 'echo SHOULD_NOT_RUN'\n")
	for _, locale := range []string{"en_US.UTF-8", "zh_CN.UTF-8"} {
		t.Run(locale, func(t *testing.T) {
			t.Setenv("LC_ALL", locale)
			for _, tc := range []struct {
				name string
				args []string
				want string
			}{
				{"version", []string{"--version"}, expectedBuildVersion(t)},
				{"help", []string{"dev", "--help"}, "--project"},
				{"preview", []string{"dev", "--dry-run"}, "//:dev"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					terminal := startTaskTerminal(t, root, tc.args...)
					terminal.wait(t, 0)
					got := terminal.out.String()
					if !strings.Contains(got, tc.want) || strings.Contains(got, "SHOULD_NOT_RUN") || strings.Contains(got, "[one]") {
						t.Fatalf("unexpected output: %q", got)
					}
					// Even a helper's package initialization can query the terminal
					// before main. These commands must not depend on a reply; an
					// unresponsive terminal would otherwise stall every invocation.
					for _, query := range []string{"\x1b]11;?", "\x1b[c", "\x1b[6n"} {
						if strings.Contains(got, query) {
							t.Fatalf("startup queried the terminal with %q: %q", query, got)
						}
					}
				})
			}
		})
	}
}

func TestE2E_TaskStartupProgressIsLocalized(t *testing.T) {
	for _, locale := range []struct{ id, env string }{{"en-US", "en_US.UTF-8"}, {"zh-CN", "zh_CN.UTF-8"}} {
		t.Run(locale.id, func(t *testing.T) {
			root := t.TempDir()
			isolateHome(t, root)
			t.Setenv("LC_ALL", locale.env)
			if err := i18n.Init(locale.id); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = i18n.Init("en-US") })
			buildWrite(t, root, "one.manifest.toml", "version = 2\n[workspace]\nid = 'startup'\nname = 'startup'\n")
			buildWrite(t, root, "mise.toml", "[tasks.dev]\nrun = 'echo CHILD_STARTED'\n")
			terminal := startTaskTerminal(t, root, "dev", "--ui", "stream")
			terminal.wait(t, 0)
			got := terminal.out.String()
			last := -1
			for _, want := range []string{i18n.T("tasks.preparing"), i18n.T("tasks.preparing_dependencies"), i18n.Tf("tasks.preparing_environment", "dev"), "CHILD_STARTED"} {
				at := strings.Index(got, want)
				if at <= last {
					t.Fatalf("missing/out of order %q: %q", want, got)
				}
				last = at
			}
		})
	}
}
