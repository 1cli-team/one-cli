//go:build !windows

package hooks

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
)

// Exercise hk itself: its file-list optimization resolves formatter output
// against the workspace root, even when a step runs inside a project directory.
func TestRealHKFixesProjectRelativeFilesWithoutStaging(t *testing.T) {
	binary := os.Getenv("ONE_TEST_HK_BINARY")
	if binary == "" {
		t.Skip("set ONE_TEST_HK_BINARY for native hk integration")
	}
	root := fixture(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	bin := t.TempDir()
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for name, script := range map[string]string{
		"mise": "shift; shift; exec \"$@\"\n",
		"pnpm": "shift; exec \"$@\"\n",
		"oxfmt": `mode=$1
shift; shift
case "$mode" in
  --check) [ "$(cat "$1")" = 'formatted' ] ;;
  --list-different) printf '%s\n' "$1"; exit 1 ;;
  --write) printf 'formatted\n' > "$1" ;;
  *) exit 99 ;;
esac
`,
	} {
		path := filepath.Join(bin, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(t, root, "apps/web/package.json", `{"devDependencies":{"oxfmt":"0.67.0"}}`)
	file := "apps/web/src/file with spaces.ts"
	write(t, root, file, "unformatted\n")
	p := fsutil.NewFilePlan(root)
	m := &workspace.Manifest{Projects: []workspace.ManifestProject{{Name: "web", RelativeDir: "apps/web", Toolchain: "node", PackageManager: "pnpm"}}}
	if err := PlanFiles(p, m); err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(context.Background(), root, "add", "--", file); err != nil {
		t.Fatal(err)
	}
	indexBefore, err := gitOutput(context.Background(), root, "diff", "--cached", "--binary")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		command string
		valid   bool
	}{{"check", false}, {"fix", true}, {"check", true}} {
		cmd := exec.Command(binary, test.command, file)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if (err == nil) != test.valid {
			t.Fatalf("hk %s: %v %s", test.command, err, out)
		}
		indexAfter, err := gitOutput(context.Background(), root, "diff", "--cached", "--binary")
		if err != nil || indexAfter != indexBefore {
			t.Fatalf("hk %s changed staged content: %v", test.command, err)
		}
	}
	if got := read(t, root, file); got != "formatted\n" {
		t.Fatalf("fix did not format the project file: %q", got)
	}
}
