package skillscmd

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/execution"
	skillsmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/skills"
	platformprocess "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/process"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/prompt"
)

func testInspection(context.Context, string) (*skillsmodule.Inspection, error) {
	dir := "/test-global"
	return &skillsmodule.Inspection{Agents: []skillsmodule.Agent{
		{ID: "codex", Name: "Codex", Detected: true, GlobalSkillsDir: &dir},
		{ID: "claude-code", Name: "Claude Code", GlobalSkillsDir: &dir},
		{ID: "universal", Name: "Universal", GlobalSkillsDir: &dir},
	}}, nil
}

func success(group skillsmodule.InstallGroup) *skillsmodule.GroupResult {
	result := &skillsmodule.GroupResult{}
	for _, name := range group.Names {
		for _, agent := range group.Agents {
			result.Targets = append(result.Targets, skillsmodule.TargetResult{Skill: name, Agent: agent, Status: "installed"})
		}
	}
	return result
}

func testPrompts(selected []string) recommendationPrompts {
	return recommendationPrompts{
		Skills: func(context.Context, string, string, []prompt.Option[string], []string) ([]string, error) {
			return selected, nil
		},
		Settings: func(_ context.Context, _ *skillsmodule.Inspection, options skillsmodule.InstallOptions) (skillsmodule.InstallOptions, error) {
			return options, nil
		},
		Review: func(context.Context, []skillsmodule.InstallGroup, skillsmodule.InstallOptions, *skillsmodule.Inspection) (string, error) {
			return "install", nil
		},
		Retry: func(context.Context) (bool, error) { return false, nil },
	}
}

func workflowCommand(t *testing.T, root string, installer skillsmodule.Installer, canPrompt bool, ui recommendationPrompts, args ...string) *cobra.Command {
	t.Helper()
	cmd := configureCommand(installer, func() bool { return canPrompt }, ui)
	cmd.SetContext(execution.WithScope(context.Background(), execution.NewScope(context.Background(), root)))
	cmd.SetErr(io.Discard)
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	cmd.SetArgs(args)
	return cmd
}

