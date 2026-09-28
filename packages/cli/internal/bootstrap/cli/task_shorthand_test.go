package cli

import (
	"reflect"
	"testing"
)

func TestTaskShorthandPreservesCommandBoundaries(t *testing.T) {
	for _, tc := range []struct{ args, want []string }{
		{[]string{"test", "-p", "web", "--", "-o", "result.json"}, []string{"run", "test", "-p", "web", "--", "-o", "result.json"}},
		{[]string{"-o", "json", "docs:build", "--dry-run"}, []string{"-o", "json", "run", "docs:build", "--dry-run"}},
		{[]string{"-ojson", "verify", "--help"}, []string{"-ojson", "run", "verify", "--help"}},
		{[]string{"env", "typo"}, []string{"env", "typo"}},
		{[]string{"help", "env"}, []string{"help", "env"}},
		{[]string{"--", "test"}, []string{"--", "test"}},
		{[]string{"__unknown"}, []string{"__unknown"}},
		{[]string{"--help"}, []string{"--help"}},
		{[]string{"--unknown", "test"}, []string{"--unknown", "test"}},
	} {
		before := append([]string{}, tc.args...)
		if got := expandTaskShorthand(tc.args, isKnownSubcommand); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%v => %v, want %v", tc.args, got, tc.want)
		}
		if !reflect.DeepEqual(tc.args, before) {
			t.Fatal("changed input arguments")
		}
	}
	for _, cmd := range rootCmd.Commands() {
		for _, name := range append([]string{cmd.Name()}, cmd.Aliases...) {
			args := []string{name, "test"}
			if got := expandTaskShorthand(args, isKnownSubcommand); !reflect.DeepEqual(got, args) {
				t.Fatalf("rewrote built-in %s", name)
			}
		}
	}
	if got := scanOutputValue([]string{"test", "--", "-o", "json"}); got != "" {
		t.Fatal("child output flag was consumed", got)
	}
}
