package cliexts

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestMountWithEmptyRegistry(t *testing.T) {
	t.Cleanup(Reset)
	Reset()

	// Mount on a registry with zero contributors must be a no-op:
	// no commands attached, no error returned. Guards against future
	// refactors accidentally turning the empty case into a panic
	// (e.g. on a nil-slice iteration or a duplicate-name false
	// positive when there are no entries).
	root := &cobra.Command{Use: "one"}
	if err := Mount(root); err != nil {
		t.Fatalf("Mount on empty registry: %v", err)
	}
	if got := len(root.Commands()); got != 0 {
		t.Errorf("empty registry should leave root untouched, got %d children", got)
	}
}

func TestMountAddsContributedCommands(t *testing.T) {
	t.Cleanup(Reset)
	Reset()

	Register("a", func() []*cobra.Command {
		return []*cobra.Command{{Use: "alpha"}}
	})
	Register("b", func() []*cobra.Command {
		return []*cobra.Command{{Use: "beta"}}
	})

	root := &cobra.Command{Use: "one"}
	if err := Mount(root); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	got := map[string]bool{}
	for _, c := range root.Commands() {
		got[c.Name()] = true
	}
	if !got["alpha"] || !got["beta"] {
		t.Fatalf("expected alpha+beta, got %v", got)
	}
}

func TestMountRejectsDuplicateName(t *testing.T) {
	t.Cleanup(Reset)
	Reset()

	Register("first", func() []*cobra.Command {
		return []*cobra.Command{{Use: "shared"}}
	})
	Register("second", func() []*cobra.Command {
		return []*cobra.Command{{Use: "shared"}}
	})

	err := Mount(&cobra.Command{Use: "one"})
	if err == nil {
		t.Fatal("expected duplicate-name error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "shared") || !strings.Contains(msg, "first") || !strings.Contains(msg, "second") {
		t.Fatalf("error should name conflict + both sources, got: %s", msg)
	}
}

func TestMountIsDeterministic(t *testing.T) {
	t.Cleanup(Reset)

	for trial := 0; trial < 3; trial++ {
		Reset()
		Register("zulu", func() []*cobra.Command { return []*cobra.Command{{Use: "z"}} })
		Register("alpha", func() []*cobra.Command { return []*cobra.Command{{Use: "a"}} })
		Register("mike", func() []*cobra.Command { return []*cobra.Command{{Use: "m"}} })

		root := &cobra.Command{Use: "one"}
		if err := Mount(root); err != nil {
			t.Fatalf("Mount: %v", err)
		}
		var names []string
		for _, c := range root.Commands() {
			names = append(names, c.Name())
		}
		want := []string{"a", "m", "z"}
		for i, n := range want {
			if names[i] != n {
				t.Fatalf("trial %d: order = %v, want sorted by source = %v", trial, names, want)
			}
		}
	}
}
