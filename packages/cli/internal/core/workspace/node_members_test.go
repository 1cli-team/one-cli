package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeNodeMember(t *testing.T, root, rel, content string) {
	t.Helper()
	file := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNodeMembersRespectDeclarationsAndExclusions(t *testing.T) {
	root := t.TempDir()
	writeNodeMember(t, root, "apps/desktop/package.json", `{"workspaces":{"packages":["apps/*","packages/*","!apps/unused"]}}`)
	for _, dir := range []string{"apps/ui", "apps/main", "apps/unused", "packages/preload", "fixtures/ignored", "node_modules/ignored"} {
		writeNodeMember(t, root, "apps/desktop/"+dir+"/package.json", `{}`)
	}
	dirs, err := NodeProjectPackageDirs(root, "apps/desktop", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"apps/desktop", "apps/desktop/apps/main", "apps/desktop/apps/ui", "apps/desktop/packages/preload"}
	if !reflect.DeepEqual(dirs, want) {
		t.Fatalf("members=%v", dirs)
	}
}

func TestNodeMembersRejectEscapesAndSymlinks(t *testing.T) {
	root := t.TempDir()
	for _, pattern := range []string{"../outside", "/tmp/outside", "apps/../../outside", "node_modules/*", "apps/**", "[invalid"} {
		t.Run(pattern, func(t *testing.T) {
			writeNodeMember(t, root, "apps/desktop/package.json", `{"workspaces":["`+pattern+`"]}`)
			if _, err := NodeProjectPackageDirs(root, "apps/desktop", nil); err == nil {
				t.Fatal("invalid pattern accepted")
			}
		})
	}
	outside := t.TempDir()
	writeNodeMember(t, outside, "package.json", `{}`)
	writeNodeMember(t, root, "apps/desktop/package.json", `{"workspaces":["linked"]}`)
	if err := os.Symlink(outside, filepath.Join(root, "apps/desktop/linked")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := NodeProjectPackageDirs(root, "apps/desktop", nil); err == nil {
		t.Fatal("external symlink accepted")
	}
}

func TestCompositeDependencyStatusIncludesInternalPackages(t *testing.T) {
	root := t.TempDir()
	writeNodeMember(t, root, "apps/desktop/package.json", `{"workspaces":["apps/ui"]}`)
	writeNodeMember(t, root, "apps/desktop/apps/ui/package.json", `{"dependencies":{"renderer":"1.0.0"}}`)
	project := filepath.Join(root, "apps/desktop")
	if ProjectDependenciesInstalled(root, project, "node") {
		t.Fatal("missing internal dependencies reported ready")
	}
	writeNodeMember(t, root, "apps/desktop/apps/ui/node_modules/renderer/package.json", `{}`)
	if !ProjectDependenciesInstalled(root, project, "node") {
		t.Fatal("installed internal dependency reported missing")
	}
}
