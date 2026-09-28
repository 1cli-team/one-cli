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
	if _, err := ResolvePackageManager(root, ""); err == nil {
		t.Fatal("unsupported lockfile accepted")
	}
	for _, manager := range []string{"npm", "yarn", "bun"} {
		put("package.json", `{"packageManager":"pnpm@12.3.4"}`)
		if _, err := ResolvePackageManager(root, manager); err == nil {
			t.Fatal("unsupported project manager accepted", manager)
		}
		put("package.json", `{"packageManager":"`+manager+`@1.0.0"}`)
		if _, err := ResolvePackageManager(root, "pnpm"); err == nil {
			t.Fatal("unsupported workspace manager accepted", manager)
		}
	}
	put("package.json", `{"packageManager":"unknown@1.0.0"}`)
	if _, err := ResolvePackageManager(root, ""); err == nil {
		t.Fatal("invalid manager accepted")
	}
	put("package.json", "malformed")
	if _, err := ResolvePackageManager(root, ""); err == nil {
		t.Fatal("invalid package ignored")
	}
}
