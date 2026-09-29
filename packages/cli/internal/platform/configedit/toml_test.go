package configedit

import (
	"github.com/pelletier/go-toml/v2"
	"strings"
	"testing"
)

func merge(t *testing.T, before, desired string) string {
	t.Helper()
	out, err := TOML([]byte(before), []byte(desired))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := toml.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	return string(out)
}
func TestTOMLPreservesUserTextAndUpdatesOwnedFields(t *testing.T) {
	before := "# My tools\n[tools]\nnode = 'system' # local choice\n[env]\nKEEP = 'yes'\n[tasks.custom]\nrun = 'hello'\n[tasks.empty]\n"
	desired := "min_version = '2026.9.7'\n[tools]\nnode = '24.15.0'\n[tasks.build]\nrun = 'pnpm run build'\nraw_args = true\n"
	first := merge(t, before, desired)
	for _, text := range []string{"# My tools", "node = 'system' # local choice", "KEEP = 'yes'", "run = 'hello'", "[tasks.empty]"} {
		if !strings.Contains(first, text) {
			t.Fatalf("lost %q: %s", text, first)
		}
	}
	if got := merge(t, first, desired); got != first {
		t.Fatalf("not idempotent:\n%s", got)
	}
	user := strings.Replace(first, "raw_args = true", "raw_args = false # mine", 1) + "\n# trailing comment\n"
	if got := merge(t, user, desired); got != user {
		t.Fatalf("user changes not preserved: %s", got)
	}
	changed := strings.Replace(desired, "pnpm run build", "pnpm run newbuild", 1)
	got := merge(t, user, changed)
	if !strings.Contains(got, "pnpm run newbuild") || !strings.Contains(got, "false # mine") {
		t.Fatal(got)
	}
	conflict := strings.Replace(got, "pnpm run newbuild", "echo custom", 1)
	if _, err := TOML([]byte(conflict), []byte(desired)); err == nil {
		t.Fatal("expected same-entry conflict")
	}
}
func TestTOMLShapes(t *testing.T) {
	for _, before := range []string{"[tasks]\nbuild = { cache = { enabled = false }, description = 'mine' }\n", "tasks.build.cache.enabled = false\ntasks.build.description = 'mine'\n", "[tasks.build]\ncache = { enabled = false }\ndescription = 'mine'\n"} {
		t.Run(before, func(t *testing.T) {
			desired := "[tasks.build]\nrun = 'pnpm run build'\nraw_args = true\n[tasks.build.cache]\nenabled = true\nenv = ['VERSION']\n"
			first := merge(t, before, desired)
			if !strings.Contains(first, "'mine'") {
				t.Fatal(first)
			}
			if second := merge(t, first, desired); first != second {
				t.Fatalf("not stable: %s", second)
			}
			_ = merge(t, first, strings.Replace(desired, "raw_args = true", "raw_args = false", 1))
		})
	}
}
func TestTOMLCustomTaskAndAdvancedTools(t *testing.T) {
	before := "[tools]\nnode = { version = 'lts', postinstall = 'echo yes' }\n[tasks]\nbuild = 'my build'\ntest = { run = 'my test', description = 'mine' }\n"
	desired := "[tools]\nnode = '24.15.0'\n[tasks.build]\nrun = 'pnpm run build'\nraw_args = true\n[tasks.test]\nrun = 'pnpm run test'\nraw_args = true\n"
	if got := merge(t, before, desired); got != before {
		t.Fatalf("custom definitions changed: %s", got)
	}
}
func TestTOMLRemovesOwnedTaskAndPreservesComments(t *testing.T) {
	before := merge(t, "", "[tasks.build]\nrun = 'build'\n[tasks.build.cache]\nenabled = false\n[tasks.keep]\nrun = 'keep'\n")
	before = strings.Replace(before, "[tasks.build]", "# build comment\n[tasks.build]", 1)
	after := merge(t, before, "[tasks.keep]\nrun = 'keep'\n")
	if strings.Contains(after, "[tasks.build") || !strings.Contains(after, "# build comment") {
		t.Fatal(after)
	}
}
func TestTOMLMultilineArrayComments(t *testing.T) {
	first := merge(t, "", "[tasks.build]\nsources = ['a']\n")
	first = strings.Replace(first, "sources = ['a']", "sources = [\n  # keep this explanation\n  'a',\n] # trailing", 1)
	got := merge(t, first, "[tasks.build]\nsources = ['b']\n")
	for _, text := range []string{"# keep this explanation", "# trailing", "['b']"} {
		if !strings.Contains(got, text) {
			t.Fatal(got)
		}
	}
}
func TestTOMLMetadataInsideStringIsNotAComment(t *testing.T) {
	before := "[env]\nTEXT = '''\n# one:managed-v1 broken\n'''\n"
	got := merge(t, before, "[tools]\nnode = '24.15.0'\n")
	if !strings.Contains(got, before) {
		t.Fatal(got)
	}
}

func TestTOMLRootFieldsStayAtRoot(t *testing.T) {
	before := "[tools]\nnode = 'system'\npnpm = 'system'\n"
	desired := "min_version = '2026.9.7'\nmonorepo_root = true\n[tools]\nnode = '24.15.0'\n[monorepo]\nconfig_roots = ['apps/web']\nlockfile = false\n[tasks.build]\ndepends=['//apps/web:build']\n"
	result := merge(t, before, desired)
	var doc map[string]any
	_ = toml.Unmarshal([]byte(result), &doc)
	if doc["monorepo_root"] != true {
		t.Fatalf("misplaced root fields: %s", result)
	}
}

func TestTOMLAddsFieldsToPreviouslyGeneratedTasks(t *testing.T) {
	first := merge(t, "", "[tasks.build]\nrun = 'pnpm run build'\n[tasks.build.cache]\nenabled = false\n")
	desired := "[tasks.build]\nrun = 'pnpm run build'\ndepends = ['//packages/lib:build']\nsources = ['src/**']\noutputs = ['dist']\n[tasks.build.cache]\nenabled = true\ncommand_inputs = ['fingerprint']\n"
	got := merge(t, first, desired)
	var doc map[string]any
	_ = toml.Unmarshal([]byte(got), &doc)
	task, _ := lookup(doc, []string{"tasks", "build"})
	fields := task.(map[string]any)
	if fields["depends"] == nil || fields["sources"] == nil || fields["outputs"] == nil || fields["cache"].(map[string]any)["command_inputs"] == nil {
		t.Fatal(got)
	}
}

func TestTOMLDeletedDefaultStaysDeleted(t *testing.T) {
	desired := "[tasks.build]\nrun = 'build'\nraw_args = true\n"
	initial := merge(t, "", desired)
	user := strings.Replace(initial, "raw_args = true\n", "", 1)
	if got := merge(t, user, desired); got != user {
		t.Fatal(got)
	}
	if _, err := TOML([]byte(user), []byte(strings.Replace(desired, "true", "false", 1))); err == nil {
		t.Fatal("changed deleted field must conflict")
	}
	for _, metadata := range []string{"null", "broken"} {
		if _, err := TOML([]byte("# one:managed-v1 "+metadata), []byte(desired)); err == nil {
			t.Fatal("invalid metadata accepted")
		}
	}
}
