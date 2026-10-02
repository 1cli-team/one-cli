package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

func writeSkill(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, ".agents", "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fakeInstall(t *testing.T, calls *[][]string) RunFunc {
	return func(_ context.Context, root string, args []string, _ io.Reader, _, _ io.Writer) error {
		*calls = append(*calls, append([]string(nil), args...))
		lock, err := readLock(root)
		if err != nil {
			t.Fatal(err)
		}
		for index := 3; index < len(args) && args[index] != "--agent"; index++ {
			name := args[index]
			if _, err := os.Stat(filepath.Join(root, ".agents", "skills", name, "SKILL.md")); os.IsNotExist(err) {
				writeSkill(t, root, name, "original")
			}
			entry := lockEntry{Source: sourceRepo(args[1]), SourceType: "github"}
			if strings.HasPrefix(args[1], "./") {
				entry.Source, entry.SourceType = args[1], "local"
			}
			lock[name] = entry
		}
		raw, _ := json.Marshal(map[string]any{"version": 1, "skills": lock})
		return os.WriteFile(filepath.Join(root, "skills-lock.json"), raw, 0o644)
	}
}

func TestInstallIsIdempotentAndPreservesUserChanges(t *testing.T) {
	root := t.TempDir()
	var calls [][]string
	installer := Installer{Run: fakeInstall(t, &calls)}
	plan := Defaults("electron-app", "local:electron-app")
	if warnings := installer.Install(context.Background(), root, plan); len(warnings) != 0 {
		t.Fatal(warnings)
	}
	firstCalls := len(calls)
	if firstCalls != len(plan) {
		t.Fatalf("calls=%d want %d", firstCalls, len(plan))
	}
	writeSkill(t, root, "one-cli", "team modifications")
	lockBefore, _ := os.ReadFile(filepath.Join(root, "skills-lock.json"))
	if warnings := installer.Install(context.Background(), root, plan); len(warnings) != 0 || len(calls) != firstCalls {
		t.Fatalf("repeat changed installations: %v %v", warnings, calls)
	}
	got, _ := os.ReadFile(filepath.Join(root, ".agents", "skills", "one-cli", "SKILL.md"))
	lockAfter, _ := os.ReadFile(filepath.Join(root, "skills-lock.json"))
	if string(got) != "team modifications" || string(lockAfter) != string(lockBefore) {
		t.Fatal("repeat overwrote user content or lock")
	}
	for _, call := range calls {
		if !reflect.DeepEqual(call[len(call)-4:], []string{"--agent", "universal", "--copy", "--yes"}) {
			t.Fatalf("unexpected flags %q", call)
		}
	}
}

func TestBundledRegistrationRetriesAfterFailure(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(map[bool]string{false: "unregistered", true: "partially registered"}[partial], func(t *testing.T) {
			root := t.TempDir()
			var calls, registrations [][]string
			register := fakeInstall(t, &registrations)
			installer := Installer{Run: func(ctx context.Context, directory string, args []string, in io.Reader, out, stderr io.Writer) error {
				calls = append(calls, append([]string(nil), args...))
				if len(calls) == 1 {
					if partial {
						first := append(append([]string(nil), args[:4]...), args[len(args)-4:]...)
						if err := register(ctx, directory, first, in, out, stderr); err != nil {
							t.Fatal(err)
						}
					}
					return errors.New("registration interrupted")
				}
				return register(ctx, directory, args, in, out, stderr)
			}}
			plan := []Selection{{Names: []string{"one-cli", "one-electron"}}}
			if warnings := installer.Install(context.Background(), root, plan); len(warnings) != 1 {
				t.Fatalf("failure was not reported: %v", warnings)
			}
			want := plan[0].Names
			if partial {
				writeSkill(t, root, "one-cli", "team modifications")
				want = []string{"one-electron"}
			}
			if warnings := installer.Install(context.Background(), root, plan); len(warnings) != 0 || len(calls) != 2 {
				t.Fatalf("registration did not retry: warnings=%v calls=%v", warnings, calls)
			}
			if got := calls[1][3 : len(calls[1])-4]; !reflect.DeepEqual(got, want) {
				t.Fatalf("retried skills=%v want %v", got, want)
			}
			lock, err := readLock(root)
			if err != nil || len(lock) != 2 {
				t.Fatalf("retry did not finish registration: %v %v", lock, err)
			}
			if partial {
				content, err := os.ReadFile(filepath.Join(root, ".agents", "skills", "one-cli", "SKILL.md"))
				if err != nil || string(content) != "team modifications" {
					t.Fatalf("retry changed an installed skill: %q %v", content, err)
				}
			}
			if warnings := installer.Install(context.Background(), root, plan); len(warnings) != 0 || len(calls) != 2 {
				t.Fatalf("successful retry was not idempotent: %v %v", warnings, calls)
			}
		})
	}
}