func TestRecommendedInstallationUsesOneReviewAndCommonSettings(t *testing.T) {
	root := commandWorkspace(t)
	var steps []string
	ui := testPrompts([]string{"one-go", "one-web", "find-skills"})
	ui.Skills = func(_ context.Context, _, _ string, _ []prompt.Option[string], defaults []string) ([]string, error) {
		steps = append(steps, "skills")
		if !slices.Contains(defaults, "one-cli") || !slices.Contains(defaults, "find-skills") {
			t.Fatal("recommendations not selected")
		}
		return []string{"one-go", "one-web", "find-skills"}, nil
	}
	want := skillsmodule.InstallOptions{Scope: "project", Mode: "copy", Agents: []string{"codex", "claude-code"}}
	ui.Settings = func(context.Context, *skillsmodule.Inspection, skillsmodule.InstallOptions) (skillsmodule.InstallOptions, error) {
		steps = append(steps, "settings")
		return want, nil
	}
	ui.Review = func(_ context.Context, groups []skillsmodule.InstallGroup, options skillsmodule.InstallOptions, _ *skillsmodule.Inspection) (string, error) {
		steps = append(steps, "review")
		if len(groups) != 2 || !reflect.DeepEqual(options, want) {
			t.Fatalf("wrong review: %+v %+v", groups, options)
		}
		return "install", nil
	}
	installer := skillsmodule.Installer{Inspect: testInspection, InstallGroup: func(_ context.Context, dir string, group skillsmodule.InstallGroup, options skillsmodule.InstallOptions, skip bool) (*skillsmodule.GroupResult, error) {
		if dir != root || !reflect.DeepEqual(options, want) || !slices.Equal(group.Agents, want.Agents) || !skip {
			t.Fatal("common settings lost")
		}
		steps = append(steps, "install")
		return success(group), nil
	}}
	if err := workflowCommand(t, root, installer, true, ui, "--install-recommend").Execute(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(steps, []string{"skills", "settings", "review", "install", "install"}) {
		t.Fatalf("flow = %v", steps)
	}
}

func TestRetryDoesNotRepeatSettingsOrSuccessfulSources(t *testing.T) {
	root := commandWorkspace(t)
	ui := testPrompts([]string{"one-go", "find-skills"})
	settings, review, retry := 0, 0, 0
	ui.Settings = func(_ context.Context, _ *skillsmodule.Inspection, options skillsmodule.InstallOptions) (skillsmodule.InstallOptions, error) {
		settings++
		return options, nil
	}
	ui.Review = func(context.Context, []skillsmodule.InstallGroup, skillsmodule.InstallOptions, *skillsmodule.Inspection) (string, error) {
		review++
		return "install", nil
	}
	ui.Retry = func(context.Context) (bool, error) { retry++; return true, nil }
	var installed []string
	installer := skillsmodule.Installer{Inspect: testInspection, InstallGroup: func(_ context.Context, _ string, group skillsmodule.InstallGroup, _ skillsmodule.InstallOptions, skip bool) (*skillsmodule.GroupResult, error) {
		installed = append(installed, group.Names...)
		if slices.Contains(group.Names, "one-go") && skip {
			return nil, errors.New("temporary download failure")
		}
		return success(group), nil
	}}
	if err := workflowCommand(t, root, installer, true, ui, "--install-recommend").Execute(); err != nil {
		t.Fatal(err)
	}
	if settings != 1 || review != 1 || retry != 1 || len(installed) != 3 || installed[len(installed)-1] != "one-go" {
		t.Fatalf("repeated successful work: %v / %d %d %d", installed, settings, review, retry)
	}
}

func TestReviewBackAndCancelDoNotInstallEarly(t *testing.T) {
	for _, final := range []string{"install", "cancel"} {
		t.Run(final, func(t *testing.T) {
			root := commandWorkspace(t)
			ui := testPrompts([]string{"one-go"})
			reviews, calls := 0, 0
			ui.Review = func(context.Context, []skillsmodule.InstallGroup, skillsmodule.InstallOptions, *skillsmodule.Inspection) (string, error) {
				reviews++
				if calls != 0 {
					t.Fatal("installed before final confirmation")
				}
				if reviews == 1 {
					return "back", nil
				}
				return final, nil
			}
			installer := skillsmodule.Installer{Inspect: testInspection, InstallGroup: func(_ context.Context, _ string, group skillsmodule.InstallGroup, _ skillsmodule.InstallOptions, _ bool) (*skillsmodule.GroupResult, error) {
				calls++
				return success(group), nil
			}}
			if err := workflowCommand(t, root, installer, true, ui, "--install-recommend").Execute(); err != nil {
				t.Fatal(err)
			}
			want := 0
			if final == "install" {
				want = 1
			}
			if reviews != 2 || calls != want {
				t.Fatalf("reviews=%d calls=%d", reviews, calls)
			}
		})
	}
}

func TestScopeChangeChecksGlobalInventoryAndReselects(t *testing.T) {
	root := commandWorkspace(t)
	if err := os.MkdirAll(filepath.Join(root, ".agents/skills/one-go"), 0o755); err != nil {
		t.Fatal(err)
	}
	ui := testPrompts(nil)
	picks := 0
	ui.Skills = func(_ context.Context, _, _ string, options []prompt.Option[string], defaults []string) ([]string, error) {
		picks++
		if picks == 1 {
			return []string{"one-web", "find-skills"}, nil
		}
		if !slices.Equal(defaults, []string{"find-skills"}) {
			t.Fatalf("scope change discarded the user's choices: %v", defaults)
		}
		if !slices.ContainsFunc(options, func(option prompt.Option[string]) bool { return option.Value == "one-go" }) {
			t.Fatal("project installation hid global candidate")
		}
		if slices.ContainsFunc(options, func(option prompt.Option[string]) bool { return option.Value == "one-web" }) {
			t.Fatal("globally installed skill remained selectable")
		}
		return []string{"one-go"}, nil
	}
	ui.Settings = func(_ context.Context, _ *skillsmodule.Inspection, options skillsmodule.InstallOptions) (skillsmodule.InstallOptions, error) {
		options.Scope = "global"
		return options, nil
	}
	installer := skillsmodule.Installer{Inspect: func(ctx context.Context, dir string) (*skillsmodule.Inspection, error) {
		result, err := testInspection(ctx, dir)
		result.Installed = []skillsmodule.InstalledSkill{{Name: "one-web", Scope: "global"}}
		return result, err
	}, InstallGroup: func(_ context.Context, _ string, group skillsmodule.InstallGroup, options skillsmodule.InstallOptions, _ bool) (*skillsmodule.GroupResult, error) {
		if options.Scope != "global" || !slices.Equal(group.Names, []string{"one-go"}) {
			t.Fatal("wrong global selection")
		}
		return success(group), nil
	}}
	if err := workflowCommand(t, root, installer, true, ui, "--install-recommend").Execute(); err != nil {
		t.Fatal(err)
	}
	if picks != 2 {
		t.Fatalf("selection count = %d", picks)
	}
}

func TestNoninteractiveOptionsAndFailures(t *testing.T) {
	root := commandWorkspace(t)
	calls := 0
	installer := skillsmodule.Installer{Inspect: testInspection, InstallGroup: func(_ context.Context, _ string, group skillsmodule.InstallGroup, options skillsmodule.InstallOptions, _ bool) (*skillsmodule.GroupResult, error) {
		calls++
		if options.Scope != "global" || options.Mode != "copy" || !slices.Equal(options.Agents, []string{"claude-code"}) {
			t.Fatalf("flags lost: %+v", options)
		}
		return nil, errors.New("source unavailable")
	}}
	err := workflowCommand(t, root, installer, false, recommendationPrompts{}, "--install-recommend", "--yes", "--agent", "claude-code", "--global", "--copy").Execute()
	var exit *platformprocess.ExitStatus
	if !errors.As(err, &exit) || exit.Code != 1 || calls != 2 {
		t.Fatalf("failure not reported or batch stopped early: %d, %v", calls, err)
	}
}

func TestPreviewEmptySelectionAndFlagErrorsDoNotInstall(t *testing.T) {
	for _, args := range [][]string{
		{"--install-recommend", "--dry-run"}, {"--install-recommend", "--dry-run", "--global"},
		{"--install-recommend"}, {"--install-recommend", "--skill", "one-cli"},
		{"--yes"}, {"--agent", "codex"}, {"--global"}, {"--copy"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			root := commandWorkspace(t)
			err := workflowCommand(t, root, skillsmodule.Installer{}, false, recommendationPrompts{}, args...).Execute()
			if slices.Contains(args, "--dry-run") && err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(args, "--dry-run") && err == nil {
				t.Fatal("invalid flags accepted")
			}
			if _, err := os.Stat(filepath.Join(root, ".agents")); !os.IsNotExist(err) {
				t.Fatal("preview or invalid flags changed files")
			}
		})
	}
	root := commandWorkspace(t)
	if err := workflowCommand(t, root, skillsmodule.Installer{}, true, testPrompts(nil), "--install-recommend").Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestCancellationAtSettingsDoesNotInstall(t *testing.T) {
	root := commandWorkspace(t)
	cancelled := errors.New("cancelled")
	ui := testPrompts([]string{"one-go"})
	ui.Settings = func(context.Context, *skillsmodule.Inspection, skillsmodule.InstallOptions) (skillsmodule.InstallOptions, error) {
		return skillsmodule.InstallOptions{}, cancelled
	}
	err := workflowCommand(t, root, skillsmodule.Installer{Inspect: testInspection}, true, ui, "--install-recommend").Execute()
	if !errors.Is(err, cancelled) {
		t.Fatalf("cancellation replaced: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "skills-lock.json")); !os.IsNotExist(err) {
		t.Fatal("cancel wrote lock")
	}
}
