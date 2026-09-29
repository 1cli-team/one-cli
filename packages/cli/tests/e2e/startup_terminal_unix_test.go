//go:build unix

package cli_test

import (
	"strings"
	"testing"
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
					if !strings.Contains(got, tc.want) || strings.Contains(got, "SHOULD_NOT_RUN") {
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
