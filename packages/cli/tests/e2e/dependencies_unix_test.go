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

func TestE2E_DevPreparesDependenciesBeforeUpstreamBuildOnce(t *testing.T) {
	root := devTerminalFixture(t, true)
	buildWrite(t, root, "apps/web/dev.sh", "test -f ../../packages/lib/artifact || exit 94\necho DEV_STARTED\n")
	buildWrite(t, root, "tools/pnpm", `#!/bin/sh
case "$1" in
--version) echo 12.3.4;;
install)
  test "$2" = --no-frozen-lockfile || exit 92
  echo installed >> installs
  touch pnpm-lock.yaml
  mkdir -p node_modules
  ;;
run)
  test -f ../../pnpm-lock.yaml || exit 93
  shift 2
  exec sh build.sh "$@"
  ;;
*) exit 95;;
esac
`)
	stdout, stderr, code := runBinaryIn(t, root, "run", "dev", "-p", "web", "--ui=stream", "-o", "text")
	if code != 0 || !strings.Contains(stdout+stderr, "DEV_STARTED") {
		t.Fatalf("exit=%d\n%s\n%s", code, stdout, stderr)
	}
	installs, err := os.ReadFile(filepath.Join(root, "installs"))
	if err != nil || string(installs) != "installed\n" {
		t.Fatalf("installs=%q, err=%v", installs, err)
	}
	order, err := os.ReadFile(filepath.Join(root, "order"))
	if err != nil || string(order) != "lib\n" {
		t.Fatalf("only the upstream finite task should build: %q %v", order, err)
	}
}
