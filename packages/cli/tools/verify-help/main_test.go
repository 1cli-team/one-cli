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
	root.AddCommand(hk, configure)
	for _, tt := range []struct {
		name, example string
		owner         *cobra.Command
		wantProblems  int
	}{
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
