package createcmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRelativeOrAbsWithSymlinkPaths(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "parent-link")
	if err := os.Symlink(root, alias); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("directory symlinks unavailable: %v", err)
		}
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "existing"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, cwd, target, want string
		inPlace                 bool
	}{
		{name: "plain", cwd: root, target: filepath.Join(root, "demo"), want: "demo"},
		{name: "target-alias", cwd: root, target: filepath.Join(alias, "demo"), want: "demo"},
		{name: "cwd-alias", cwd: alias, target: filepath.Join(root, "demo"), want: "demo"},
		{name: "both-aliases", cwd: alias, target: filepath.Join(alias, "demo"), want: "demo"},
		{name: "missing-parents", cwd: root, target: filepath.Join(alias, "new", "nested", "demo"), want: filepath.Join("new", "nested", "demo")},
		{name: "existing-target", cwd: root, target: filepath.Join(alias, "existing"), want: "existing"},
		{name: "in-place", cwd: root, target: alias, inPlace: true, want: "."},
		{name: "sibling", cwd: filepath.Join(root, "existing"), target: filepath.Join(alias, "demo"), want: filepath.Join("..", "demo")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := relativeOrAbs(tc.cwd, tc.target, tc.inPlace); got != tc.want {
				t.Errorf("relativeOrAbs(%q, %q, %v) = %q; want %q", tc.cwd, tc.target, tc.inPlace, got, tc.want)
			}
		})
	}
}
