package skillscmd

import (
	"context"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	skillsmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/skills"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func TestPassthroughPreservesUpstreamArguments(t *testing.T) {
	want := []string{"add", "owner/repo", "--skill", "example", "--global", "--json"}
	var got []string
	cmd := Commands(func(_ context.Context, _ string, args []string, _ io.Reader, _, _ io.Writer) error {
		got = slices.Clone(args)
		return nil
	})[0]
	cmd.SetArgs(want)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("forwarded %q, want %q", got, want)
	}
}

func TestConfigureHelpRefreshesBothLanguages(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Init("en-US") })
	cmd := ConfigureCommand(skillsmodule.Initializer{})
	root := &cobra.Command{Use: "one"}
	root.AddCommand(cmd)
	for _, test := range []struct{ locale, short, flag string }{
		{"en-US", "Initialize official One skills", "Preview skill initialization"},
		{"zh-CN", "初始化 One 官方技能", "预览技能初始化"},
		{"en-US", "Initialize official One skills", "Preview skill initialization"},
	} {
		if err := i18n.Init(test.locale); err != nil {
			t.Fatal(err)
		}
		i18n.RefreshTree(root)
		if !strings.Contains(cmd.Short, test.short) || !strings.Contains(cmd.Flags().Lookup("dry-run").Usage, test.flag) {
			t.Fatalf("help did not refresh to %s: %s", test.locale, cmd.Short)
		}
	}
}
