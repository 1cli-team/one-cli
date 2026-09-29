package execcmd

import (
	"context"
	"errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	platformprocess "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

type recordingLoader struct {
	calls int
	err   error
}

func (l *recordingLoader) ID() string { return "infisical" }
func (l *recordingLoader) Load(context.Context, string, string, string) (map[string]string, error) {
	l.calls++
	return map[string]string{"TOKEN": "remote"}, l.err
}

func TestSecretsUseOnlyExplicitRemoteBinding(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("TOKEN=local\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		env      *workspace.EnvironmentConfig
		disabled bool
		calls    int
	}{
		{name: "unconfigured"},
		{name: "configured", env: &workspace.EnvironmentConfig{ProjectID: "remote"}, calls: 1},
		{name: "disabled", env: &workspace.EnvironmentConfig{ProjectID: "remote"}, disabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loader := &recordingLoader{}
			m := &workspace.Manifest{Env: tc.env, Projects: []workspace.ManifestProject{{Name: "web", RelativeDir: "apps/web", Env: &workspace.ProjectEnvOverride{Disabled: tc.disabled}}}}
			vars, _, err := loadRunSecrets(context.Background(), secrets.MustRegistry(loader), &runFlags{}, root, m, "apps/web")
			if err != nil || loader.calls != tc.calls {
				t.Fatalf("calls=%d err=%v", loader.calls, err)
			}
			if tc.calls == 0 && len(vars) != 0 {
				t.Fatalf("loaded local variables: %v", vars)
			}
			if tc.calls == 1 && vars["TOKEN"] != "remote" {
				t.Fatal("remote variables missing")
			}
		})
	}
}
func TestRemoteFailureDoesNotFallBackToLocalFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("TOKEN=local\n"), 0600); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("remote authentication failed")
	loader := &recordingLoader{err: failure}
	vars, _, err := loadRunSecrets(context.Background(), secrets.MustRegistry(loader), &runFlags{}, root, &workspace.Manifest{Env: &workspace.EnvironmentConfig{ProjectID: "remote"}}, "")
	if !errors.Is(err, failure) || vars != nil {
		t.Fatalf("vars=%v err=%v", vars, err)
	}
}

func TestWorkspaceExecPreservesChildOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	root := t.TempDir()
	manifest := `{"version":1,"workspace":{"id":"test","name":"test"},"environments":{"names":["dev"],"default":"dev"},"env":{"projectId":"remote"},"projects":[{"name":"web","relativeDir":"apps/web","toolchain":"node"}]}`
	if err := os.WriteFile(filepath.Join(root, "one.manifest.json"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "apps/web"), 0755); err != nil {
		t.Fatal(err)
	}
	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	errOut, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer errOut.Close()
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = out, errOut
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()
	ctx := execution.WithScope(context.Background(), execution.NewScope(context.Background(), root))
	err = runRun(ctx, secrets.MustRegistry(&recordingLoader{}), &runFlags{project: "web", runtime: runtimeport.Builtin}, []string{"sh", "-c", `printf '%s\n' "$TOKEN"; printf '%s\n' "$TOKEN" >&2; exit 6`})
	if platformprocess.ExitCode(err) != 6 {
		t.Fatal(err)
	}
	for _, path := range []string{out.Name(), errOut.Name()} {
		log, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(log), "remote") || strings.Contains(string(log), "[REDACTED]") {
			t.Fatalf("changed child output: %q", log)
		}
	}
}
