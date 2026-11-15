package tasks

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	runtimeport "github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/runtime"
)

type mirrorEnvironmentProvider struct{ binary string }

func (p mirrorEnvironmentProvider) Prepare(_ context.Context, command runtimeport.Command) (runtimeport.Command, error) {
	return command, nil
}
func (p mirrorEnvironmentProvider) PrepareCLI(_ context.Context, command runtimeport.Command) (runtimeport.Command, error) {
	command.Argv = []string{p.binary}
	command.Env = []string{"PATH=/bin", "GOPROXY=https://project.internal"}
	return command, nil
}

func TestToolEnvironmentUsesPreparedDefaults(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX executable")
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "mise")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf '{}'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	s := Service{Provider: mirrorEnvironmentProvider{binary}}
	env, err := s.toolEnvironment(t.Context(), Task{Directory: dir}, []string{"GOPROXY=https://parent.internal"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(env, "GOPROXY=https://project.internal") || slices.Contains(env, "GOPROXY=https://parent.internal") {
		t.Fatalf("environment restored parent defaults: %q", env)
	}
}
