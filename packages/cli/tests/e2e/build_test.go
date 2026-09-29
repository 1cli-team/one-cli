package cli_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/miseconfig"
	buildmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/tasks"
	"gopkg.in/yaml.v3"
)

func buildWrite(t *testing.T, root, path, value string) {
	t.Helper()
	path = filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0755); err != nil {
		t.Fatal(err)
	}
}

// Fake package tools make dependency and runtime integration deterministic and
// offline while the real One binary executes the full build -> run pipeline.
func buildFixture(t *testing.T, mise bool) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fake package tools")
	}
	misePath, err := exec.LookPath("mise")
	if pinned := os.Getenv("ONE_TEST_MISE_BINARY"); pinned != "" {
		misePath, err = pinned, nil
	}
	if err != nil {
		t.Skip("real mise is required for task integration")
	}
	root := t.TempDir()
	// mise does not discover a monorepo rooted at HOME.
	isolateHome(t, t.TempDir())
	t.Setenv("ONE_RUNTIME", "builtin")
	t.Setenv("MISE_EXPERIMENTAL", "0")
	// Nested mise may select a different Go binary than the parent test task.
	// Let that binary discover its own matching standard library.
	t.Setenv("GOROOT", "")
	buildWrite(t, root, "one.manifest.toml", `version = 2

[workspace]
id = "build-test"
name = "build-test"

[projects."web"]
path = "apps/web"
toolchain = "node"

[projects."lib"]
path = "packages/lib"
toolchain = "node"

[projects."mobile"]
path = "apps/mobile"
toolchain = "node"
`)
	buildWrite(t, root, "package.json", `{"packageManager":"pnpm@12.3.4"}`)
	buildWrite(t, root, "apps/web/package.json", `{"name":"@build/web","scripts":{"build":"sh build.sh"},"dependencies":{"@build/lib":"workspace:*"}}`)
	buildWrite(t, root, "packages/lib/package.json", `{"name":"@build/lib","scripts":{"build":"sh build.sh"}}`)
	buildWrite(t, root, "apps/mobile/package.json", `{"name":"@build/mobile","scripts":{}}`)
	buildWrite(t, root, "packages/lib/build.sh", "#!/bin/sh\necho lib >> ../../order\necho built > artifact\nprintf 'library-output\\n'\n")
	buildWrite(t, root, "apps/web/build.sh", "#!/bin/sh\n[ -f ../../packages/lib/artifact ] || exit 93\necho web >> ../../order\nprintf 'web-env=%s runtime=%s\\n' \"$BUILD_VALUE\" \"${ONE_MISE_ONLY:-builtin}\"\n")
	buildWrite(t, root, "apps/web/.env", "BUILD_VALUE=base\n")
	buildWrite(t, root, "apps/web/.env.prod", "BUILD_VALUE=production\n")
	buildWrite(t, root, "tools/pnpm", "#!/bin/sh\ncase \"$1\" in\n--version) echo 12.3.4;;\ninstall) mkdir -p node_modules; echo installed >> installs;;\nrun) script=$2; shift 2; exec sh \"$script.sh\" \"$@\";;\n*) exit 95;;\nesac\n")
	buildWrite(t, root, "tools/node", "#!/bin/sh\necho v24.15.0\n")
	// One prepends the selected mise binary's directory to PATH. Keep it with
	// the fixture tools so a host pnpm cannot override the fake package manager.
	fixtureMise := filepath.Join(root, "tools/mise")
	if err := os.Symlink(misePath, fixtureMise); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Join(root, "tools")+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ONE_MISE_BINARY", fixtureMise)
	buildWrite(t, root, "mise.toml", "[tools]\nnode=\"system\"\npnpm=\"system\"\n")
	t.Setenv("MISE_TRUSTED_CONFIG_PATHS", root)
	for _, key := range []string{"MISE_CONFIG_DIR", "MISE_CACHE_DIR", "MISE_DATA_DIR", "MISE_STATE_DIR"} {
		t.Setenv(key, filepath.Join(root, key))
	}

	plan, e := miseconfig.Build(root, miseconfig.Options{})
	if e != nil {
		t.Fatal(e)
	}
	if e = plan.Apply(context.Background()); e != nil {
		t.Fatal(e)
	}
	return root
}

