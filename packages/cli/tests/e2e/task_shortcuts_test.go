package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestE2E_TaskShortcutsAndBuiltinPrecedence(t *testing.T) {
	root := buildFixture(t, true)
	buildWrite(t, root, "mise.toml", `[tools]
node="system"
pnpm="system"
[tasks.hello]
run = 'sh hello.sh'
raw_args = true
[tasks.env]
run = 'echo TASK_ENV'
[tasks.dev]
run = 'echo ROOT_TASK_OVERRIDE'
depends = []
[tasks.build]
run = 'echo ROOT_TASK_OVERRIDE'
depends = []
[tasks.test]
run = 'echo ROOT_TASK_OVERRIDE'
depends = []
[tasks.lint]
run = 'echo ROOT_TASK_OVERRIDE'
depends = []
`)
	buildWrite(t, root, "hello.sh", "printf '<%s>\\n' \"$@\"\n")
	for _, prefix := range [][]string{{"hello"}, {"run", "hello"}} {
		args := append(append([]string{}, prefix...), "-o", "json", "--", "a b", "$(touch INJECTED)", "--output", "text", "--help")
		out, logs, code := runBinaryIn(t, root, args...)
		if code != 0 || !json.Valid([]byte(out)) || !strings.Contains(logs, "<a b>\n<$(touch INJECTED)>\n<--output>\n<text>\n<--help>") {
			t.Fatalf("exit=%d out=%s logs=%s", code, out, logs)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "INJECTED")); !os.IsNotExist(err) {
		t.Fatal("task argument was evaluated as shell code")
	}
	out, logs, code := runBinaryIn(t, root, "env", "--help")
	if code != 0 || strings.Contains(out+logs, "TASK_ENV") {
		t.Fatal(out, logs)
	}
	_, logs, code = runBinaryIn(t, root, "run", "env", "-o", "json")
	if code != 0 || !strings.Contains(logs, "TASK_ENV") {
		t.Fatal(logs)
	}
	for _, name := range []string{"dev", "build", "test", "lint"} {
		for _, args := range [][]string{{name}, {"run", name}} {
			_, logs, code = runBinaryIn(t, root, append(args, "-o", "json")...)
			if code != 0 || !strings.Contains(logs, "ROOT_TASK_OVERRIDE") {
				t.Fatal(args, code, logs)
			}
		}
	}
}

func TestE2E_TaskShortcutPreviewAndHelpAreStatic(t *testing.T) {
	root := buildFixture(t, true)
	buildWrite(t, root, "packages/lib/package.json", `{"name":"@build/lib","scripts":{"dev":"sh dev.sh","test":"sh test.sh","build":"sh build.sh","lint":"sh lint.sh"}}`)
	t.Setenv("ONE_MISE_BINARY", filepath.Join(root, "missing-mise"))
	for _, name := range []string{"dev", "build", "test", "lint"} {
		short, logs, code := runBinaryIn(t, root, name, "-p", "lib", "--dry-run", "-o", "json")
		if code != 0 {
			t.Fatal(name, logs)
		}
		long, logs, code := runBinaryIn(t, root, "run", name, "-p", "lib", "--dry-run", "-o", "json")
		if code != 0 {
			t.Fatal(name, logs)
		}
		if !reflect.DeepEqual(mustParseJSON(t, short), mustParseJSON(t, long)) {
			t.Fatalf("%s preview differs", name)
		}
	}
	outside := t.TempDir()
	for _, locale := range []string{"en_US.UTF-8", "zh_CN.UTF-8"} {
		t.Setenv("LC_ALL", locale)
		want, logs, code := runBinaryIn(t, outside, "run", "--help")
		if code != 0 || !strings.Contains(want, "--project") || strings.Contains(want, "tasks.") {
			t.Fatal(want, logs)
		}
		for _, dir := range []string{root, outside} {
			for _, name := range []string{"dev", "build", "test", "lint"} {
				for _, args := range [][]string{{name, "--help"}, {"run", name, "--help"}} {
					out, logs, code := runBinaryIn(t, dir, args...)
					if code != 0 || out != want {
						t.Fatalf("%s %v help differs from run: exit=%d out=%s logs=%s", locale, args, code, out, logs)
					}
				}
			}
		}
	}
	for _, path := range []string{"installs", "node_modules", "apps/web/mise.toml"} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatalf("preview wrote %s", path)
		}
	}
}

func TestE2E_NativeAndManifestDevForwardArguments(t *testing.T) {
	for _, manifest := range []bool{false, true} {
		t.Run(map[bool]string{false: "native", true: "manifest"}[manifest], func(t *testing.T) {
			root := buildFixture(t, true)
			if manifest {
				overrideDevCommand(t, root, "lib", "sh dev.sh")
			} else {
				buildWrite(t, root, "packages/lib/package.json", `{"name":"@build/lib","scripts":{"dev":"sh dev.sh"}}`)
			}
			buildWrite(t, root, "packages/lib/dev.sh", "printf '<%s>\\n' \"$@\"\n")
			_, logs, code := runBinaryIn(t, root, "dev", "-p", "lib", "-o", "json", "--", "a b", "$(touch INJECTED)", "--port", "4300")
			if code != 0 || !strings.Contains(logs, "<a b>\n<$(touch INJECTED)>\n<--port>\n<4300>") {
				t.Fatal(code, logs)
			}
			if _, err := os.Stat(filepath.Join(root, "packages/lib/INJECTED")); !os.IsNotExist(err) {
				t.Fatal("dev argument was evaluated")
			}
		})
	}
}

func TestE2E_DevelopmentRejectsBlockingModesBeforePreparation(t *testing.T) {
	root := buildFixture(t, true)
	for _, project := range []string{"web", "lib"} {
		overrideDevCommand(t, root, project, "echo DEV_SHOULD_NOT_START")
	}
	for _, locale := range []string{"zh_CN.UTF-8", "en_US.UTF-8"} {
		t.Setenv("LC_ALL", locale)
		for _, args := range [][]string{{"dev", "--ui", "raw"}, {"dev", "--concurrency", "1"}, {"test", "--", ":::"}} {
			out, logs, code := runBinaryIn(t, root, append(args, "--dry-run")...)
			if code == 0 || strings.Contains(out+logs, "DEV_SHOULD_NOT_START") || strings.Contains(logs, `"message": "tasks.`) {
				t.Fatal(args, code, out, logs)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(root, "installs")); !os.IsNotExist(err) {
		t.Fatal("validation installed dependencies")
	}
}
