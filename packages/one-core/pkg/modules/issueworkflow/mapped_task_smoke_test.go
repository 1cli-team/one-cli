package issueworkflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInstalledCodexAppServerRunsMappedTask(t *testing.T) {
	if os.Getenv("ONE_GATEWAY_CODEX_SMOKE") != "1" {
		t.Skip("set ONE_GATEWAY_CODEX_SMOKE=1 to use the installed Codex App Server")
	}
	codex, err := exec.LookPath("codex")
	if err != nil {
		t.Fatal(err)
	}
	state, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(state, "checkout")
	if err := os.Mkdir(workspace, 0700); err != nil {
		t.Fatal(err)
	}
	one := os.Getenv("ONE_WORKFLOW_ONE_SMOKE")
	check := "touch verified"
	if one != "" {
		check = "one --version > verified"
	}
	for name, text := range map[string]string{
		"one.manifest.toml": "version = 2\n[workspace]\nid = 'one-gateway-check'\nname = 'Gateway test'\n",
		"mise.toml":         "monorepo_root = true\n[tasks.check]\nrun = '" + check + "'\n",
	} {
		if err := os.WriteFile(filepath.Join(workspace, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	executor := Codex{Executable: codex, StateDir: state, One: one}
	job := Job{Worktree: workspace, Project: Project{Check: checkCommand("//:check")}}
	if err := executor.Verify(t.Context(), job, ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(workspace, "verified"))
	if err != nil {
		t.Fatal(err)
	}
	if one != "" && len(data) == 0 {
		t.Fatal("One CLI produced no version output inside the Codex session")
	}
}
