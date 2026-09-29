package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Removed automation flags must fail before creating even an empty workspace.
func TestE2E_CreateRejectsRemovedFlagsBeforeWriting(t *testing.T) {
	for _, locale := range []string{"zh_CN.UTF-8", "en_US.UTF-8"} {
		for _, flag := range []string{"--preset", "--project-names"} {
			t.Run(locale+"/"+flag, func(t *testing.T) {
				temp := t.TempDir()
				isolateHome(t, filepath.Join(temp, "home"))
				t.Setenv("LC_ALL", locale)
				target := filepath.Join(temp, "demo")
				stdout, stderr, code := runBinary(t, "create", target, flag, "retired", "--yes", "-o", "json")
				if code == 0 || !strings.Contains(stderr+stdout, strings.TrimPrefix(flag, "--")) {
					t.Fatalf("expected an error naming %s: %d %s %s", flag, code, stdout, stderr)
				}
				if _, err := os.Stat(target); !os.IsNotExist(err) {
					t.Fatalf("removed flag changed destination: %v", err)
				}
			})
		}
	}
}
