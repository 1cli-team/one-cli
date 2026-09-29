package miseconfig

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestGeneratedTasksDoNotEnableExperimentalCaching(t *testing.T) {
	root := fixture(t)
	plan, err := Build(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err = plan.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, Filename))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = toml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc["settings"]; ok {
		t.Fatal("generated mise settings", doc["settings"])
	}
	for name, value := range doc["tasks"].(map[string]any) {
		task := value.(map[string]any)
		for _, field := range []string{"cache", "sources", "outputs"} {
			if _, ok := task[field]; ok {
				t.Fatalf("generated %s.%s", name, field)
			}
		}
	}
}

func TestGenerationPreservesExplicitUserCacheAndFreshness(t *testing.T) {
	root := fixture(t)
	userConfig := `[settings]
experimental = true
[tasks."web:build"]
dir = "apps/web"
run = "pnpm run build"
sources = ["src/**/*", "package.json"]
outputs = ["custom-dist"]
cache = { enabled = true, env = ["NODE_ENV"] }
`
	writeFixture(t, filepath.Join(root, Filename), userConfig)
	plan, err := Build(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err = plan.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, Filename))
	if err != nil {
		t.Fatal(err)
	}
	var before, after map[string]any
	if err = toml.Unmarshal([]byte(userConfig), &before); err != nil {
		t.Fatal(err)
	}
	if err = toml.Unmarshal(raw, &after); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before["settings"], after["settings"]) {
		t.Fatal("changed user settings")
	}
	want := before["tasks"].(map[string]any)["web:build"].(map[string]any)
	got := after["tasks"].(map[string]any)["web:build"].(map[string]any)
	for key, value := range want {
		if !reflect.DeepEqual(value, got[key]) {
			t.Fatalf("changed user field %s: %#v", key, got[key])
		}
	}
}
