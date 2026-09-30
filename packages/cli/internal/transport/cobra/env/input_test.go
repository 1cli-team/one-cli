package envcmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestReadSetValuePreservesMultilineInput(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, want string }{
		{"LF", "-----BEGIN PRIVATE KEY-----\nfixture\n-----END PRIVATE KEY-----\n", "-----BEGIN PRIVATE KEY-----\nfixture\n-----END PRIVATE KEY-----"},
		{"CRLF", "header\r\nbody\r\n", "header\r\nbody"},
		{"two trailing lines", "body\n\n", "body\n"},
		{"literal escape", "header\\nbody", "header\\nbody"},
		{"empty", "", ""},
		{"limit", strings.Repeat("a", 1<<20), strings.Repeat("a", 1<<20)},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cmd := &cobra.Command{}
			cmd.Flags().Bool("stdin", true, "")
			cmd.SetIn(strings.NewReader(test.input))
			got, provided, err := readSetValue(cmd, []string{"PRIVATE_KEY"})
			if err != nil || !provided || got != test.want {
				t.Fatalf("stdin value was not preserved (provided=%v, error=%v)", provided, err)
			}
		})
	}
}

func TestSetRejectsStdinConflictAndOversizeBeforeRemoteAccess(t *testing.T) {
	for _, global := range []bool{false, true} {
		for _, test := range []struct {
			args  []string
			input string
		}{
			{[]string{"PRIVATE_KEY", "private-argument-marker", "--stdin"}, ""},
			{[]string{"PRIVATE_KEY=private-argument-marker", "--stdin"}, ""},
			{[]string{"PRIVATE_KEY", "--stdin"}, strings.Repeat("private-input-marker", 60_000)},
		} {
			// A nil service proves these failures cannot reach project planning or writes.
			cmd := Commands(Dependencies{})[0]
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			args := append([]string{"set"}, test.args...)
			if global {
				args = append(args, "--global")
			}
			cmd.SetArgs(args)
			cmd.SetIn(strings.NewReader(test.input))
			err := cmd.Execute()
			if err == nil || strings.Contains(err.Error(), "private-argument-marker") || strings.Contains(err.Error(), "private-input-marker") {
				t.Fatalf("unsafe stdin rejection (global=%v): %v", global, err)
			}
		}
	}
}

type brokenInput struct{ err error }

func (input brokenInput) Read([]byte) (int, error) { return 0, input.err }

func TestReadSetValuePreservesReadErrorAndExplicitValueBehavior(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("stdin", true, "")
	want := errors.New("stdin transport failed")
	cmd.SetIn(brokenInput{want})
	if _, _, err := readSetValue(cmd, []string{"TOKEN"}); !errors.Is(err, want) {
		t.Fatalf("lost stdin error: %v", err)
	}
	if err := cmd.Flags().Set("stdin", "false"); err != nil {
		t.Fatal(err)
	}
	value, provided, err := readSetValue(cmd, []string{"TOKEN="})
	if err != nil || !provided || value != "" {
		t.Fatal("explicit empty values changed")
	}
	_, provided, err = readSetValue(cmd, []string{"TOKEN"})
	if err != nil || provided {
		t.Fatal("hidden-prompt selection changed")
	}
}
