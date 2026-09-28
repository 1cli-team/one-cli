package tasks

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/dependencies"
	process "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/ports/secrets"
)

func TestLeafMasksInjectedValuesBeforeSchedulerCanCacheLogs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	w := taskWorkspace(t)
	w.Manifest().Env = &workspace.EnvironmentConfig{ProjectID: "remote"}
	p, err := NewPlan(w, Options{Name: "build", Projects: []string{"web"}})
	if err != nil {
		t.Fatal(err)
	}
	loader := &countingLoader{calls: map[string]int{}, value: "private-test-value-"}
	env, _, cleanup, err := prepareContext(context.Background(), w, p, secrets.MustRegistry(loader))
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	for _, entry := range env {
		if key, value, ok := strings.Cut(entry, "="); ok && key == contextVariable {
			t.Setenv(key, value)
		}
	}
	binary := filepath.Join(w.Root(), "apps/web/node_modules/.bin/pnpm")
	writeTaskFile(t, w.Root(), "apps/web/node_modules/.bin/pnpm", "#!/bin/sh\nprintf '%s\\n' \"$VALUE\"\nprintf '%s\\n' \"$VALUE\" >&2\nprintf '\\033[32mordinary log\\033[0m\\n'\nexit 9\n")
	if err := os.Chmod(binary, 0755); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	err = ExecuteLeaf(context.Background(), w, "web", "build", nil, nil, &out, &errOut)
	if process.ExitCode(err) != 9 {
		t.Fatal(err)
	}
	for _, log := range []string{out.String(), errOut.String()} {
		if strings.Contains(log, "private-test-value-") || !strings.Contains(log, "[REDACTED]") {
			t.Fatalf("unsafe logs: %q", log)
		}
	}
	if !strings.Contains(out.String(), "\x1b[32mordinary log\x1b[0m") {
		t.Fatal("changed unrelated log formatting")
	}
}

type replayProvider struct{}

func (replayProvider) Prepare(_ context.Context, command runtimeport.Command) (runtimeport.Command, error) {
	return command, nil
}
func (replayProvider) PrepareCLI(_ context.Context, command runtimeport.Command) (runtimeport.Command, error) {
	script := "printf '{}'"
	if command.Argv[0] == "run" {
		script = "printf 'cache replay: private-test-value-apps/web\\n'; printf 'cache replay: private-test-value-packages/lib\\n' >&2"
	}
	command.Argv = []string{"sh", "-c", script}
	return command, nil
}
func TestSchedulerMasksCachedOutputAcrossProjects(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	w := taskWorkspace(t)
	w.Manifest().Env = &workspace.EnvironmentConfig{ProjectID: "remote"}
	opts := Options{Name: "build", Projects: []string{"web"}, Cache: "off", UI: "stream", Jobs: 1}
	plan, err := NewPlan(w, opts)
	if err != nil {
		t.Fatal(err)
	}
	service := Service{Provider: replayProvider{}, Loaders: secrets.MustRegistry(&countingLoader{calls: map[string]int{}, value: "private-test-value-"}), Prepare: func(context.Context, dependencies.Input) error { return nil }}
	var out, errOut bytes.Buffer
	result, err := service.Execute(context.Background(), w, plan, opts, nil, &out, &errOut)
	if err != nil || result.Status != "succeeded" {
		t.Fatalf("execution: %v %+v", err, result)
	}
	combined := out.String() + errOut.String()
	if strings.Contains(combined, "private-test-value-") || strings.Count(combined, "[REDACTED]") != 2 {
		t.Fatalf("unsafe cached logs: %q", combined)
	}
}

// The Dashboard worker merges both streams into one private console pipe.
func TestDevConsoleMasksBeforeForwardingAndReportsSchedulerStart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	w := taskWorkspace(t)
	w.Manifest().Env = &workspace.EnvironmentConfig{ProjectID: "remote"}
	for i := range w.Manifest().Projects {
		w.Manifest().Projects[i].Dev = &workspace.ProjectDevOverride{Command: "echo dev"}
	}
	if err := workspace.WriteManifest(w.Root(), w.Manifest()); err != nil {
		t.Fatal(err)
	}
	opts := Options{Name: "dev", Projects: []string{"web", "lib"}, Cache: "off", UI: "stream", Jobs: 1}
	plan, err := NewPlan(w, opts)
	if err != nil {
		t.Fatal(err)
	}
	started := false
	service := Service{Provider: replayProvider{}, Loaders: secrets.MustRegistry(&countingLoader{calls: map[string]int{}, value: "private-test-value-"}), Prepare: func(context.Context, dependencies.Input) error { return nil }, OnStarted: func() { started = true }}
	var console bytes.Buffer
	result, err := service.Execute(context.Background(), w, plan, opts, nil, &console, &console)
	if err != nil || result.Status != "succeeded" || !started {
		t.Fatalf("%+v %v %v", result, err, started)
	}
	if strings.Contains(console.String(), "private-test-value-") || strings.Count(console.String(), "[REDACTED]") != 2 {
		t.Fatal("unsafe console output")
	}
}
