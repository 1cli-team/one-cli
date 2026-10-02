package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestE2E_CreateInstallsCommonSkillsInWorkspace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fake npx; cross-platform runner is covered by helper-process tests")
	}
	parent := t.TempDir()
	home := t.TempDir()
	isolateHome(t, home)
	globalPath := filepath.Join(home, ".agents", "skills", "one-cli", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(globalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	globalContent := []byte("user's global guidance")
	if err := os.WriteFile(globalPath, globalContent, 0o644); err != nil {
		t.Fatal(err)
	}
	tools := t.TempDir()
	tracePath := filepath.Join(tools, "trace")
	t.Setenv("ONE_SKILLS_TEST_TRACE", tracePath)
	script := `#!/bin/sh
set -eu
printf '%s\n' "$PWD" "$@" >> "$ONE_SKILLS_TEST_TRACE"
printf 'SKILLS_UPSTREAM_BANNER\nTip: use --global\n'
printf 'UPSTREAM_STDERR_NOTICE\n' >&2
for arg in "$@"; do
  case "$arg" in --global|-g) exit 17 ;; esac
done
mkdir -p ".agents/skills/$6"
case "$6" in
  one-cli) test "$4" = './.agents/skills'; test -s "$4/one-cli/SKILL.md" ;;
  find-skills) printf 'discovery fixture\n' > .agents/skills/find-skills/SKILL.md ;;
  *) exit 18 ;;
esac
`
	if err := os.WriteFile(filepath.Join(tools, "npx"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	root := filepath.Join(parent, "demo")
	cmd := exec.Command(binaryPath(t), "create", root, "--yes", "-o", "json")
	cmd.Dir = parent
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("create: %v stdout=%s stderr=%s", err, &out, &stderr)
	}
	mustParseJSON(t, out.String())
	for _, hidden := range []string{"SKILLS_UPSTREAM_BANNER", "Tip: use --global", "UPSTREAM_STDERR_NOTICE"} {
		if strings.Contains(out.String(), hidden) || strings.Contains(stderr.String(), hidden) {
			t.Fatalf("automatic install leaked upstream UI: stdout=%s stderr=%s", &out, &stderr)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, ".agents", "skills"))
	if err != nil || len(entries) != 2 || entries[0].Name() != "find-skills" || entries[1].Name() != "one-cli" {
		t.Fatalf("unexpected project skills: %v %v", entries, err)
	}
	localContent, err := os.ReadFile(filepath.Join(root, ".agents", "skills", "one-cli", "SKILL.md"))
	if err != nil || len(localContent) == 0 || bytes.Equal(localContent, globalContent) {
		t.Fatalf("global skill was used instead of a project installation: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".one")); !os.IsNotExist(err) {
		t.Fatalf("create left a duplicate skill source directory: %v", err)
	}
	ignored := exec.Command("git", "check-ignore", "--no-index", "--stdin")
	ignored.Dir = root
	ignored.Stdin = strings.NewReader(".one/cache.json\n.agents/skills/one-cli/SKILL.md\nskills-lock.json\n")
	if got, err := ignored.CombinedOutput(); err != nil || string(got) != ".one/cache.json\n" {
		t.Fatalf("local state must be ignored while skills and their lock remain versionable: %q %v", got, err)
	}
	globalAfter, err := os.ReadFile(globalPath)
	if err != nil || !bytes.Equal(globalAfter, globalContent) {
		t.Fatalf("global guidance changed: %v", err)
	}
	trace, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(trace), canonical+"\n") != 2 || strings.Count(string(trace), "skills@1.7.0\n") != 2 || !strings.Contains(string(trace), "\nfind-skills\n") || !strings.Contains(string(trace), "\nvercel-labs/skills#") || strings.Contains(string(trace), "--global") {
		t.Fatalf("common skills did not install from their sources in the project: %s", trace)
	}
}

func TestE2E_SkillsPreservesUpstreamArgumentsHelpAndExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fake npx; cross-platform runner is covered by helper-process tests")
	}
	root := t.TempDir()
	isolateHome(t, t.TempDir())
	tools := t.TempDir()
	script := "#!/bin/sh\npwd\nprintf '%s\\n' \"$@\"\nprintf 'native stderr\\n' >&2\nexit 37\n"
	if err := os.WriteFile(filepath.Join(tools, "npx"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--help"}, {"add", "owner/repo", "--skill", "a b", "", "$(literal)", "--global", "--yes", "--agent", "codex", "--output", "json"}} {
		out, stderr, code := runBinaryIn(t, root, append([]string{"skills"}, args...)...)
		want := canonical + "\n--yes\nskills@1.7.0\n" + strings.Join(args, "\n") + "\n"
		if code != 37 || out != want || stderr != "native stderr\n" {
			t.Fatalf("upstream semantics changed: code=%d out=%q stderr=%q, want %q", code, out, stderr, want)
		}
	}
}
