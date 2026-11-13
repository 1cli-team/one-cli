package skills

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
)

func TestBatchContinuesAndRetriesOnlyFailedTargets(t *testing.T) {
	options := InstallOptions{Scope: "project", Mode: "copy", Agents: []string{"codex", "claude-code"}}
	groups := []InstallGroup{
		{Source: "owner/first", Names: []string{"a", "b"}, Agents: options.Agents},
		{Source: "owner/second", Names: []string{"c"}, Agents: options.Agents},
	}
	var called []InstallGroup
	installer := Installer{InstallGroup: func(_ context.Context, _ string, group InstallGroup, got InstallOptions, skip bool) (*GroupResult, error) {
		called = append(called, group)
		if !reflect.DeepEqual(got, options) || !skip {
			t.Fatal("initial options were not preserved")
		}
		result := &GroupResult{}
		for _, name := range group.Names {
			for _, agent := range group.Agents {
				status := "installed"
				if name == "a" && agent == "claude-code" {
					status = "failed"
				}
				result.Targets = append(result.Targets, TargetResult{Skill: name, Agent: agent, Status: status, Ref: "pinned-commit"})
			}
		}
		return result, nil
	}}
	result, err := installer.InstallGroups(context.Background(), t.TempDir(), groups, options, true, nil)
	if err != nil || len(called) != 2 {
		t.Fatalf("batch stopped: %v, %v", called, err)
	}
	retry := result.RetryGroups()
	want := []InstallGroup{{Source: "owner/first", Names: []string{"a"}, Agents: []string{"claude-code"}, Ref: "pinned-commit"}}
	if !reflect.DeepEqual(retry, want) {
		t.Fatalf("retry = %#v, want %#v", retry, want)
	}
	installer.InstallGroup = func(_ context.Context, _ string, group InstallGroup, _ InstallOptions, skip bool) (*GroupResult, error) {
		if skip || !reflect.DeepEqual(group, want[0]) {
			t.Fatalf("retried successful targets: %#v", group)
		}
		return &GroupResult{Targets: []TargetResult{{Skill: "a", Agent: "claude-code", Status: "installed"}}}, nil
	}
	next, err := installer.InstallGroups(context.Background(), result.Root, retry, options, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	result.Merge(next)
	if len(result.Targets) != 6 || len(result.RetryGroups()) != 0 {
		t.Fatalf("bad merged result: %+v", result)
	}
}

func TestBatchSourceFailureAndMissingResultsAreRetryable(t *testing.T) {
	groups := []InstallGroup{{Source: "bad/repo", Names: []string{"a"}, Agents: []string{"codex"}}, {Source: "ok/repo", Names: []string{"b"}, Agents: []string{"codex"}}}
	installer := Installer{InstallGroup: func(_ context.Context, _ string, group InstallGroup, _ InstallOptions, _ bool) (*GroupResult, error) {
		if group.Source == "bad/repo" {
			return nil, errors.New("download unavailable")
		}
		return &GroupResult{}, nil
	}}
	result, err := installer.InstallGroups(context.Background(), t.TempDir(), groups, InstallOptions{}, true, nil)
	if err != nil || len(result.Targets) != 2 || len(result.RetryGroups()) != 2 {
		t.Fatalf("failed = %+v, %v", result, err)
	}
	if result.Targets[0].Message != "download unavailable" || result.Targets[1].Message == "" {
		t.Fatal("lost failure detail")
	}
}

func TestBatchCancellationStopsFollowingSources(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	installer := Installer{InstallGroup: func(context.Context, string, InstallGroup, InstallOptions, bool) (*GroupResult, error) {
		calls++
		cancel()
		return nil, context.Canceled
	}}
	_, err := installer.InstallGroups(ctx, t.TempDir(), []InstallGroup{{Source: "first"}, {Source: "second"}}, InstallOptions{}, true, nil)
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("cancellation = %d %v", calls, err)
	}
}

func TestInstallOptionsValidateTargetsAndScope(t *testing.T) {
	global := "/global"
	inspection := &Inspection{Agents: []Agent{{ID: "codex", GlobalSkillsDir: &global}, {ID: "project-only"}}}
	for _, options := range []InstallOptions{
		{Scope: "global", Mode: "copy", Agents: []string{"missing"}},
		{Scope: "global", Mode: "copy", Agents: []string{"project-only"}},
		{Scope: "project", Mode: "copy"},
		{Scope: "other", Mode: "copy", Agents: []string{"codex"}},
		{Scope: "project", Mode: "other", Agents: []string{"codex"}},
	} {
		if options.Validate(inspection) == nil {
			t.Fatalf("accepted invalid options: %+v", options)
		}
	}
	if err := (InstallOptions{Scope: "global", Mode: "copy", Agents: []string{"codex"}}).Validate(inspection); err != nil {
		t.Fatal(err)
	}
}

func TestGlobalRecommendationsUseGlobalInventory(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, root, "one.manifest.toml", "version = 2\n")
	putFiles(t, root, "one-cli", Files{"SKILL.md": []byte("local instructions")})
	scan, err := Recommend(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	global, err := scan.ForScope("global", &Inspection{Installed: []InstalledSkill{{Name: "one-go", Scope: "global"}}})
	if err != nil {
		t.Fatal(err)
	}
	if recommendation(t, global, "one-cli").Status != "missing" || recommendation(t, global, "one-go").Status != "installed" {
		t.Fatal("mixed project and global installations")
	}
	groups, err := global.InstallGroups([]string{"one-cli", "one-go"}, InstallOptions{Agents: []string{"codex"}})
	if err != nil || len(groups) != 1 || !slices.Equal(groups[0].Names, []string{"one-cli"}) {
		t.Fatalf("global plan = %+v, %v", groups, err)
	}
}
