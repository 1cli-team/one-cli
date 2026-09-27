package creation

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"gopkg.in/yaml.v3"
)

func TestElectronProjectsShareRootWorkspace(t *testing.T) {
	s := newCreationService(t)
	root := filepath.Join(t.TempDir(), "desktop workspace")
	if _, err := s.CreateWorkspace(context.Background(), WorkspaceInput{TargetDir: root, Name: "demo"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"desktop", "studio"} {
		if err := addLanguageProject(t, s, root, "electron-app", name); err != nil {
			t.Fatal(err)
		}
	}
	manifest, err := workspace.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Projects) != 2 {
		t.Fatalf("internal packages became One projects: %+v", manifest.Projects)
	}
	var config struct {
		Packages    []string        `yaml:"packages"`
		AllowBuilds map[string]bool `yaml:"allowBuilds"`
	}
	raw, err := os.ReadFile(filepath.Join(root, "pnpm-workspace.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	if !config.AllowBuilds["electron"] {
		t.Fatal("missing Electron installation policy")
	}
	for _, name := range []string{"desktop", "studio"} {
		base := "apps/" + name
		for _, member := range []string{"apps/electron", "apps/ui", "packages/preload"} {
			want := base + "/" + member
			found := false
			for _, pattern := range config.Packages {
				found = found || pattern == want
			}
			if !found {
				t.Errorf("missing member %s in %s", want, raw)
			}
			pkg, err := workspace.ReadPackageJSON(filepath.Join(root, want))
			if err != nil {
				t.Fatal(err)
			}
			if pkg.Name != "@"+name+"/"+filepath.Base(member) {
				t.Fatalf("wrong package name: %+v", pkg)
			}
			if member != "packages/preload" && pkg.Dependencies["@"+name+"/preload"] != "workspace:*" {
				t.Fatalf("wrong dependency: %+v", pkg)
			}
		}
		for _, file := range []string{"pnpm-lock.yaml", "pnpm-workspace.yaml", ".npmrc"} {
			if _, err := os.Stat(filepath.Join(root, base, file)); !os.IsNotExist(err) {
				t.Errorf("unexpected project-owned %s", file)
			}
		}
		if err := filepath.WalkDir(filepath.Join(root, base), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(data), "@app/") || strings.Contains(string(data), "{{projectName") || strings.HasSuffix(path, ".hbs") {
				t.Errorf("unrendered reference in %s", path)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}

	// Exercise the generated filter scripts with pnpm and local-only packages.
	// Stub package commands avoid a GUI/network while preserving the real
	// package names, workspace links and parent orchestration scripts.
	pnpm, err := exec.LookPath("pnpm")
	if err != nil {
		t.Skip("pnpm is not installed")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("CI", "true")
	rewrite := func(rel string, internal bool) {
		file := filepath.Join(root, rel, "package.json")
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var pkg map[string]any
		if err := json.Unmarshal(raw, &pkg); err != nil {
			t.Fatal(err)
		}
		delete(pkg, "packageManager")
		delete(pkg, "engines")
		delete(pkg, "devDependencies")
		deps := map[string]any{}
		if current, ok := pkg["dependencies"].(map[string]any); ok {
			for key, value := range current {
				if strings.HasPrefix(key, "@desktop/") || strings.HasPrefix(key, "@studio/") {
					deps[key] = value
				}
			}
		}
		pkg["dependencies"] = deps
		if internal {
			command := `node -e "console.log('MEMBER:` + pkg["name"].(string) + `')"`
			pkg["scripts"] = map[string]string{"dev": command, "build": command}
		}
		out, err := json.MarshalIndent(pkg, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, out, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rewrite(".", false)
	for _, name := range []string{"desktop", "studio"} {
		rewrite("apps/"+name, false)
		for _, member := range []string{"apps/electron", "apps/ui", "packages/preload"} {
			rewrite("apps/"+name+"/"+member, true)
		}
	}
	install := exec.Command(pnpm, "install", "--offline", "--no-frozen-lockfile")
	install.Dir = root
	if out, err := install.CombinedOutput(); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	for _, name := range []string{"desktop", "studio"} {
		for _, task := range []string{"dev", "build"} {
			cmd := exec.Command(pnpm, "run", task)
			cmd.Dir = filepath.Join(root, "apps", name)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s %s: %v\n%s", name, task, err, out)
			}
			text := string(out)
			preload := strings.Index(text, "MEMBER:@"+name+"/preload")
			for _, member := range []string{"electron", "ui"} {
				if index := strings.Index(text, "MEMBER:@"+name+"/"+member); preload < 0 || index <= preload {
					t.Fatalf("preload did not finish first: %s", out)
				}
			}
			other := "studio"
			if name == "studio" {
				other = "desktop"
			}
			if strings.Contains(text, "MEMBER:@"+other+"/") {
				t.Fatalf("started another application: %s", out)
			}
		}
	}
	lock, err := os.ReadFile(filepath.Join(root, "pnpm-lock.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"desktop", "studio"} {
		for _, member := range []string{"apps/electron", "apps/ui", "packages/preload"} {
			if !strings.Contains(string(lock), "apps/"+name+"/"+member+":") {
				t.Errorf("missing lockfile importer for %s/%s", name, member)
			}
		}
	}
}

func TestElectronPreservesRootSettings(t *testing.T) {
	for _, policy := range []string{"", "allowBuilds:\n  electron: false\n  esbuild: true\n", "onlyBuiltDependencies: [esbuild]\n"} {
		t.Run(policy, func(t *testing.T) {
			s := newCreationService(t)
			root := filepath.Join(t.TempDir(), "demo")
			if _, err := s.CreateWorkspace(context.Background(), WorkspaceInput{TargetDir: root, Name: "demo"}); err != nil {
				t.Fatal(err)
			}
			before := "# user settings\npackages: [apps/*]\nminimumReleaseAge: 1440\n" + policy
			if err := os.WriteFile(filepath.Join(root, "pnpm-workspace.yaml"), []byte(before), 0o644); err != nil {
				t.Fatal(err)
			}
			npmrc := "registry=https://registry.npmjs.org/\n"
			if err := os.WriteFile(filepath.Join(root, ".npmrc"), []byte(npmrc), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := addLanguageProject(t, s, root, "electron-app", "desktop"); err != nil {
				t.Fatal(err)
			}
			raw, _ := os.ReadFile(filepath.Join(root, "pnpm-workspace.yaml"))
			var cfg map[string]any
			if err := yaml.Unmarshal(raw, &cfg); err != nil {
				t.Fatal(err)
			}
			if cfg["minimumReleaseAge"] != 1440 || !strings.Contains(string(raw), "# user settings") {
				t.Fatalf("lost user config: %s", raw)
			}
			if policy == "" && cfg["allowBuilds"].(map[string]any)["electron"] != true {
				t.Fatal("missing default build policy")
			}
			if strings.HasPrefix(policy, "allowBuilds") && cfg["allowBuilds"].(map[string]any)["electron"] != false {
				t.Fatal("overwrote explicit denial")
			}
			if strings.HasPrefix(policy, "onlyBuilt") && cfg["allowBuilds"] != nil {
				t.Fatal("mixed old and new build policies")
			}
			after, _ := os.ReadFile(filepath.Join(root, ".npmrc"))
			if string(after) != npmrc {
				t.Fatal("changed user registry")
			}
		})
	}
}

func TestElectronRejectsOtherManagerWithoutPartialWrites(t *testing.T) {
	s := newCreationService(t)
	root := filepath.Join(t.TempDir(), "demo")
	if _, err := s.CreateWorkspace(context.Background(), WorkspaceInput{TargetDir: root, Name: "demo"}); err != nil {
		t.Fatal(err)
	}
	pkg := `{"private":true,"packageManager":"npm@11.0.0"}`
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(root, "one.manifest.json"))
	if err := addLanguageProject(t, s, root, "electron-app", "desktop"); err == nil {
		t.Fatal("unsupported manager accepted")
	}
	after, _ := os.ReadFile(filepath.Join(root, "one.manifest.json"))
	if string(before) != string(after) {
		t.Fatal("changed manifest on failure")
	}
	if _, err := os.Stat(filepath.Join(root, "apps/desktop")); !os.IsNotExist(err) {
		t.Fatal("partial project left after failure")
	}
	after, _ = os.ReadFile(filepath.Join(root, "package.json"))
	if string(after) != pkg {
		t.Fatal("overwrote root manager")
	}
}

func TestElectronRejectsNormalizedNameCollision(t *testing.T) {
	s := newCreationService(t)
	root := filepath.Join(t.TempDir(), "demo")
	if _, err := s.CreateWorkspace(context.Background(), WorkspaceInput{TargetDir: root, Name: "demo"}); err != nil {
		t.Fatal(err)
	}
	if err := addLanguageProject(t, s, root, "electron-app", "DesktopApp"); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(root, "pnpm-workspace.yaml"))
	if err := addLanguageProject(t, s, root, "electron-app", "desktop-app"); err == nil {
		t.Fatal("duplicate normalized scope accepted")
	}
	after, _ := os.ReadFile(filepath.Join(root, "pnpm-workspace.yaml"))
	if string(before) != string(after) {
		t.Fatal("failed add changed workspace")
	}
	if _, err := os.Stat(filepath.Join(root, "apps/desktop-app")); !os.IsNotExist(err) {
		t.Fatal("failed add left partial project")
	}
}
