package skillscmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	platformprocess "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
)

func TestPassthroughPreservesArgumentsStreamsAndExit(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"add", "owner/repo", "--skill", "a b", "", "$(literal)", "--yes", "--global", "--agent", "codex"}, {"future-command", "--output", "json"}} {
		var out, stderr bytes.Buffer
		cwd, _ := os.Getwd()
		wantExit := &platformprocess.ExitStatus{Code: 37}
		called := false
		root := &cobra.Command{Use: "one", SilenceErrors: true, SilenceUsage: true}
		root.PersistentFlags().String("output", "", "")
		root.AddCommand(Commands(func(_ context.Context, dir string, got []string, in io.Reader, stdout, errout io.Writer) error {
			called = true
			if dir != cwd || !reflect.DeepEqual(append([]string{}, got...), append([]string{}, args...)) {
				t.Fatalf("forwarded %s %q; want %s %q", dir, got, cwd, args)
			}
			_, _ = io.Copy(stdout, in)
			_, _ = io.WriteString(errout, "raw tool error\n")
			return wantExit
		})...)
		root.SetIn(bytes.NewBufferString("interactive input\n"))
		root.SetOut(&out)
		root.SetErr(&stderr)
		root.SetArgs(append([]string{"skills"}, args...))
		err := root.Execute()
		if !called || !errors.Is(err, wantExit) || out.String() != "interactive input\n" || stderr.String() != "raw tool error\n" {
			t.Fatalf("passthrough: called=%v error=%v stdout=%q stderr=%q", called, err, out.String(), stderr.String())
		}
	}
}

func TestHelpRefreshesBothLanguages(t *testing.T) {
	defer i18n.Init(i18n.DefaultLocale)
	cmd := Commands(nil)[0]
	for _, locale := range []string{"zh-CN", "en-US", "zh-CN"} {
		_ = i18n.Init(locale)
		i18n.RefreshTree(cmd)
		if cmd.Short != i18n.T("skills.short") || cmd.Long != i18n.T("skills.tip") {
			t.Fatalf("stale help after switching to %s", locale)
		}
	}
}
