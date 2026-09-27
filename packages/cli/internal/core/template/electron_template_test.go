package template

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestElectronSandboxProfileUsesInstalledBinaryWithoutDisablingSandbox(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	root := t.TempDir()
	if err := Render("electron-app", root, CommonVariables("desktop", "pnpm")); err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(root, "apps/electron")
	module := filepath.Join(project, "node_modules/electron")
	if err := os.MkdirAll(module, 0o755); err != nil {
		t.Fatal(err)
	}
	binary := filepath.ToSlash(filepath.Join(root, "electron [dev]*", "electron"))
	value, _ := json.Marshal(binary)
	if err := os.WriteFile(filepath.Join(module, "index.js"), []byte("module.exports = "+string(value)), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, "script/sandbox-profile.mjs")
	cmd.Dir = project
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("profile generator: %v\n%s", err, out)
	}
	for _, want := range []string{"abi <abi/4.0>", "profile one-electron-", "userns,", `electron \[dev\]\*`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
	if strings.Contains(string(out), "sudo") || strings.Contains(string(out), "chmod") {
		t.Fatalf("profile contains shell commands: %s", out)
	}
	// Production/default development keeps Chromium's sandbox enabled.
	for _, rel := range []string{"package.json", "apps/electron/package.json", "apps/electron/src/index.ts", "apps/electron/src/windows/main.window.ts"} {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "--no-sandbox") || strings.Contains(string(raw), "sandbox: false") {
			t.Errorf("sandbox disabled in %s", rel)
		}
	}
	// The generator leaves applying privileges to the administrator.
	if _, err := os.Stat(filepath.Join(root, "one-electron.apparmor")); !os.IsNotExist(err) {
		t.Fatal("generator wrote a profile automatically")
	}
}
