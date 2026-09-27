package build

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/dependencies"
	platformprocess "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
)

func TestExecuteWaitsForAllBuildsAndPreparesLibraries(t *testing.T) {
	w := fixture(t)
	plan, err := NewPlan(w, "", "")
	if err != nil {
		t.Fatal(err)
	}
	events := []string{}
	service := Service{
		Prepare: func(_ context.Context, in dependencies.Input) error {
			if !reflect.DeepEqual(in.Projects, []string{"library", "web", "api"}) {
				t.Fatal(in.Projects)
			}
			events = append(events, "prepare")
			return nil
		},
		Run: func(_ context.Context, root string, task Task, env string, log io.Writer) error {
			if root != w.Root() {
				t.Fatal(root)
			}
			events = append(events, task.Project)
			return nil
		},
	}
	result, err := service.Execute(context.Background(), w, plan, nil)
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("%+v %v", result, err)
	}
	if !reflect.DeepEqual(events, []string{"prepare", "library", "web", "api"}) {
		t.Fatal(events)
	}
	for i := 0; i < 3; i++ {
		if result.Tasks[i].Status != "succeeded" || plan.Tasks[i].Status != "pending" {
			t.Fatal(result, plan)
		}
	}
	if result.DryRun || result.Schema != "one-cli/build-result/v1" || result.Tasks[3].Status != "skipped" {
		t.Fatal(result)
	}
}

func TestExecuteStopsOnFailureAndPreservesExitCode(t *testing.T) {
	w := fixture(t)
	plan, _ := NewPlan(w, "", "")
	service := Service{Run: func(_ context.Context, _ string, task Task, _ string, _ io.Writer) error {
		if task.Project == "web" {
			return &platformprocess.ExitStatus{Code: 42}
		}
		if task.Project == "api" {
			t.Fatal("started after failure")
		}
		return nil
	}}
	result, err := service.Execute(context.Background(), w, plan, nil)
	var exit *platformprocess.ExitStatus
	if !errors.As(err, &exit) || exit.Code != 42 || result.ExitCode != 42 {
		t.Fatalf("%+v %v", result, err)
	}
	if result.Tasks[0].Status != "succeeded" || result.Tasks[1].Status != "failed" || result.Tasks[1].ExitCode != 42 || result.Tasks[2].Status != "not_run" {
		t.Fatal(result.Tasks)
	}
	if !strings.Contains(result.Error, "web") {
		t.Fatal(result.Error)
	}
}

func TestPreparationFailureAndCancellationPreventBuilds(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelled), func(t *testing.T) {
			w := fixture(t)
			plan, _ := NewPlan(w, "", "")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			service := Service{
				Prepare: func(context.Context, dependencies.Input) error {
					if cancelled {
						cancel()
						return ctx.Err()
					}
					return errors.New("dependency install failed")
				},
				Run: func(context.Context, string, Task, string, io.Writer) error { t.Fatal("build started"); return nil },
			}
			result, err := service.Execute(ctx, w, plan, nil)
			if err == nil || result.Tasks[0].Status != "not_run" {
				t.Fatalf("%+v %v", result, err)
			}
			if cancelled && result.ExitCode != 130 {
				t.Fatal(result)
			}
		})
	}
}

func TestPrefixWriterPreservesLargeAndPartialOutput(t *testing.T) {
	var out bytes.Buffer
	w := &prefixWriter{out: &out, prefix: "[web] "}
	_, _ = w.Write([]byte("first\npar"))
	_, _ = w.Write([]byte("tial\n" + strings.Repeat("x", 200000)))
	w.Flush()
	got := strings.ReplaceAll(out.String(), "[web] ", "")
	want := "first\npartial\n" + strings.Repeat("x", 200000) + "\n"
	if got != want {
		t.Fatalf("output lost: length %d, want %d", len(got), len(want))
	}
}
