package creation

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	internaltoolchain "github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/toolchain"
)

// This dependency-backed gate is explicit so ordinary Go unit tests remain
// offline. mise run check:templates and the Linux CI job enable it.
func TestRunnableTemplateSourcesAndProjects(t *testing.T) {
	if os.Getenv("ONE_TEST_TEMPLATE_BUILDS") != "1" {
		t.Skip("run mise run check:templates")
	}
	_, here, _, _ := runtime.Caller(0)
	repo := filepath.Clean(filepath.Join(filepath.Dir(here), "../../../../.."))
	sources := filepath.Join(repo, "packages/templates")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "CI=true")
		t.Logf("%s: %s", dir, strings.Join(args, " "))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
	}
	// Local go.work files make ordinary commands in the source directories work.
	for _, id := range []string{"go-lib", "go-api"} {
		run(filepath.Join(sources, id), "go", "test", "-mod=readonly", "./...")
		run(filepath.Join(sources, id), "go", "build", "-mod=readonly", "./...")
	}
	desktopSource := filepath.Join(sources, "electron-app")
	run(desktopSource, "pnpm", "install", "--no-frozen-lockfile")
	run(desktopSource, "pnpm", "run", "format:check")
	run(desktopSource, "pnpm", "run", "build")
	run(desktopSource, "pnpm", "run", "test")
	formatter := filepath.Join(desktopSource, "node_modules/.bin/oxfmt")
	if runtime.GOOS == "windows" {
		formatter += ".cmd"
	}
	t.Setenv("ONE_TEST_OXFMT_BINARY", formatter)
	internaltoolchain.RegisterBundled()
	TestGeneratedNodeProjectsPassFormatting(t)

	// Exercise a user-owned workspace with the repository's installed pnpm.
	// The generator must honor this configuration rather than the starter pin.
	s := newCreationService(t)
	root := filepath.Join(t.TempDir(), "workspace with spaces")
	if _, err := s.CreateWorkspace(ctx, WorkspaceInput{TargetDir: root, Name: "runnable"}); err != nil {
		t.Fatal(err)
	}
	sourcePackage, err := os.ReadFile(filepath.Join(desktopSource, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	manager := packageManagerFromTestJSON(t, sourcePackage)
	raw := []byte("{\"private\":true,\"packageManager\":" + manager + "}\n")
	if err := os.WriteFile(filepath.Join(root, "package.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pnpm-workspace.yaml"), []byte("packages: []\nallowBuilds:\n  electron: true\n  electron-winstaller: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, project := range []struct{ id, name string }{{"go-api", "api"}, {"go-lib", "shared"}, {"electron-app", "Alpha_Desktop"}, {"electron-app", "zulu-desktop"}} {
		if err := addLanguageProject(t, s, root, project.id, project.name); err != nil {
			t.Fatal(err)
		}
	}
	run(root, "pnpm", "install", "--no-frozen-lockfile")
	for _, dir := range []string{"services/api", "packages/shared"} {
		run(filepath.Join(root, dir), "go", "test", "-mod=readonly", "./...")
	}
	for _, project := range []struct{ name, pkg string }{{"Alpha_Desktop", "alpha-desktop"}, {"zulu-desktop", "zulu-desktop"}} {
		run(root, "pnpm", "--filter", project.pkg+"-main...", "run", "build")
		run(filepath.Join(root, "services", project.name+"-main"), "pnpm", "run", "test")
		for _, member := range []struct{ dir, suffix string }{{"apps", "-renderer"}, {"services", "-main"}, {"packages", "-preload"}} {
			run(filepath.Join(root, member.dir, project.name+member.suffix), formatter, "--check", ".")
		}
	}

}

func packageManagerFromTestJSON(t *testing.T, raw []byte) string {
	t.Helper()
	var pkg map[string]json.RawMessage
	if err := json.Unmarshal(raw, &pkg); err != nil {
		t.Fatal(err)
	}
	if len(pkg["packageManager"]) == 0 {
		t.Fatal("missing source packageManager")
	}
	return string(pkg["packageManager"])
}
