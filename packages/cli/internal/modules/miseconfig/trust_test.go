package miseconfig

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	runtimeport "github.com/torchstellar-team/one-cli/packages/cli/internal/ports/runtime"
)

type trustProbe struct {
	commands []runtimeport.Command
	prepare  func(runtimeport.Command) (runtimeport.Command, error)
}

func (p *trustProbe) Prepare(context.Context, runtimeport.Command) (runtimeport.Command, error) {
	panic("trust must not load the project environment")
}
func (p *trustProbe) PrepareCLI(_ context.Context, command runtimeport.Command) (runtimeport.Command, error) {
	p.commands = append(p.commands, command)
	return p.prepare(command)
}

func TestTrustOnlyCompleteGeneratedConfigurations(t *testing.T) {
	root := fixture(t) // The root has a user-defined [env] block.
	plan, err := Build(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	unavailable := errors.New("runtime unavailable")
	probe := &trustProbe{prepare: func(c runtimeport.Command) (runtimeport.Command, error) { return c, unavailable }}
	if err := plan.TrustGenerated(context.Background(), probe); err != nil || len(probe.commands) != 0 {
		t.Fatalf("custom root must not be auto-trusted: %v %v", err, probe.commands)
	}
	writeFixture(t, filepath.Join(root, Filename), "")
	plan, err = Build(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := plan.TrustGenerated(context.Background(), probe); !errors.Is(err, unavailable) {
		t.Fatalf("lost preparation error: %v", err)
	}
	var paths []string
	for _, command := range probe.commands {
		if command.Directory != plan.Root || !reflect.DeepEqual(command.Argv[:3], []string{"trust", "--yes", "--quiet"}) {
			t.Fatal(command)
		}
		paths = append(paths, command.Argv[3])
	}
	if !reflect.DeepEqual(paths, []string{filepath.Join(plan.Root, Filename)}) {
		t.Fatalf("trusted unexpected configs: %v", paths)
	}
}

func TestTrustRejectsFileChangedWhilePreparingRuntime(t *testing.T) {
	root := fixture(t)
	writeFixture(t, filepath.Join(root, Filename), "")
	plan, err := Build(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	generated := plan.generated[0]
	path := filepath.Join(plan.Root, generated.Path)
	probe := &trustProbe{prepare: func(c runtimeport.Command) (runtimeport.Command, error) {
		writeFixture(t, path, generated.After+"\n[env]\nUSER_CHANGE = 'yes'\n")
		c.Argv = []string{"must-not-execute"}
		return c, nil
	}}
	err = plan.trustFile(context.Background(), probe, generated)
	if err == nil || strings.Contains(err.Error(), "must-not-execute") {
		t.Fatalf("changed file reached execution: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "USER_CHANGE") {
		t.Fatal("user change was overwritten")
	}
}
