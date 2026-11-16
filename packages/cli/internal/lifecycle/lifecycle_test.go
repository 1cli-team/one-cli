package lifecycle

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

type testPlugin struct {
	id              string
	events          *[]string
	startErr        error
	beforeErr       error
	stopErr         error
	observed        error
	cleanupCanceled bool
}

func (p *testPlugin) ID() string          { return p.id }
func (p *testPlugin) Register(Host) error { return nil }
func (p *testPlugin) record(event string) { *p.events = append(*p.events, p.id+":"+event) }
func (p *testPlugin) Start(context.Context, Invocation) (func(context.Context) error, error) {
	p.record("start")
	return func(ctx context.Context) error {
		p.record("stop")
		p.cleanupCanceled = ctx.Err() != nil
		return p.stopErr
	}, p.startErr
}
func (p *testPlugin) BeforeCommand(context.Context, Invocation) error {
	p.record("before")
	return p.beforeErr
}
func (p *testPlugin) AfterCommand(_ context.Context, _ Invocation, err error) error {
	p.record("after")
	p.observed = err
	return nil
}

func TestInvocationOrderAndFailureCleanup(t *testing.T) {
	failure := errors.New("command failed")
	for _, test := range []struct {
		name string
		fail string
		want []string
	}{
		{"success", "", []string{"a:start", "b:start", "c:start", "a:before", "b:before", "c:before", "command", "c:after", "b:after", "a:after", "c:stop", "b:stop", "a:stop"}},
		{"command failure", "command", []string{"a:start", "b:start", "c:start", "a:before", "b:before", "c:before", "command", "c:after", "b:after", "a:after", "c:stop", "b:stop", "a:stop"}},
		{"partial start", "start", []string{"a:start", "b:start", "b:stop", "a:stop"}},
		{"before failure", "before", []string{"a:start", "b:start", "c:start", "a:before", "b:before", "b:after", "a:after", "c:stop", "b:stop", "a:stop"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var events []string
			a, b, c := &testPlugin{id: "a", events: &events}, &testPlugin{id: "b", events: &events}, &testPlugin{id: "c", events: &events}
			if test.fail == "start" {
				b.startErr = failure
			}
			if test.fail == "before" {
				b.beforeErr = failure
			}
			l, err := New(a, b, c)
			if err != nil {
				t.Fatal(err)
			}
			err = l.Run(context.Background(), Invocation{}, func(context.Context) error {
				events = append(events, "command")
				if test.fail == "command" {
					return failure
				}
				return nil
			})
			if (test.fail != "") != errors.Is(err, failure) {
				t.Fatalf("error = %v", err)
			}
			if !reflect.DeepEqual(events, test.want) {
				t.Fatalf("events = %v, want %v", events, test.want)
			}
			if test.fail == "command" && !errors.Is(a.observed, failure) {
				t.Fatal("command error was not observed")
			}
		})
	}
}

func TestCanceledInvocationKeepsCleanupAndOriginalError(t *testing.T) {
	var events []string
	cleanupFailure := errors.New("cleanup failed")
	p := &testPlugin{id: "a", events: &events, stopErr: cleanupFailure}
	l, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	err = l.Run(ctx, Invocation{}, func(ctx context.Context) error { cancel(); return ctx.Err() })
	if !errors.Is(err, context.Canceled) || !errors.Is(err, cleanupFailure) {
		t.Fatalf("lost error chain: %v", err)
	}
	if p.cleanupCanceled {
		t.Fatal("cleanup inherited canceled context")
	}
}

func TestRegistrationRejectsDuplicates(t *testing.T) {
	var events []string
	p := &testPlugin{id: "same", events: &events}
	if _, err := New(p, p); err == nil {
		t.Fatal("duplicate plugin accepted")
	}
	l, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	root := &cobra.Command{Use: "one"}
	root.AddCommand(&cobra.Command{Use: "env"}, &cobra.Command{Use: "other", Aliases: []string{"env"}})
	if err := l.Register(Host{Root: root}); err == nil {
		t.Fatal("duplicate command alias accepted")
	}
}
