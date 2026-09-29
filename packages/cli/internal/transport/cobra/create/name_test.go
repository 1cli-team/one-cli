package createcmd

import (
	"path/filepath"
	"testing"
)

func TestWorkspaceNameUsesDirectoryUnlessExplicitlyOverridden(t *testing.T) {
	for _, tc := range []struct{ path, override, want string }{
		{filepath.Join(t.TempDir(), "demo"), "", "demo"},
		{filepath.Join(t.TempDir(), "projects", "demo") + string(filepath.Separator), "", "demo"},
		{filepath.Join(t.TempDir(), "invalid folder"), "custom-name", "custom-name"},
		{filepath.Join(t.TempDir(), "invalid folder"), "", ""},
	} {
		got, err := resolveWorkspaceName(tc.path, tc.override)
		if got != tc.want || (err != nil) != (tc.want == "") {
			t.Errorf("name(%q,%q)=%q,%v", tc.path, tc.override, got, err)
		}
	}
}