func TestE2E_BuildOrdersProjectsAndKeepsStructuredOutputClean(t *testing.T) {
	root := buildFixture(t, true)
	t.Setenv("BUILD_VALUE", "from-shell")
	stdout, stderr, code := runBinaryIn(t, filepath.Join(root, "apps/web"), "build", "--env", "prod", "-o", "json")
	if code != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	var result buildmodule.Result
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err, stdout)
	}
	if result.Schema != "one-cli/task-result/v1" || result.Status != "succeeded" || len(result.Tasks) != 3 {
		t.Fatal(result)
	}
	for _, task := range result.Tasks {
		if task.Status != "unknown" {
			t.Fatal(task)
		}
	}
	if !strings.Contains(stderr, "web-env=from-shell") || !strings.Contains(stderr, "library-output") {
		t.Fatal(stderr)
	}
	order, err := os.ReadFile(filepath.Join(root, "order"))
	if err != nil || string(order) != "lib\nweb\n" {
		t.Fatalf("%s %v", order, err)
	}
	installs, err := os.ReadFile(filepath.Join(root, "installs"))
	if err != nil || string(installs) != "installed\n" {
		t.Fatalf("%s %v", installs, err)
	}
}

func TestE2E_BuildFailureStopsRemainingTasksAndReturnsChildCode(t *testing.T) {
	root := buildFixture(t, false)
	buildWrite(t, root, "packages/lib/build.sh", "#!/bin/sh\necho build-failed\nexit 42\n")
	stdout, stderr, code := runBinaryIn(t, root, "build", "-o", "json")
	var result buildmodule.Result
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err, stdout, stderr)
	}
	if code != 42 || result.ExitCode != 42 || result.Status != "failed" {
		t.Fatalf("%d %+v %s", code, result, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "order")); !os.IsNotExist(err) {
		t.Fatal("later project executed")
	}
}

func TestE2E_BuildPreviewAndProjectSelection(t *testing.T) {
	root := buildFixture(t, true)
	// These invalid env contents and a nonexistent mise prove preview never loads
	// secrets or probes tools. The selected lib has no dev configuration.
	buildWrite(t, root, "packages/lib/.env", "INVALID=\"unterminated")
	t.Setenv("ONE_MISE_BINARY", filepath.Join(root, "missing-mise"))
	stdout, stderr, code := runBinaryIn(t, root, "build", "-p", "packages/lib", "-p", "lib", "--dry-run", "-o", "yaml")
	if code != 0 || stderr != "" {
		t.Fatalf("%d %s %s", code, stdout, stderr)
	}
	var result map[string]any
	if err := yaml.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	tasks := result["tasks"].([]any)
	if result["schema"] != "one-cli/task-plan/v1" || len(tasks) != 1 {
		t.Fatal(result)
	}
	for _, path := range []string{"installs", "order", "node_modules", "apps/web/mise.toml"} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatalf("dry-run wrote %s", path)
		}
	}
	for _, args := range [][]string{{"build", "-p", "mobile", "--dry-run"}, {"build", "-p", "unknown", "--dry-run"}, {"build", "-p", "web", "-p", "lib", "--dry-run", "--", "argument"}, {"build", "-p", "web", "--env", "typo", "--dry-run"}} {
		if _, _, code := runBinaryIn(t, root, args...); code == 0 {
			t.Fatal(args)
		}
	}
}

func TestE2E_GoLibraryTemplateHasBuildTask(t *testing.T) {
	root := t.TempDir()
	isolateHome(t, root)
	ws := bootstrapWorkspace(t, root, "go-build")
	_, stderr, code := runBinaryIn(t, ws, "add", "go-lib", "--name", "lib", "-y", "-o", "json")
	if code != 0 {
		t.Fatal(stderr)
	}
	stdout, stderr, code := runBinaryIn(t, ws, "build", "-p", "lib", "--dry-run", "-o", "json")
	if code != 0 {
		t.Fatalf("%d %s %s", code, stdout, stderr)
	}
	var plan buildmodule.Plan
	if err := json.Unmarshal([]byte(stdout), &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.Tasks) != 1 || plan.Tasks[0].Operation != "build" || plan.Tasks[0].Run != "task build --" {
		t.Fatal(plan)
	}
	raw, err := os.ReadFile(filepath.Join(ws, "packages/lib/Taskfile.yml"))
	if err != nil || !strings.Contains(string(raw), "go build {{.CLI_ARGS}} ./...") {
		t.Fatalf("%s %v", raw, err)
	}
	raw, err = os.ReadFile(filepath.Join(ws, "mise.toml"))
	if err != nil || !strings.Contains(string(raw), "[tasks.'lib:build']") {
		t.Fatalf("%s %v", raw, err)
	}
}

func syncFixtureTasks(t *testing.T, root string) {
	t.Helper()
	plan, err := miseconfig.Build(root, miseconfig.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err = plan.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
}
