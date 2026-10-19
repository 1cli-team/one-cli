package installation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPairUpgradePreservesRunningHelperAndRollsBackReceipt(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "one")
	source := filepath.Join(dir, "next")
	helper := filepath.Join(dir, "one-api")
	for path, value := range map[string]string{target: "cli-old", source: "cli-new", helper: "server-old"} {
		if err := os.WriteFile(path, []byte(value), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if IsBundled(target) {
		t.Fatal("standalone companion mistaken for Desktop bundle")
	}
	if err := ReplacePair(source, helper, target, "1.0.0"); err != nil {
		t.Fatal(err)
	}
	oldHelper := Companion(target)
	if data, err := os.ReadFile(oldHelper); err != nil || string(data) != "server-old" {
		t.Fatalf("helper: %s %v", data, err)
	}
	receipt, err := os.ReadFile(target + ".installation.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(helper, []byte("server-new"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = ReplacePair(filepath.Join(dir, "missing"), helper, target, "2.0.0"); err == nil {
		t.Fatal("missing CLI accepted")
	}
	after, err := os.ReadFile(target + ".installation.json")
	if err != nil || string(after) != string(receipt) {
		t.Fatalf("receipt not restored: %v", err)
	}
	if data, err := os.ReadFile(oldHelper); err != nil || string(data) != "server-old" {
		t.Fatalf("running helper replaced: %v", err)
	}
}
