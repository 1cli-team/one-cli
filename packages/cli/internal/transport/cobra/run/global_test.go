package runcmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecretWriterMasksAcrossChunks(t *testing.T) {
	for _, size := range []int{1, 2, 5, 100} {
		var out bytes.Buffer
		w := newSecretWriter(&out, map[string]string{"A": "very-secret", "B": "short", "C": "very-secret-long", "EMPTY": ""})
		input := "before very-secret-long and short after very-secret"
		for i := 0; i < len(input); i += size {
			end := min(i+size, len(input))
			if _, e := w.Write([]byte(input[i:end])); e != nil {
				t.Fatal(e)
			}
		}
		if e := w.Close(); e != nil {
			t.Fatal(e)
		}
		if got := out.String(); got != "before [REDACTED] and [REDACTED] after [REDACTED]" {
			t.Fatalf("chunk %d: %q", size, got)
		}
	}
}
func TestGlobalCommandEnvRejectsRepositoryAndRelativePATH(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	env := globalCommandEnv([]string{"OTHER=keep", "PATH=" + strings.Join([]string{".", "relative", filepath.Join(root, "node_modules", ".bin"), binDir}, string(os.PathListSeparator))})
	if strings.Join(env, "\n") != "OTHER=keep\nPATH="+binDir {
		t.Fatal(env)
	}
	for _, k := range []string{"PATH", "LD_PRELOAD", "NODE_OPTIONS", "GIT_CONFIG_COUNT"} {
		if !reservedGlobalKey(k) {
			t.Fatal(k)
		}
	}
	if reservedGlobalKey("OSS_ACCESS_KEY_ID") {
		t.Fatal("blocked business credential")
	}
}
func TestGlobalRunRequiresExplicitScopeBeforeAuthentication(t *testing.T) {
	for _, flags := range []*runFlags{{global: true, envName: "prod"}, {global: true, globalPath: "/docker"}, {global: true, envName: "prod", globalPath: "/docker", project: "web"}} {
		if e := runGlobal(context.Background(), flags, []string{"echo"}); e == nil {
			t.Fatal("accepted ambiguous scope")
		}
	}
}
func TestGlobalDryRunDoesNotRequireLogin(t *testing.T) {
	e := runGlobal(context.Background(), &runFlags{global: true, envName: "prod", globalPath: "/docker", dryRun: true}, []string{os.Args[0]})
	if e != nil {
		t.Fatal(e)
	}
}
