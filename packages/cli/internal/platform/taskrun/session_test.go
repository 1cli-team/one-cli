package taskrun

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	platformprocess "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
)

func TestSchedulerDependenciesConcurrencyAndDeterministicFailure(t *testing.T) {
	tasks := []Task{{Name: "lib"}, {Name: "web", Dependencies: []string{"lib"}}, {Name: "api"}, {Name: "worker", Dependencies: []string{"web"}}}
	var mu sync.Mutex
	finished := map[string]bool{}
	active, peak := 0, 0
	both := make(chan struct{})
	count := 0
	results, err := Run(context.Background(), tasks, Options{Concurrency: 2, Run: func(_ context.Context, t Task, _ io.Writer) error {
		mu.Lock()
		active++
		peak = max(peak, active)
		count++
		if count == 2 {
			close(both)
		}
		for _, dep := range t.Dependencies {
			if !finished[dep] {
				testingError := dep
				mu.Unlock()
				return errors.New("dependency ran too late: " + testingError)
			}
		}
		mu.Unlock()
		if t.Name == "lib" || t.Name == "api" {
			select {
			case <-both:
			case <-time.After(time.Second):
				return errors.New("tasks were not concurrent")
			}
		}
		mu.Lock()
		finished[t.Name] = true
		active--
		mu.Unlock()
		return nil
	}})
	if err != nil || peak != 2 || len(results) != 4 {
		t.Fatalf("peak=%d results=%+v err=%v", peak, results, err)
	}
	for _, r := range results {
		if r.Status != "succeeded" {
			t.Fatal(r)
		}
	}
}
func TestBuildFailureBlocksDependentsAndKeepsRunningIndependentTask(t *testing.T) {
	started := make(chan struct{})
	fail := make(chan struct{})
	results, err := Run(context.Background(), []Task{{Name: "lib"}, {Name: "api"}, {Name: "web", Dependencies: []string{"lib"}}}, Options{Concurrency: 2, Run: func(_ context.Context, t Task, _ io.Writer) error {
		switch t.Name {
		case "lib":
			<-started
			close(fail)
			return &platformprocess.ExitStatus{Code: 42}
		case "api":
			close(started)
			<-fail
			return nil
		default:
			return errors.New("dependent started")
		}
	}})
	var exit *platformprocess.ExitStatus
	if !errors.As(err, &exit) || exit.Code != 42 || results[0].Status != "failed" || results[1].Status != "succeeded" || results[2].Status != "blocked" {
		t.Fatalf("%+v %v", results, err)
	}
}
func TestDevelopmentKeepGoingAndFailFast(t *testing.T) {
	for _, keep := range []bool{false, true} {
		t.Run(map[bool]string{true: "keep", false: "stop"}[keep], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			sibling := make(chan struct{})
			failed := make(chan struct{})
			results, err := Run(ctx, []Task{{Name: "bad"}, {Name: "sibling"}}, Options{Development: true, KeepGoing: keep, Run: func(ctx context.Context, t Task, _ io.Writer) error {
				if t.Name == "bad" {
					<-sibling
					close(failed)
					return &platformprocess.ExitStatus{Code: 7}
				}
				close(sibling)
				<-failed
				if keep {
					return nil
				}
				<-ctx.Done()
				return ctx.Err()
			}})
			if err == nil || results[0].ExitCode != 7 {
				t.Fatalf("%+v %v", results, err)
			}
			want := "stopped"
			if keep {
				want = "succeeded"
			}
			if results[1].Status != want {
				t.Fatal(results)
			}
		})
	}
}
func TestCycleRejectedBeforeStarting(t *testing.T) {
	_, err := Run(context.Background(), []Task{{Name: "a", Dependencies: []string{"b"}}, {Name: "b", Dependencies: []string{"a"}}}, Options{Run: func(context.Context, Task, io.Writer) error { t.Fatal("started cycle"); return nil }})
	if err == nil {
		t.Fatal("accepted cycle")
	}
}

func TestLineWriterBoundsPartialOutputWithoutLoss(t *testing.T) {
	var out strings.Builder
	var mu sync.Mutex
	w := &lineWriter{out: &out, mu: &mu, prefix: "[web] "}
	_, _ = w.Write([]byte("first\npar"))
	_, _ = w.Write([]byte("tial\n" + strings.Repeat("x", 200000)))
	w.flush()
	if got := strings.ReplaceAll(out.String(), "[web] ", ""); got != "first\npartial\n"+strings.Repeat("x", 200000)+"\n" {
		t.Fatalf("output lost: %d bytes", len(got))
	}
}
