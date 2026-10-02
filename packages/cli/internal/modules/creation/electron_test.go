package creation

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/miseconfig"
	"gopkg.in/yaml.v3"
)

func TestElectronProjectsShareRootWorkspace(t *testing.T) {
	s := newCreationService(t)
	root := filepath.Join(t.TempDir(), "desktop workspace")
	if _, err := s.CreateWorkspace(context.Background(), WorkspaceInput{TargetDir: root, Name: "demo"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"desktop", "electron"} {
		if err := addLanguageProject(t, s, root, "electron-app", name); err != nil {
			t.Fatal(err)
		}
	}
	manifest, err := workspace.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Projects) != 6 || len(manifest.Groups) != 2 {
		t.Fatalf("manifest: %+v", manifest)
	}
	var config struct {
		Packages    []string        `yaml:"packages"`
		AllowBuilds map[string]bool `yaml:"allowBuilds"`
	}
	raw, _ := os.ReadFile(filepath.Join(root, "pnpm-workspace.yaml"))
	if err := yaml.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	if !config.AllowBuilds["electron"] {
		t.Fatal("missing install policy")
	}
	for _, name := range []string{"desktop", "electron"} {
		for _, part := range []struct{ dir, role string }{{"apps", "renderer"}, {"services", "main"}, {"packages", "preload"}} {
			rel := part.dir + "/" + name + "-" + part.role
			pkg, err := workspace.ReadPackageJSON(filepath.Join(root, rel))
			if err != nil {
				t.Fatal(err)
			}
			if pkg.Name != name+"-"+part.role {
				t.Fatalf("package: %+v", pkg)
			}
			if part.role != "preload" && pkg.Dependencies[name+"-preload"] != "workspace:*" {
				t.Fatalf("preload dependency: %+v", pkg)
			}
			raw, _ := os.ReadFile(filepath.Join(root, rel, "package.json"))
			if strings.Contains(string(raw), `"workspaces"`) {
				t.Fatal("nested workspace remained")
			}
			for _, file := range []string{"pnpm-lock.yaml", "pnpm-workspace.yaml", ".npmrc"} {
				if _, err := os.Stat(filepath.Join(root, rel, file)); !os.IsNotExist(err) {
					t.Fatalf("project-owned %s", file)
				}
			}
			if err := filepath.WalkDir(filepath.Join(root, rel), func(file string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					return nil
				}
				raw, err := os.ReadFile(file)
				if err != nil {
					return err
				}
				if strings.Contains(string(raw), "@one-template-electron/") || strings.Contains(string(raw), "{{projectName") || strings.HasSuffix(file, ".hbs") {
					t.Errorf("unrendered reference: %s", file)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := os.Stat(filepath.Join(root, "apps", name)); !os.IsNotExist(err) {
			t.Fatal("created nested project root")
		}
	}
	// Verify regenerated group tasks and packaging edges, without a GUI or network.
	plan, err := miseconfig.Build(root, miseconfig.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) != 0 {
		t.Fatalf("not idempotent: %+v", plan.Changes)
	}
	var mise struct {
		Tasks map[string]struct {
			Depends []string `toml:"depends"`
		} `toml:"tasks"`
	}
	raw, _ = os.ReadFile(filepath.Join(root, "mise.toml"))
	if err := toml.Unmarshal(raw, &mise); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"desktop", "electron"} {
		for _, op := range []string{"dev", "build", "test", "pack"} {
			if len(mise.Tasks[name+":"+op].Depends) == 0 {
				t.Fatalf("missing %s:%s", name, op)
			}
		}
		for _, role := range []string{"renderer", "main", "preload"} {
			want := "//:" + name + "-" + role + ":build"
			found := false
			for _, dep := range mise.Tasks[name+"-main:pack"].Depends {
				found = found || dep == want
			}
			if !found {
				t.Fatalf("missing pack dependency %s", want)
			}
		}
	}
}

func TestElectronCompositeNamesAndPreflightRollback(t *testing.T) {
	for _, name := range []string{"desktop", "electron", "Alpha_Desktop"} {
		t.Run(name, func(t *testing.T) {
			s := newCreationService(t)
			root := filepath.Join(t.TempDir(), "workspace")
			if _, err := s.CreateWorkspace(context.Background(), WorkspaceInput{TargetDir: root, Name: "demo"}); err != nil {
				t.Fatal(err)
			}
			if err := addLanguageProject(t, s, root, "electron-app", name); err != nil {
				t.Fatal(err)
			}
			m, err := workspace.ReadManifest(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(m.Groups) != 1 || m.Groups[0].Name != name {
				t.Fatalf("group: %+v", m.Groups)
			}
			for _, p := range m.Projects {
				if !strings.HasPrefix(p.Name, name+"-") {
					t.Fatalf("lost input name: %+v", p)
				}
			}
		})
	}
	for _, conflict := range []string{"services/desktop-main/keep.txt", "pnpm-workspace.yaml", "mise.toml"} {
		t.Run(conflict, func(t *testing.T) {
			s := newCreationService(t)
			root := filepath.Join(t.TempDir(), "workspace")
			if _, err := s.CreateWorkspace(context.Background(), WorkspaceInput{TargetDir: root, Name: "demo"}); err != nil {
				t.Fatal(err)
			}
			content := "user content"
			if conflict == "pnpm-workspace.yaml" {
				content = "packages: ['!services/*']\n"
			}
			if conflict == "mise.toml" {
				content = "# one:managed-v1 invalid\n"
			}
			dest := filepath.Join(root, conflict)
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(filepath.Join(root, "one.manifest.toml"))
			if err := addLanguageProject(t, s, root, "electron-app", "desktop"); err == nil {
				t.Fatal("accepted conflict")
			}
			after, _ := os.ReadFile(filepath.Join(root, "one.manifest.toml"))
			if string(before) != string(after) {
				t.Fatal("changed manifest")
			}
			after, _ = os.ReadFile(dest)
			if string(after) != content {
				t.Fatal("changed user file")
			}
			for _, rel := range []string{"apps/desktop-renderer", "packages/desktop-preload"} {
				if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
					t.Fatalf("partial output: %s", rel)
				}
			}
		})
	}
}

func TestElectronNativePackageGraphIsIsolated(t *testing.T) {
	pnpm, err := exec.LookPath("pnpm")
	if err != nil {
		t.Skip("pnpm is not installed")
	}
	s := newCreationService(t)
	root := filepath.Join(t.TempDir(), "workspace with spaces")
	if _, err := s.CreateWorkspace(context.Background(), WorkspaceInput{TargetDir: root, Name: "demo"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"desktop", "studio"} {
		if err := addLanguageProject(t, s, root, "electron-app", name); err != nil {
			t.Fatal(err)
		}
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("CI", "true")
	t.Setenv("npm_config_manage_package_manager_versions", "false")
	m, err := workspace.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range m.Projects {
		file := filepath.Join(root, p.RelativeDir, "package.json")
		raw, _ := os.ReadFile(file)
		var pkg map[string]any
		if err := json.Unmarshal(raw, &pkg); err != nil {
			t.Fatal(err)
		}
		deps := map[string]any{}
		for _, field := range []string{"dependencies", "devDependencies"} {
			if current, ok := pkg[field].(map[string]any); ok {
				for key, value := range current {
					if value == "workspace:*" {
						deps[key] = value
					}
				}
			}
		}
		delete(pkg, "devDependencies")
		pkg["dependencies"] = deps
		delete(pkg, "engines")
		delete(pkg, "packageManager")
		pkg["scripts"] = map[string]string{"build": `node -e "console.log('MEMBER:` + p.Name + `')"`}
		raw, _ = json.MarshalIndent(pkg, "", "  ")
		if err := os.WriteFile(file, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// This offline graph fixture has no external packages or manager downloads.
	rootPackage := filepath.Join(root, "package.json")
	raw, _ := os.ReadFile(rootPackage)
	var pkg map[string]any
	if err := json.Unmarshal(raw, &pkg); err != nil {
		t.Fatal(err)
	}
	delete(pkg, "packageManager")
	raw, _ = json.Marshal(pkg)
	if err := os.WriteFile(rootPackage, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	install := exec.Command(pnpm, "install", "--offline", "--no-frozen-lockfile")
	install.Dir = root
	if out, err := install.CombinedOutput(); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	for _, name := range []string{"desktop", "studio"} {
		cmd := exec.Command(pnpm, "--filter", name+"-main...", "run", "build")
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("build: %v\n%s", err, out)
		}
		text := string(out)
		preload := strings.Index(text, "MEMBER:"+name+"-preload")
		renderer := strings.Index(text, "MEMBER:"+name+"-renderer")
		main := strings.Index(text, "MEMBER:"+name+"-main")
		if preload < 0 || renderer <= preload || main <= renderer {
			t.Fatalf("incorrect order: %s", out)
		}
		other := "desktop"
		if name == other {
			other = "studio"
		}
		if strings.Contains(text, "MEMBER:"+other+"-") {
			t.Fatalf("started another group: %s", out)
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
				t.Fatalf("lost config: %s", raw)
			}
			if policy == "" && cfg["allowBuilds"].(map[string]any)["electron"] != true {
				t.Fatal("missing default policy")
			}
			if strings.HasPrefix(policy, "allowBuilds") && cfg["allowBuilds"].(map[string]any)["electron"] != false {
				t.Fatal("overwrote denial")
			}
			if strings.HasPrefix(policy, "onlyBuilt") && cfg["allowBuilds"] != nil {
				t.Fatal("mixed policies")
			}
			after, _ := os.ReadFile(filepath.Join(root, ".npmrc"))
			if string(after) != npmrc {
				t.Fatal("changed registry")
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
	before, _ := os.ReadFile(filepath.Join(root, "one.manifest.toml"))
	if err := addLanguageProject(t, s, root, "electron-app", "desktop"); err == nil {
		t.Fatal("accepted manager")
	}
	after, _ := os.ReadFile(filepath.Join(root, "one.manifest.toml"))
	if string(before) != string(after) {
		t.Fatal("changed manifest")
	}
	for _, rel := range []string{"apps/desktop-renderer", "services/desktop-main", "packages/desktop-preload"} {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Fatal("partial output")
		}
	}
	after, _ = os.ReadFile(filepath.Join(root, "package.json"))
	if string(after) != pkg {
		t.Fatal("changed manager")
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
		t.Fatal("accepted duplicate package names")
	}
	after, _ := os.ReadFile(filepath.Join(root, "pnpm-workspace.yaml"))
	if string(before) != string(after) {
		t.Fatal("changed workspace")
	}
	if _, err := os.Stat(filepath.Join(root, "apps/desktop-app-renderer")); !os.IsNotExist(err) {
		t.Fatal("partial project")
	}
}
