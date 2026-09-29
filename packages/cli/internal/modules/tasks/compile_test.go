package tasks

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestWaitsAreSelectedOnlyAndRejectCombinedCycles(t *testing.T) {
	w := taskWorkspace(t)
	writeTaskFile(t, w.Root(), "mise.toml", `
 [tasks.root]
 depends=["a","b"]
 [tasks.a]
 wait_for=["b","unselected"]
 [tasks.b]
 wait_for=["a"]
 [tasks.unselected]
 run="{{ dynamic }}"
 `)
	if _, err := NewPlan(w, Options{Name: "root"}); err == nil {
		t.Fatal("combined wait cycle accepted")
	}
	plan, err := NewPlan(w, Options{Name: "a"})
	if err != nil || len(plan.Tasks) != 1 || len(plan.Tasks[0].waitFor) != 0 {
		t.Fatalf("%+v %v", plan, err)
	}
}

func TestTaskEnvironmentIsPrivateAndUnsetOverrides(t *testing.T) {
	task := Task{}
	validateTaskFields(&task, map[string]any{"env": map[string]any{"VALUE": "synthetic-secret", "REMOVE": false}})
	if task.unsupported != "" || task.env["VALUE"] != "synthetic-secret" || len(task.unsetEnv) != 1 {
		t.Fatal(task)
	}
	raw, _ := json.Marshal(task)
	if strings.Contains(string(raw), "synthetic-secret") {
		t.Fatal("task plan exposes environment")
	}
	validateTaskFields(&task, map[string]any{"env": map[string]any{"REMOVE": "restored"}})
	if len(task.unsetEnv) != 0 {
		t.Fatal("unset persisted after overriding value")
	}
}

func TestFreshnessHandlesRecursiveGlobsAndMissingOutputs(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "src", "nested", "source.go")
	if err := os.MkdirAll(filepath.Dir(input), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, []byte("input"), 0600); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(input, past, past); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "result")
	if err := os.WriteFile(output, []byte("output"), 0600); err != nil {
		t.Fatal(err)
	}
	task := Task{Directory: dir, Sources: []string{"src/**/*.go"}, Outputs: []string{"result"}}
	if !fresh(task) {
		t.Fatal("recursive input failed freshness")
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(input, future, future); err != nil {
		t.Fatal(err)
	}
	if fresh(task) {
		t.Fatal("changed source reused output")
	}
	task.Sources = []string{"missing"}
	if fresh(task) {
		t.Fatal("missing source reused output")
	}
	task.Sources = []string{"src/**/*.go"}
	task.Outputs = append(task.Outputs, "missing")
	if fresh(task) {
		t.Fatal("missing output reused output")
	}
}

func TestFileTaskAliasesHiddenAndUnsupportedMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable scripts")
	}
	w := taskWorkspace(t)
	writeTaskFile(t, w.Root(), "mise-tasks/internal", "#!/bin/sh\n#MISE alias = 'shortcut'\n#MISE hide = true\nprintf ready\n")
	writeTaskFile(t, w.Root(), "mise-tasks/dynamic", "#!/bin/sh\n#MISE env = 42\nprintf ignored\n")
	for _, name := range []string{"internal", "dynamic"} {
		if err := os.Chmod(filepath.Join(w.Root(), "mise-tasks", name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := NewPlan(w, Options{Name: "shortcut"})
	if err != nil || len(plan.Tasks) != 1 || plan.Entries[0] != "//:internal" {
		t.Fatalf("%+v %v", plan, err)
	}
	catalog, err := (Service{}).Catalog(context.Background(), w)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range catalog {
		if task.Name == "//:internal" || task.Name == "//:shortcut" {
			t.Fatal("hidden task listed", task.Name)
		}
	}
	if _, err := NewPlan(w, Options{Name: "dynamic"}); err == nil {
		t.Fatal("invalid selected metadata accepted")
	}
}
