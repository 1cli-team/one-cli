package execcmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
