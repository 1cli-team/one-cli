package miseconfig

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/resources/bundled"
)

func TestGoCacheIncludesExternalWorkspaceSourcesAndRejectsChangedBuild(t *testing.T) {
	root := fixture(t)
	external := t.TempDir()
	writeFixture(t, filepath.Join(external, "go.mod"), "module example.com/shared\ngo 1.25.0\n")
	writeFixture(t, filepath.Join(root, "go.work"), "go 1.25.0\nuse (\n ./services/api\n "+filepath.ToSlash(external)+"\n)\n")
	taskfile := filepath.Join(root, "services/api/Taskfile.yml")
	scaffold, err := bundled.TemplatesFS.ReadFile(bundled.TemplatesRoot + "/go-api/Taskfile.yml")
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, taskfile, string(scaffold))
	plan, err := Build(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err = plan.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	readBuild := func() Task {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, "services/api", Filename))
		if err != nil {
			t.Fatal(err)
		}
		var config config
		if err = toml.Unmarshal(raw, &config); err != nil {
			t.Fatal(err)
		}
		return config.Tasks["build"]
	}
	build := readBuild()
	if build.Cache == nil || !build.Cache.Enabled {
		t.Fatal("known Go build should cache")
	}
	relative, _ := filepath.Rel(filepath.Join(root, "services/api"), external)
	if !strings.Contains(strings.Join(build.Sources, "\n"), filepath.ToSlash(relative)+"/**/*") {
		t.Fatal("external Go member not hashed", build.Sources)
	}
	writeFixture(t, taskfile, "version: '3'\ntasks:\n  build:\n    cmds: ['go build -o other/server ./cmd/server']\n")
	plan, err = Build(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err = plan.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if readBuild().Cache.Enabled {
		t.Fatal("custom output cached with obsolete contract")
	}
}

func TestNodeCacheRequiresKnownOutputConfiguration(t *testing.T) {
	root := fixture(t)
	for _, name := range []string{"vite.config.ts", "tsconfig.json", "tsconfig.app.json", "tsconfig.node.json"} {
		raw, err := bundled.TemplatesFS.ReadFile(bundled.TemplatesRoot + "/react-spa/" + name)
		if err != nil {
			t.Fatal(err)
		}
		writeFixture(t, filepath.Join(root, "apps/web", name), string(raw))
	}
	writeFixture(t, filepath.Join(root, "apps/web/package.json"), `{"name":"web","scripts":{"build":"tsc -b && vite build"}}`)
	check := func(want bool) {
		t.Helper()
		plan, err := Build(root, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if err = plan.Apply(context.Background()); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(filepath.Join(root, "apps/web", Filename))
		var config config
		if err = toml.Unmarshal(raw, &config); err != nil {
			t.Fatal(err)
		}
		if config.Tasks["build"].Cache.Enabled != want {
			t.Fatalf("cache enabled = %v, want %v", config.Tasks["build"].Cache.Enabled, want)
		}
	}
	check(true)
	writeFixture(t, filepath.Join(root, "apps/web/vite.config.ts"), "export default {build:{outDir:'public-build'}}\n")
	check(false)
}

func TestSourceDependencyClosureIncludesLibrariesWithoutBuild(t *testing.T) {
	edges := map[string][]string{"web": {"middle"}, "middle": {"base"}}
	deps, err := dependencyClosure("web", edges)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(deps, ",") != "base,middle" {
		t.Fatal(deps)
	}
	edges["base"] = []string{"web"}
	if _, err = dependencyClosure("web", edges); err == nil {
		t.Fatal("source dependency cycle accepted")
	}
}