func TestUnregisteredTeamSkillsArePreserved(t *testing.T) {
	for _, file := range []string{"SKILL.md", "references/layouts.md"} {
		t.Run(file, func(t *testing.T) {
			root := t.TempDir()
			plan := []Selection{{Names: []string{"one-cli"}}}
			failed := Installer{Run: func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
				return errors.New("registration interrupted")
			}}
			if warnings := failed.Install(context.Background(), root, plan); len(warnings) != 1 {
				t.Fatal(warnings)
			}
			path := filepath.Join(root, ".agents", "skills", "one-cli", filepath.FromSlash(file))
			if err := os.WriteFile(path, []byte("team modifications"), 0o644); err != nil {
				t.Fatal(err)
			}
			var calls [][]string
			warnings := (Installer{Run: fakeInstall(t, &calls)}).Install(context.Background(), root, plan)
			content, err := os.ReadFile(path)
			if len(warnings) != 0 || len(calls) != 0 || err != nil || string(content) != "team modifications" {
				t.Fatalf("unregistered team skill changed: %v %v %q %v", warnings, calls, content, err)
			}
			if _, err := os.Stat(filepath.Join(root, "skills-lock.json")); !os.IsNotExist(err) {
				t.Fatalf("team skill was registered implicitly: %v", err)
			}
		})
	}
}

func TestSourceConflictAndInvalidLockArePreserved(t *testing.T) {
	for _, raw := range []string{`{"version":1,"skills":{"shadcn":{"source":"team/custom","sourceType":"github"}}}`, `{"version":99,"skills":{}}`, `invalid json`} {
		root := t.TempDir()
		lockPath := filepath.Join(root, "skills-lock.json")
		_ = os.WriteFile(lockPath, []byte(raw), 0o644)
		var calls [][]string
		warnings := (Installer{Run: fakeInstall(t, &calls)}).Install(context.Background(), root, []Selection{{Source: uiSource, Names: []string{"shadcn"}}})
		got, _ := os.ReadFile(lockPath)
		if len(warnings) != 1 || len(calls) != 0 || string(got) != raw {
			t.Fatalf("conflict mutated state: %v %q %q", warnings, calls, got)
		}
	}
}

func TestFailureAndTimeoutReturnRecoveryAndKeepWorkspace(t *testing.T) {
	for _, timedOut := range []bool{false, true} {
		root := t.TempDir()
		marker := filepath.Join(root, "keep.txt")
		_ = os.WriteFile(marker, []byte("project files"), 0o644)
		installer := Installer{Timeout: 30 * time.Millisecond, Run: func(ctx context.Context, _ string, _ []string, _ io.Reader, _, _ io.Writer) error {
			if timedOut {
				<-ctx.Done()
				return ctx.Err()
			}
			return errors.New("network unavailable")
		}}
		warnings := installer.Install(context.Background(), root, []Selection{{Source: goSource, Names: []string{"golang-patterns"}}})
		got, _ := os.ReadFile(marker)
		if len(warnings) != 1 || !strings.Contains(warnings[0], "one skills add "+goSource) || string(got) != "project files" {
			t.Fatalf("recovery=%v marker=%q", warnings, got)
		}
	}
}

