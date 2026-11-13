package skills

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestReleaseInstallationUsesBundledSkills(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	previous := bundledSourceRef
	bundledSourceRef = strings.Repeat("a", 40)
	t.Cleanup(func() { bundledSourceRef = previous })
	root := t.TempDir()
	installer := Installer{Hash: testHashes, Run: func(_ context.Context, _ string, _ []string, _ io.Reader, _, _ io.Writer) error {
		t.Fatal("official initialization must not download from upstream")
		return nil
	}}
	if warnings := installer.Install(context.Background(), root, []Selection{{Names: []string{"one-cli", "one-go"}}}); len(warnings) != 0 {
		t.Fatalf("install: %v", warnings)
	}
	lock, err := readLock(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one-cli", "one-go"} {
		if matches, err := matchesBundledSkill(root, name); err != nil || !matches {
			t.Fatalf("bundle mismatch: %s %v", name, err)
		}
		if lock[name].SourceType != "github" || lock[name].Ref != "master" || lock[name].SkillPath != officialSkillPath(name) {
			t.Fatalf("unmanaged skill: %+v", lock[name])
		}
	}
}

func TestExistingSkillsArePreserved(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	previous := bundledSourceRef
	t.Cleanup(func() { bundledSourceRef = previous })
	for _, tt := range []struct {
		name, buildRef, entry string
	}{
		{"release commit", "release", `{"source":"1cli-team/one-cli","sourceType":"github","ref":"old-commit"}`},
		{"legacy local", "release", `{"source":"./.one/skill-sources/old","sourceType":"local"}`},
		{"master branch", "release", `{"source":"1cli-team/one-cli","sourceType":"github","ref":"master"}`},
		{"default branch", "release", `{"source":"1cli-team/one-cli","sourceType":"github"}`},
		{"development source", "", `{"source":"./packages/agent-skills","sourceType":"local"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bundledSourceRef = tt.buildRef
			root := t.TempDir()
			path := filepath.Join(root, ".agents", "skills", "one-cli")
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
			original := []byte("custom team instructions")
			if err := os.WriteFile(filepath.Join(path, "SKILL.md"), original, 0o644); err != nil {
				t.Fatal(err)
			}
			lock := []byte(`{"version":1,"skills":{"one-cli":` + tt.entry + `}}`)
			if err := os.WriteFile(filepath.Join(root, "skills-lock.json"), lock, 0o644); err != nil {
				t.Fatal(err)
			}
			installer := Installer{Run: func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
				t.Fatal("existing skill must not be reinstalled automatically")
				return nil
			}}
			warnings := installer.Install(context.Background(), root, []Selection{{Names: []string{"one-cli"}}})
			if len(warnings) != 0 {
				t.Fatalf("unexpected warning: %v", warnings)
			}
			for file, want := range map[string][]byte{filepath.Join(path, "SKILL.md"): original, filepath.Join(root, "skills-lock.json"): lock} {
				got, err := os.ReadFile(file)
				if err != nil || string(got) != string(want) {
					t.Fatalf("existing file changed: %s (%v)", file, err)
				}
			}
		})
	}
}

func TestThirdPartyInstallationStillUsesUpstream(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	root := t.TempDir()
	var called []string
	installer := Installer{Run: func(_ context.Context, dir string, args []string, _ io.Reader, _, _ io.Writer) error {
		called = slices.Clone(args)
		path := filepath.Join(dir, ".agents", "skills", "find-skills")
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("upstream fixture"), 0o644)
	}}
	if warnings := installer.Install(context.Background(), root, []Selection{{Source: findSource, Names: []string{"find-skills"}}}); len(warnings) != 0 {
		t.Fatal(warnings)
	}
	want := []string{"add", findSource, "--skill", "find-skills", "--agent", "universal", "--copy", "--yes"}
	if !slices.Equal(called, want) {
		t.Fatalf("upstream arguments = %q, want %q", called, want)
	}
}
