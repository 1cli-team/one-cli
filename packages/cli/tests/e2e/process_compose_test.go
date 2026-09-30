package cli_test

import (
	"encoding/json"
	tasks "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/tasks"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2E_ProcessComposeOrdersSilentStepsWaitsAndSharedAliases(t *testing.T) {
	root := buildFixture(t, true)
	appendRootTaskConfig(t, root, `
 [tasks.compose-demo]
 depends=["first","second","silent"]
 [tasks.seed]
 alias="same-seed"
 run="echo seed >> order; touch ready"
 [tasks.first]
 depends=["seed","same-seed"]
 run=["test -f ready; echo first-1 >> order", "sleep .1", "echo first-2 >> order"]
 [tasks.second]
 wait_for=["first"]
 run="echo second >> order"
 [tasks.silent]
 run="sleep .1"
 [tasks.unselected]
 run="{{ unsupported_expression }}"
 `)
	out, stderr, code := runBinaryIn(t, root, "run", "compose-demo", "-o", "json")
	if code != 0 {
		t.Fatalf("%d %s %s", code, out, stderr)
	}
	var result tasks.Result
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Tasks) != 5 {
		t.Fatal(result.Tasks)
	}
	for _, task := range result.Tasks {
		if task.Status != "succeeded" {
			t.Fatal(task)
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, "order"))
	if err != nil || string(raw) != "seed\nfirst-1\nfirst-2\nsecond\n" {
		t.Fatalf("order=%q err=%v", raw, err)
	}
}

func TestE2E_ProcessComposeFailsBeforeUnsupportedExecution(t *testing.T) {
	root := buildFixture(t, true)
	appendRootTaskConfig(t, root, `
 [tasks.unsupported-demo]
 run="touch SHOULD_NOT_RUN"
 depends=[{ task="seed", args=["value"] }]
 [tasks.seed]
 run="touch SHOULD_NOT_RUN"
 `)
	out, stderr, code := runBinaryIn(t, root, "run", "unsupported-demo", "-o", "json")
	if code == 0 || !strings.Contains(out+stderr, "depends") {
		t.Fatalf("%d %s %s", code, out, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "SHOULD_NOT_RUN")); !os.IsNotExist(err) {
		t.Fatal("unsupported graph executed")
	}
	if _, _, code := runBinaryIn(t, root, "run", "build", "--concurrency", "1"); code == 0 {
		t.Fatal("unsupported limit accepted")
	}
}
