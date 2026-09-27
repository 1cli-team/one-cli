//go:build unix

package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2E_DevSynchronizesPNPMLockfileAndReusesInstall(t *testing.T) {
	root := devTerminalFixture(t, false)
	buildWrite(t, root, "package.json", `{"packageManager":"pnpm@12.3.4"}`)
	buildWrite(t, root, "pnpm-lock.yaml", "lockfileVersion: '9.0'\nimporters:\n  .: {}\n")
	buildWrite(t, root, "packages/lib/dev.sh", "echo DEV_STARTED\n")
	buildWrite(t, root, "tools/pnpm", `#!/bin/sh
case "$1" in
--version) echo 12.3.4;;
--config.verify-deps-before-run=error) test -f deps-current;;
install)
  printf '%s\n' "$*" >> installs
  test "$2" = --no-frozen-lockfile || { echo ERR_PNPM_PACKAGE_MANAGER_NO_IMPORTER >&2; exit 1; }
  mkdir -p node_modules/@build/lib
  touch deps-current
  ;;
*) exit 95;;
esac
`)
	for i := 0; i < 2; i++ {
		stdout, stderr, code := runBinaryIn(t, root, "dev", "lib", "--ui=stream", "-o", "text")
		if code != 0 || !strings.Contains(stdout+stderr, "DEV_STARTED") {
			t.Fatalf("run %d: exit=%d\n%s\n%s", i, code, stdout, stderr)
		}
	}
	installs, err := os.ReadFile(filepath.Join(root, "installs"))
	if err != nil || string(installs) != "install --no-frozen-lockfile\n" {
		t.Fatalf("installs=%q, err=%v", installs, err)
	}
}
