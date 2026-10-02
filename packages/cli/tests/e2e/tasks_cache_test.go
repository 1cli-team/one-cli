package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func appendRootTaskConfig(t *testing.T, root, value string) {
	t.Helper()
	path := filepath.Join(root, "mise.local.toml")
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	current := map[string]any{}
	patch := map[string]any{}
	if len(raw) > 0 {
		if err = toml.Unmarshal(raw, &current); err != nil {
			t.Fatal(err)
		}
	}
	if err = toml.Unmarshal([]byte(value), &patch); err != nil {
		t.Fatal(err)
	}
	var merge func(map[string]any, map[string]any)
	merge = func(target, source map[string]any) {
		for key, value := range source {
			next, ok := value.(map[string]any)
			if old, exists := target[key].(map[string]any); ok && exists {
				merge(old, next)
			} else {
				target[key] = value
			}
		}
	}
	merge(current, patch)
	raw, err = toml.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	buildWrite(t, root, "mise.local.toml", string(raw))
}

func TestE2E_NativeMiseFileTask(t *testing.T) {
	root := buildFixture(t, true)
	buildWrite(t, root, ".mise/tasks/hello", "#!/bin/sh\necho native-file-task\n")
	for _, args := range [][]string{{"run", "-o", "json"}, {"run", "hello", "--dry-run", "-o", "json"}} {
		stdout, stderr, code := runBinaryIn(t, root, args...)
		if code != 0 || !strings.Contains(stdout, "//:hello") {
			t.Fatalf("%v: %d %s %s", args, code, stdout, stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "apps/web/mise.toml")); !os.IsNotExist(err) {
		t.Fatal("static discovery wrote managed configuration")
	}
	stdout, stderr, code := runBinaryIn(t, root, "run", "hello", "-o", "json")
	if code != 0 || !strings.Contains(stderr, "native-file-task") || !strings.Contains(stdout, "one-cli/task-result/v1") {
		t.Fatalf("%d %s %s", code, stdout, stderr)
	}
}

func TestE2E_NativeCacheNeedsNoOneCommand(t *testing.T) {
	root := buildFixture(t, true)
	t.Setenv("MISE_EXPERIMENTAL", "1")
	appendRootTaskConfig(t, root, "[tasks.\"web:build\"]\nsources=['package.json']\noutputs=['dist']\ncache={enabled=true}\n")
	stdout, stderr, code := runBinaryIn(t, root, "run", "build", "-p", "web", "-o", "json")
	if code != 0 {
		t.Fatalf("native cache failed: %d %s %s", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "order")); err != nil {
		t.Fatal("native cached task did not run")
	}
}

func TestE2E_NativeMiseIncludedTOMLTask(t *testing.T) {
	root := buildFixture(t, true)
	buildWrite(t, root, "mise.toml", "[tools]\nnode='system'\npnpm='system'\n[task_config]\nincludes=['tasks.toml']\n")
	buildWrite(t, root, "tasks.toml", "[hello]\nrun='echo included-task'\ndescription='Included task'\n")
	for _, args := range [][]string{{"run", "-o", "json"}, {"run", "hello", "--dry-run", "-o", "json"}, {"run", "hello", "-o", "json"}} {
		stdout, stderr, code := runBinaryIn(t, root, args...)
		if code != 0 || !strings.Contains(stdout, "//:hello") {
			t.Fatalf("%v: %d %s %s", args, code, stdout, stderr)
		}
	}
}

func TestE2E_GoTaskBuildRebuildsExecutableAndForwardsArguments(t *testing.T) {
	taskBinary, err := exec.LookPath("task")
	if err != nil {
		t.Skip("real Task is required for Go integration")
	}
	// Resolve the actual binary before isolating mise's data directory. A mise
	// shim would otherwise look for this tool inside the empty fixture cache.
	if out, err := exec.Command("mise", "which", "task").Output(); err == nil {
		taskBinary = strings.TrimSpace(string(out))
	}
	root := buildFixture(t, true)
	// Own the fixture's VCS boundary rather than inheriting a parent repository
	// or a sandbox's placeholder .git directory under the temporary root.
	gitInit := exec.Command("git", "init", "--quiet")
	gitInit.Dir = root
	if err := gitInit.Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(taskBinary, filepath.Join(root, "tools/task")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CGO_ENABLED", "0")
	buildWrite(t, root, "one.manifest.toml", `version = 2

[workspace]
id = "go-test"
name = "go-test"

[projects."api"]
path = "services/api"
toolchain = "go"
`)
	buildWrite(t, root, "mise.toml", "[tools]\ngo='system'\ntask='system'\nnode='system'\npnpm='system'\n")
	buildWrite(t, root, "services/api/go.mod", "module example.com/task-test\n\ngo 1.25.0\n")
	buildWrite(t, root, "services/api/cmd/server/main.go", "package main\nimport \"fmt\"\nvar message=\"default\"\nfunc main(){fmt.Print(message)}\n")
	scaffold, err := os.ReadFile(filepath.Join(repoRoot(t), "../templates/go-api/Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}
	buildWrite(t, root, "services/api/Taskfile.yml", string(scaffold))
	run := func(args ...string) {
		t.Helper()
		stdout, stderr, code := runBinaryIn(t, root, args...)
		if code != 0 {
			t.Fatalf("%v: %d %s %s", args, code, stdout, stderr)
		}
	}
	check := func(want string) {
		t.Helper()
		out, err := exec.Command(filepath.Join(root, "services/api/bin/server")).CombinedOutput()
		if err != nil || string(out) != want {
			t.Fatalf("built executable: %s %v, want %s", out, err, want)
		}
	}
	run("init", "mise")
	run("build", "-p", "api")
	check("default")
	if err = os.RemoveAll(filepath.Join(root, "services/api/bin")); err != nil {
		t.Fatal(err)
	}
	run("run", "build", "-p", "api")
	check("default")
	run("run", "build", "-p", "api", "--", "-ldflags=-X 'main.message=hello world'")
	check("hello world")
}

func TestE2E_ArtifactCacheStaysOffWithoutPublicFlag(t *testing.T) {
	root := buildFixture(t, true)
	// Ambient mise preferences cannot enable artifact caching through One.
	t.Setenv("MISE_EXPERIMENTAL", "1")
	t.Setenv("MISE_TASK_CACHE", "local-only")
	t.Setenv("MISE_TASK_CACHE_DIR", t.TempDir())
	buildWrite(t, root, "apps/web/build.sh", "#!/bin/sh\nmkdir -p dist\necho artifact > dist/value\necho executed >> executions\nprintf '<%s>\\n' \"$@\"\n")
	appendRootTaskConfig(t, root, `[tasks."web:build"]
sources = ["package.json", "build.sh"]
outputs = ["dist"]
cache = { enabled = true }
`)
	run := func(extra ...string) string {
		t.Helper()
		args := append([]string{"run", "build", "-p", "web", "--ui", "stream"}, extra...)
		out, stderr, code := runBinaryIn(t, root, args...)
		if code != 0 {
			t.Fatalf("%v: %d %s %s", args, code, out, stderr)
		}
		return out + stderr
	}
	check := func(want int) {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, "apps/web/executions"))
		if err != nil || strings.Count(string(raw), "executed") != want {
			t.Fatalf("executions = %q, want %d: %v", raw, want, err)
		}
	}
	run()
	check(1)
	run() // Ordinary freshness checks are unchanged.
	check(1)
	if err := os.RemoveAll(filepath.Join(root, "apps/web/dist")); err != nil {
		t.Fatal(err)
	}
	run() // Missing outputs must be rebuilt instead of restored from cache.
	check(2)
	run("--force")
	check(3)
	out := run("--", "a b", "中文", "$(nope)")
	check(4)
	for _, want := range []string{"<a b>", "<中文>", "<$(nope)>"} {
		if !strings.Contains(out, want) {
			t.Fatalf("lost argv %q: %s", want, out)
		}
	}
	if _, _, code := runBinaryIn(t, root, "run", "build", "--cache", "off"); code == 0 {
		t.Fatal("removed --cache flag was accepted")
	}
	for _, name := range []string{"__task", "__task-input", "__exec"} {
		if _, _, code := runBinaryIn(t, root, name); code == 0 {
			t.Fatalf("retired command %s is available", name)
		}
	}
}