func TestAutomaticInstallHidesToolUIAndRetainsFailureReason(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Init(i18n.DefaultLocale) })
	for _, locale := range []string{"en-US", "zh-CN"} {
		t.Run(locale, func(t *testing.T) {
			if err := i18n.Init(locale); err != nil {
				t.Fatal(err)
			}
			for _, fail := range []bool{false, true} {
				root := t.TempDir()
				var log bytes.Buffer
				var calls [][]string
				install := fakeInstall(t, &calls)
				installer := Installer{Log: &log, Run: func(ctx context.Context, directory string, args []string, in io.Reader, out, stderr io.Writer) error {
					if _, err := io.WriteString(out, strings.Repeat("SKILLS UPSTREAM BANNER\n", 300)); err != nil {
						return err
					}
					if _, err := io.WriteString(stderr, "\x1b[31mfatal: permission denied /tmp/skill-source\x1b[0m\n"); err != nil {
						return err
					}
					if fail {
						return errors.New("installation failed")
					}
					return install(ctx, directory, args, in, out, stderr)
				}}
				warnings := installer.Install(context.Background(), root, []Selection{{Names: []string{"one-cli"}}})
				if got, want := log.String(), i18n.Tf("skills.installing", "one-cli")+"\n"; got != want {
					t.Fatalf("automatic install leaked upstream UI: %q, want %q", got, want)
				}
				if !fail && len(warnings) != 0 {
					t.Fatal(warnings)
				}
				if fail && (len(warnings) != 1 || !strings.Contains(warnings[0], "fatal: permission denied /tmp/skill-source") || !strings.Contains(warnings[0], "one skills add ") || strings.Contains(warnings[0], "\x1b")) {
					t.Fatalf("failure lost its diagnosis or recovery command: %v", warnings)
				}
			}
		})
	}
}

