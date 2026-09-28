package execcmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
