package template

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNodePackageRenameIsBoundedAndKeepsDependencyOrder(t *testing.T) {
	files := map[string][]byte{
		"package.json": []byte(`{
  "name": "starter",
  "workspaces": ["packages/*"],
  "scripts": { "dev": "pnpm -F @starter/preload build && pnpm -F @starter/preload-extra build" },
  "custom": { "name": "@starter/preload" }
}`),
		"packages/preload/package.json": []byte(`{"name":"@starter/preload"}`),
		"packages/ui/package.json": []byte(`{
  "name": "@starter/ui",
  "dependencies": {
    "@starter/preload": "workspace:*",
    "@vendor/lib": "^1.0.0"
  },
  "peerDependencies": { "@starter/preload": "workspace:*" },
  "optionalDependencies": { "@starter/preload": "workspace:*" }
}`),
		"ui.ts": []byte("import '@starter/preload/channels';\n// @starter/preload\n"),
	}
	if err := rewriteNode(files, NodeSpec{Scope: "@starter", SourceFiles: []string{"ui.ts"}}, CommonVariables("Zulu_Desktop", "pnpm")); err != nil {
		t.Fatal(err)
	}
	root := string(files["package.json"])
	if !strings.Contains(root, "@zulu-desktop/preload build") || !strings.Contains(root, "@starter/preload-extra build") || !strings.Contains(root, `"custom": { "name": "@starter/preload" }`) {
		t.Fatal(root)
	}
	if string(files["ui.ts"]) != "import '@zulu-desktop/preload/channels';\n// @zulu-desktop/preload\n" {
		t.Fatal(string(files["ui.ts"]))
	}
	raw := string(files["packages/ui/package.json"])
	if strings.Index(raw, `"@vendor/lib"`) > strings.Index(raw, `"@zulu-desktop/preload"`) {
		t.Fatal("dependency order:", raw)
	}
	var pkg map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &pkg); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"dependencies", "peerDependencies", "optionalDependencies"} {
		var deps map[string]string
		if err := json.Unmarshal(pkg[key], &deps); err != nil {
			t.Fatal(err)
		}
		if deps["@zulu-desktop/preload"] != "workspace:*" {
			t.Fatal(key, deps)
		}
	}
}

func TestNodeRenameRejectsDependencyKeyCollision(t *testing.T) {
	files := map[string][]byte{
		"package.json":                  []byte(`{"name":"starter","workspaces":["packages/preload"],"dependencies":{"@starter/preload":"workspace:*","@target/preload":"1.0.0"}}`),
		"packages/preload/package.json": []byte(`{"name":"@starter/preload"}`),
	}
	if err := rewriteNode(files, NodeSpec{Scope: "@starter"}, CommonVariables("target", "pnpm")); err == nil {
		t.Fatal("overwrote external dependency")
	}
}