func TestDevelopmentSkillsKeepOnlySelectedPortableCopies(t *testing.T) {
	for _, names := range [][]string{
		{"one-cli"},
		{"one-cli", "one-nestjs"},
		{"one-cli", "one-go"},
		{"one-cli", "one-expo"},
		{"one-cli", "one-web"},
		{"one-cli", "one-electron", "one-web"},
		{"one-cli", "one-web", "one-fumadocs"},
	} {
		t.Run(strings.Join(names, "+"), func(t *testing.T) {
			root := t.TempDir()
			var calls [][]string
			warnings := (Installer{Run: fakeInstall(t, &calls)}).Install(context.Background(), root, []Selection{{Names: names}})
			if len(warnings) != 0 || len(calls) != 1 || calls[0][1] != "./.agents/skills" {
				t.Fatalf("local registration failed: %v %v", warnings, calls)
			}
			entries, err := os.ReadDir(filepath.Join(root, ".agents", "skills"))
			if err != nil || len(entries) != len(names) {
				t.Fatalf("unselected skills were written: %v %v", entries, err)
			}
			lock, err := readLock(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range names {
				entry := lock[name]
				if entry.SourceType != "local" || entry.Source != "./.agents/skills" {
					t.Fatalf("nonportable source for %s: %+v", name, entry)
				}
				if raw, err := os.ReadFile(filepath.Join(root, entry.Source, name, "SKILL.md")); err != nil || !strings.Contains(string(raw), "name: "+name) {
					t.Fatalf("source is not the installed skill: %s %v", name, err)
				}
			}
			if _, err := os.Stat(filepath.Join(root, ".one")); !os.IsNotExist(err) {
				t.Fatalf("duplicate sources remain: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "one-cli", "references", "layouts.md")); err != nil {
				t.Fatal("selected skill lost its references", err)
			}
		})
	}
}

func TestLegacyBundledSourcesPreserveTeamFilesAndLock(t *testing.T) {
	for _, source := range []string{"./.one/skill-sources/0123456789abcdef", "./packages/agent-skills", "./.agents/skills"} {
		t.Run(source, func(t *testing.T) {
			root := t.TempDir()
			writeSkill(t, root, "one-cli", "team modifications")
			raw, err := json.Marshal(map[string]any{"version": 1, "skills": map[string]any{"one-cli": map[string]any{"source": source, "sourceType": "local", "computedHash": "team-hash"}}})
			if err != nil {
				t.Fatal(err)
			}
			lockPath := filepath.Join(root, "skills-lock.json")
			if err := os.WriteFile(lockPath, raw, 0o644); err != nil {
				t.Fatal(err)
			}
			var calls [][]string
			installer := Installer{Run: fakeInstall(t, &calls)}
			warnings := installer.Install(context.Background(), root, []Selection{{Names: []string{"one-cli"}}})
			after, err := os.ReadFile(lockPath)
			if len(warnings) != 0 || len(calls) != 0 || err != nil || !bytes.Equal(after, raw) {
				t.Fatalf("existing installation changed: %v %v %s %v", warnings, calls, after, err)
			}
			warnings = installer.Install(context.Background(), root, []Selection{{Names: []string{"one-cli", "one-electron"}}})
			content, err := os.ReadFile(filepath.Join(root, ".agents", "skills", "one-cli", "SKILL.md"))
			if len(warnings) != 0 || len(calls) != 1 || !reflect.DeepEqual(calls[0][3:4], []string{"one-electron"}) || err != nil || string(content) != "team modifications" {
				t.Fatalf("adding a skill overwrote team changes: %v %v %q %v", warnings, calls, content, err)
			}
		})
	}
}

func TestReleaseSkillsUsePublishedCommitAndPreserveOlderSources(t *testing.T) {
	previous := bundledSourceRef
	bundledSourceRef = strings.Repeat("a", 40)
	t.Cleanup(func() { bundledSourceRef = previous })
	root := t.TempDir()
	var calls [][]string
	installer := Installer{Run: fakeInstall(t, &calls)}
	plan := []Selection{{Names: []string{"one-cli", "one-electron"}}}
	warnings := installer.Install(context.Background(), root, plan)
	want := oneSource + "/packages/agent-skills#" + bundledSourceRef
	if len(warnings) != 0 || len(calls) != 1 || calls[0][1] != want {
		t.Fatalf("release did not use its published commit: %v %v", warnings, calls)
	}
	lock, err := readLock(root)
	if err != nil || lock["one-cli"].Source != oneSource || lock["one-cli"].SourceType != "github" {
		t.Fatalf("unexpected GitHub provenance: %v %v", lock, err)
	}
	writeSkill(t, root, "one-cli", "team changes")
	warnings = installer.Install(context.Background(), root, plan)
	if len(warnings) != 0 || len(calls) != 1 {
		t.Fatalf("release reinstalled existing files: %v %v", warnings, calls)
	}
	// A release also accepts a development/legacy local source without forcing
	// a provenance migration or overwriting the installed instructions.
	legacyRoot := t.TempDir()
	writeSkill(t, legacyRoot, "one-cli", "legacy team changes")
	raw := []byte(`{"version":1,"skills":{"one-cli":{"source":"./.one/skill-sources/old","sourceType":"local","computedHash":"team"}}}`)
	lockPath := filepath.Join(legacyRoot, "skills-lock.json")
	if err := os.WriteFile(lockPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	warnings = installer.Install(context.Background(), legacyRoot, []Selection{{Names: []string{"one-cli"}}})
	after, err := os.ReadFile(lockPath)
	if len(warnings) != 0 || len(calls) != 1 || err != nil || !bytes.Equal(raw, after) {
		t.Fatalf("release migrated the legacy lock implicitly: %v %v %v", warnings, calls, err)
	}
}

func TestDefaultsCoverRegistryWithoutBusinessSkills(t *testing.T) {
	registry, err := template.Fetch(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range registry.Templates {
		plan := Defaults(entry.ID, entry.Repo)
		if len(plan) < 2 || plan[0].Names[0] != "one-cli" || plan[1].Source != findSource || !reflect.DeepEqual(plan[1].Names, []string{"find-skills"}) {
			t.Fatalf("missing common skills: %s %v", entry.ID, plan)
		}
		if entry.Toolchain != "none" && len(plan) <= 2 {
			t.Fatalf("unmapped template: %s", entry.ID)
		}
		for _, selection := range plan {
			for _, name := range selection.Names {
				if strings.Contains(name, "auth") || strings.Contains(name, "business") {
					t.Fatalf("unexpected automatic skill: %s", name)
				}
			}
		}
	}
	if got := Defaults("react-spa", "https://example.com/custom"); len(got) != 2 {
		t.Fatalf("custom template inherited built-in assumptions: %v", got)
	}
}
