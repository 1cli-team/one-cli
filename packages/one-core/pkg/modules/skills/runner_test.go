package skills

import (
	"context"
	"os/exec"
	"testing"
)

func TestHashMatchesUpstreamPathOrdering(t *testing.T) {
	// The fixture digest was generated with this locale. Production follows
	// the same environment locale as the upstream Skills CLI.
	t.Setenv("LC_ALL", "C")
	t.Setenv("LANG", "C")
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node is unavailable")
	}
	files := Files{
		"SKILL.md":            []byte("guidance\n"),
		"references/a.md":     []byte("a\n"),
		"references/Z.md":     []byte("Z\n"),
		"references/_note.md": []byte("note\n"),
		"references/中文.md":    []byte("中文\n"),
	}
	hashes, err := (Runner{}).Hash(context.Background(), t.TempDir(), map[string]Files{"one-cli": files})
	if err != nil {
		t.Fatal(err)
	}
	// skills@1.7.0 sorts with localeCompare, then hashes each path and its bytes.
	const expected = "5ca0c2ce3acd873d899879f864f0f7aca9632b0be6e97f5db8c357f43e5c10eb"
	if hashes["one-cli"] != expected {
		t.Fatalf("hash = %s, want %s", hashes["one-cli"], expected)
	}
}
