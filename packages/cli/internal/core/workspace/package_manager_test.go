package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePackageManagerPrecedence(t *testing.T) {
	root := t.TempDir()
	put := func(name, contents string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	check := func(fallback, want string) {
		t.Helper()
		got, err := ResolvePackageManager(root, fallback)
		if err != nil || got != want {
			t.Fatalf("got %q %v, want %q", got, err, want)
		}
	}
	check("", "pnpm")
	put("package-lock.json", "{}")
	check("", "npm")
	check("yarn@4.0.0", "yarn")
	put("package.json", `{"packageManager":"bun@1.0.0"}`)
	check("yarn", "bun")
	put("package.json", `{"packageManager":"unknown@1.0.0"}`)
	if _, err := ResolvePackageManager(root, ""); err == nil {
		t.Fatal("invalid manager accepted")
	}
	put("package.json", "malformed")
	if _, err := ResolvePackageManager(root, ""); err == nil {
		t.Fatal("invalid package ignored")
	}
}
