package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRegisteredHelp(t *testing.T) {
	if problems := run(); len(problems) != 0 {
		t.Fatal(strings.Join(problems, "\n"))
	}
}

func TestExampleFlagsRespectResolvedCommandParsing(t *testing.T) {
	root := &cobra.Command{Use: "one"}
	hk := &cobra.Command{Use: "hk", DisableFlagParsing: true}
	configure := &cobra.Command{Use: "configure"}
	configure.Flags().Bool("dry-run", false, "")
	run := &cobra.Command{Use: "run [task]"}
	run.Flags().String("project", "", "")
	root.AddCommand(hk, configure, run)
	for _, tt := range []struct {
		name, example string
		owner         *cobra.Command
		wantProblems  int
	}{
		{"dev shorthand", "one dev --project web", run, 0},
		{"build shorthand", "one build --project web", run, 0},
		{"test shorthand", "one test --project web", run, 0},
		{"lint shorthand", "one lint --project web", run, 0},
		{"namespaced shorthand", "one docs:build --project docs", run, 0},
		{"unknown task flag", "one dev --typo", run, 1},
		{"task arguments", "one dev --project web -- --port 4300", run, 0},
		{"built-in precedence", "one configure --project web", run, 1},
		{"native flag", "one configure --dry-run", configure, 0},
		{"unknown native flag", "one configure --typo", configure, 1},
		{"forwarded flag", "one hk check --all", hk, 0},
		{"forwarded flag in another command's help", "one hk check --all", configure, 0},
		{"native typo in passthrough help", "one configure --typo", hk, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			problems := scanInvocations(root, tt.owner, "Example", tt.example)
			if len(problems) != tt.wantProblems {
				t.Fatalf("problems = %v, want %d", problems, tt.wantProblems)
			}
		})
	}
}
