package skillscmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	skillsmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/skills"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
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
	cmd := ConfigureCommand(skillsmodule.Installer{})
	root := &cobra.Command{Use: "one"}
	root.AddCommand(cmd)
	for _, test := range []struct{ locale, short, flag, recommend string }{
		{"en-US", "Initialize official One skills", "Preview skill initialization", "Select and install missing skills"},
		{"zh-CN", "初始化 One 官方技能", "预览技能初始化", "根据工作区子项目"},
		{"en-US", "Initialize official One skills", "Preview skill initialization", "Select and install missing skills"},
	} {
		if err := i18n.Init(test.locale); err != nil {
			t.Fatal(err)
		}
		i18n.RefreshTree(root)
		if !strings.Contains(cmd.Short, test.short) || !strings.Contains(cmd.Flags().Lookup("dry-run").Usage, test.flag) {
			t.Fatalf("help did not refresh to %s: %s", test.locale, cmd.Short)
		}
		if !strings.Contains(cmd.Flags().Lookup("install-recommend").Usage, test.recommend) {
			t.Fatalf("recommendation help did not refresh to %s", test.locale)
		}
	}
}

func TestRecommendationOptionsKeepInstalledSkillsReadOnly(t *testing.T) {
	options, selected, description := recommendationOptions(&skillsmodule.RecommendationResult{Skills: []skillsmodule.RecommendedSkill{
		{Name: "one-cli", Status: "installed", Recommended: true},
		{Name: "one-go", Status: "missing", Recommended: true, Selected: true},
		{Name: "one-web", Status: "missing"},
	}})
	if len(options) != 2 || !slices.Equal(selected, []string{"one-go"}) || !strings.Contains(description, "one-cli") {
		t.Fatalf("options = %+v, selected = %q, description = %s", options, selected, description)
	}
	for _, option := range options {
		if option.Label != option.Value || option.Description != "" {
			t.Fatalf("selection is not names-only: %+v", option)
		}
	}
}

func commandWorkspace(t *testing.T) string {
	t.Helper()
	t.Setenv("ONE_HOME", t.TempDir())
	output.SetMode(output.ModeJSON)
	t.Cleanup(func() { output.SetMode(output.ModeAuto) })
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "one.manifest.toml"), []byte("version = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}
