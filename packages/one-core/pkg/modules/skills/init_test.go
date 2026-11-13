package skills

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// Lifecycle tests use a deterministic hash stub; runner_test separately checks
// the production hasher against the upstream algorithm and locale ordering.
func testHashes(_ context.Context, _ string, all map[string]Files) (map[string]string, error) {
	result := map[string]string{}
	for name, files := range all {
		paths := make([]string, 0, len(files))
		for path := range files {
			paths = append(paths, path)
		}
		slices.Sort(paths)
		hash := sha256.New()
		for _, path := range paths {
			hash.Write([]byte(path))
			hash.Write(files[path])
		}
		result[name] = hex.EncodeToString(hash.Sum(nil))
	}
	return result, nil
}

func putFiles(t *testing.T, root, name string, files Files) {
	t.Helper()
	for path, raw := range files {
		target := filepath.Join(root, ".agents", "skills", name, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func writeTestLock(t *testing.T, root, name string, files Files, source, kind, ref string) []byte {
	t.Helper()
	hashes, _ := testHashes(context.Background(), root, map[string]Files{name: files})
	lock := map[string]any{
		"version": 1, "extension": "preserve me",
		"skills": map[string]any{
			name:          lockEntry{Source: source, SourceType: kind, Ref: ref, ComputedHash: hashes[name]},
			"third-party": map[string]any{"source": "./custom", "sourceType": "local", "computedHash": "custom", "extension": true},
		},
	}
	raw, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skills-lock.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestInitInstallsMissingAndRegistersGitHubSource(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	root := t.TempDir()
	initializer := Initializer{Hash: testHashes}
	preview, err := initializer.Init(context.Background(), root, nil, true)
	if err != nil || len(preview.Skills) != 1 || preview.Skills[0].Status != "would_install" {
		t.Fatalf("preview = %+v, %v", preview, err)
	}
	if exists, _ := installed(root, "one-cli"); exists {
		t.Fatal("preview installed files")
	}
	result, err := initializer.Init(context.Background(), root, nil, false)
	if err != nil || result.Skills[0].Status != "installed" {
		t.Fatalf("init = %+v, %v", result, err)
	}
	if same, err := matchesBundledSkill(root, "one-cli"); err != nil || !same {
		t.Fatalf("bundle mismatch: %v", err)
	}
	lock, err := readLock(root)
	if err != nil {
		t.Fatal(err)
	}
	entry := lock["one-cli"]
	if entry.Source != oneSource || entry.SourceType != "github" || entry.Ref != "master" || entry.SkillPath != "packages/agent-skills/skills/one-cli/SKILL.md" || len(entry.ComputedHash) != 64 {
		t.Fatalf("not ready for upstream update: %+v", entry)
	}
	before, _ := os.ReadFile(filepath.Join(root, "skills-lock.json"))
	again, err := initializer.Init(context.Background(), root, nil, false)
	after, _ := os.ReadFile(filepath.Join(root, "skills-lock.json"))
	if err != nil || again.Skills[0].Status != "managed" || len(again.Changes) != 0 || !bytes.Equal(before, after) {
		t.Fatalf("repeat init changed registration: %+v, %v", again, err)
	}
}

func TestInitMigratesSourceWithoutReplacingOldFiles(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	root := t.TempDir()
	old := Files{"SKILL.md": []byte("old instructions"), "obsolete.md": []byte("old reference")}
	putFiles(t, root, "one-cli", old)
	beforeLock := writeTestLock(t, root, "one-cli", old, "./.agents/skills", "local", "")
	initializer := Initializer{Hash: testHashes}
	preview, err := initializer.Init(context.Background(), root, nil, true)
	if err != nil || preview.Skills[0].Status != "would_register" || !slices.Equal(preview.Changes, []string{"skills-lock.json"}) {
		t.Fatalf("preview = %+v, %v", preview, err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "skills-lock.json")); !bytes.Equal(got, beforeLock) {
		t.Fatal("preview changed lock")
	}
	result, err := initializer.Init(context.Background(), root, nil, false)
	if err != nil || result.Skills[0].Status != "registered" {
		t.Fatalf("init = %+v, %v", result, err)
	}
	if got, _ := snapshotSkill(root, "one-cli"); !sameFiles(got, old) {
		t.Fatal("registration replaced old skill files")
	}
	afterLock, _ := os.ReadFile(filepath.Join(root, "skills-lock.json"))
	var before, after map[string]any
	_ = json.Unmarshal(beforeLock, &before)
	_ = json.Unmarshal(afterLock, &after)
	if after["extension"] != before["extension"] || !reflect.DeepEqual(before["skills"].(map[string]any)["third-party"], after["skills"].(map[string]any)["third-party"]) {
		t.Fatal("unrelated lock entries changed")
	}
	entry := after["skills"].(map[string]any)["one-cli"].(map[string]any)
	if entry["sourceType"] != "github" || entry["ref"] != "master" || entry["computedHash"] != before["skills"].(map[string]any)["one-cli"].(map[string]any)["computedHash"] {
		t.Fatalf("invalid migration: %+v", entry)
	}
}

func TestInitPreservesUserChangesAndSources(t *testing.T) {
	for _, change := range []string{"modified", "extra file", "deleted file", "custom source", "untracked", "symlink", "agent copy"} {
		t.Run(change, func(t *testing.T) {
			t.Setenv("ONE_HOME", t.TempDir())
			root := t.TempDir()
			old := Files{"SKILL.md": []byte("old"), "reference.md": []byte("reference")}
			putFiles(t, root, "one-cli", old)
			source := "./.agents/skills"
			if change == "custom source" {
				source = "./team-skills"
			}
			lock := writeTestLock(t, root, "one-cli", old, source, "local", "")
			switch change {
			case "modified":
				putFiles(t, root, "one-cli", Files{"SKILL.md": []byte("team edits")})
			case "extra file":
				putFiles(t, root, "one-cli", Files{"extra.md": []byte("notes")})
			case "deleted file":
				if err := os.Remove(filepath.Join(root, ".agents/skills/one-cli/reference.md")); err != nil {
					t.Fatal(err)
				}
			case "untracked":
				if err := os.Remove(filepath.Join(root, "skills-lock.json")); err != nil {
					t.Fatal(err)
				}
				lock = nil
			case "symlink":
				if err := os.Symlink(t.TempDir(), filepath.Join(root, ".agents/skills/one-cli/linked")); err != nil {
					t.Skip(err)
				}
			case "agent copy":
				if err := os.MkdirAll(filepath.Join(root, ".claude/skills/one-cli"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := snapshotSkill(root, "one-cli")
			result, err := (Initializer{Hash: testHashes}).Init(context.Background(), root, nil, false)
			if err != nil || result.Skills[0].Status != "skipped" {
				t.Fatalf("init = %+v, %v", result, err)
			}
			after, _ := snapshotSkill(root, "one-cli")
			if !sameFiles(before, after) {
				t.Fatal("skipped files changed")
			}
			if got, _ := os.ReadFile(filepath.Join(root, "skills-lock.json")); !bytes.Equal(got, lock) {
				t.Fatal("skipped lock changed")
			}
		})
	}
}

func TestInitKeepsManagedBranchesAndPins(t *testing.T) {
	for _, ref := range []string{"master", "custom-branch", "v1.0.0", strings.Repeat("a", 40)} {
		t.Run(ref, func(t *testing.T) {
			t.Setenv("ONE_HOME", t.TempDir())
			root := t.TempDir()
			old := Files{"SKILL.md": []byte("remote instructions")}
			putFiles(t, root, "one-cli", old)
			raw := writeTestLock(t, root, "one-cli", old, oneSource, "github", ref)
			var doc map[string]any
			_ = json.Unmarshal(raw, &doc)
			doc["skills"].(map[string]any)["one-cli"].(map[string]any)["skillPath"] = officialSkillPath("one-cli")
			raw, _ = json.Marshal(doc)
			if err := os.WriteFile(filepath.Join(root, "skills-lock.json"), raw, 0o644); err != nil {
				t.Fatal(err)
			}
			result, err := (Initializer{}).Init(context.Background(), root, nil, false)
			if err != nil || result.Skills[0].Status != "managed" {
				t.Fatalf("init = %+v, %v", result, err)
			}
			if after, _ := os.ReadFile(filepath.Join(root, "skills-lock.json")); !bytes.Equal(raw, after) {
				t.Fatal("changed managed source")
			}
		})
	}
}

func TestInitRegistersExactUntrackedBundleAndExplicitSkills(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	root := t.TempDir()
	files, _ := bundledFiles("one-cli")
	putFiles(t, root, "one-cli", files)
	result, err := (Initializer{Hash: testHashes}).Init(context.Background(), root, []string{"one-cli", "one-go", "one-go"}, false)
	if err != nil || len(result.Skills) != 2 || result.Skills[0].Status != "registered" || result.Skills[1].Status != "installed" {
		t.Fatalf("init = %+v, %v", result, err)
	}
	if _, err := (Initializer{Hash: testHashes}).Init(context.Background(), root, []string{"../outside"}, false); err == nil {
		t.Fatal("unknown skill accepted")
	}
}

func TestInitFailureAndConcurrentEditLeaveFilesUnchanged(t *testing.T) {
	for _, mode := range []string{"hash failure", "concurrent edit", "concurrent addition"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("ONE_HOME", t.TempDir())
			root := t.TempDir()
			old := Files{"SKILL.md": []byte("old")}
			putFiles(t, root, "one-cli", old)
			lock := writeTestLock(t, root, "one-cli", old, "./.agents/skills", "local", "")
			hash := func(ctx context.Context, dir string, files map[string]Files) (map[string]string, error) {
				if mode == "hash failure" {
					return nil, errors.New("hash failure")
				}
				path := "SKILL.md"
				if mode == "concurrent addition" {
					path = "added.md"
				}
				putFiles(t, root, "one-cli", Files{path: []byte("concurrent edit")})
				return testHashes(ctx, dir, files)
			}
			if _, err := (Initializer{Hash: hash}).Init(context.Background(), root, nil, false); err == nil {
				t.Fatal("expected failure")
			}
			if got, _ := os.ReadFile(filepath.Join(root, "skills-lock.json")); !bytes.Equal(got, lock) {
				t.Fatal("failed init changed lock")
			}
		})
	}
}
