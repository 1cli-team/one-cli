package cli

// Help-text drift snapshots. Walks the full cobra command tree, renders
// each command's --help output in-process, and compares against a
// per-command .txt fixture under testdata/reference/help/. Any rename,
// removal, or text edit produces a visible diff in PRs.
//
// Root special-case: `one --help` is intercepted in Execute() before
// cobra runs (see shouldRenderRootHelp) so users see the curated
// rootHelp constant. We snapshot that constant directly as root.txt
// rather than cobra's auto-generated root help — which users never see.
//
// Refresh fixtures with: UPDATE_SNAPSHOTS=1 go test ./internal/cli/ -run TestHelpSnapshots
//
// Pair with tools/verify-help, which enforces structural invariants
// snapshots cannot catch (rootHelp lists every registered command;
// every flag named in an Example: block actually exists on that
// command).

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/i18n"
)

// TestMain forces every help snapshot test (and any cmd help renders
// downstream) to run in DefaultLocale for determinism. Production
// code calls i18n.Init from Execute; tests don't go through Execute,
// so they'd otherwise inherit whatever the developer's $LANG happens
// to be — which would make snapshots non-portable.
func TestMain(m *testing.M) {
	_ = i18n.Init(i18n.DefaultLocale)
	i18n.RefreshTree(RootCmd())
	os.Exit(m.Run())
}

// helpSnapshotDir returns the absolute path to testdata/reference/help/.
// Derived from this file's location so `go test` works from any cwd.
func helpSnapshotDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	// snapshot_help_test.go lives at packages/cli/internal/cli/ — two
	// dirs up gets us to packages/cli/, which contains testdata/.
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "reference", "help")
}

// snapshotName turns a command path like ["configure", "add", "env/infisical"]
// into a safe filename "configure_add_env-infisical.txt". The slash inside
// "env/infisical" is replaced with a dash so we don't accidentally write
// into a nested directory.
func snapshotName(path []string) string {
	if len(path) == 0 {
		return "root.txt"
	}
	parts := make([]string, len(path))
	for i, p := range path {
		parts[i] = strings.ReplaceAll(p, "/", "-")
	}
	return strings.Join(parts, "_") + ".txt"
}

// renderHelp returns the same output a user gets from `one <path...> --help`.
// We capture cobra's default HelpFunc into a buffer rather than exec'ing
// the binary — same template, deterministic across machines, doesn't
// require `task build` first.
func renderHelp(t *testing.T, cmd *cobra.Command) string {
	t.Helper()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	// Cobra's default HelpFunc is set on the root command and inherits
	// down the tree; calling Help() on any subcommand uses it.
	if err := cmd.Help(); err != nil {
		t.Fatalf("cmd.Help() for %q failed: %v", cmd.CommandPath(), err)
	}
	return buf.String()
}

// walkTree returns every command in the tree under root in stable
// alphabetical order, paired with its path relative to root (the root
// itself gets an empty path). Hidden / deprecated commands are skipped
// the way cobra would skip them in --help output, so what we snapshot
// is what the user sees.
type cmdWalk struct {
	cmd  *cobra.Command
	path []string
}

func walkTree(root *cobra.Command) []cmdWalk {
	var out []cmdWalk
	var visit func(c *cobra.Command, path []string)
	visit = func(c *cobra.Command, path []string) {
		out = append(out, cmdWalk{cmd: c, path: append([]string{}, path...)})
		children := append([]*cobra.Command{}, c.Commands()...)
		sort.Slice(children, func(i, j int) bool {
			return children[i].Name() < children[j].Name()
		})
		for _, child := range children {
			if child.Hidden || child.Deprecated != "" {
				continue
			}
			// Cobra auto-injects a `help` command on every parent with
			// subcommands. It's identical across every command, so
			// snapshotting it everywhere would be pure noise.
			if child.Name() == "help" {
				continue
			}
			visit(child, append(path, child.Name()))
		}
	}
	visit(root, nil)
	// Drop the root entry (we snapshot the intercepted rootHelp
	// constant separately — cobra's auto-generated root help is never
	// shown to users).
	if len(out) > 0 && len(out[0].path) == 0 {
		out = out[1:]
	}
	return out
}

// assertHelpSnapshot compares got against the file at testdata/reference/help/<name>.
// Mismatch fails with a hint to re-run with UPDATE_SNAPSHOTS=1.
func assertHelpSnapshot(t *testing.T, name, got string) {
	t.Helper()
	dir := helpSnapshotDir(t)
	path := filepath.Join(dir, name)

	if os.Getenv("UPDATE_SNAPSHOTS") == "1" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		t.Logf("UPDATE_SNAPSHOTS=1: wrote %s (%d bytes)", path, len(got))
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v\n  hint: run with UPDATE_SNAPSHOTS=1 to create", path, err)
	}
	if string(want) != got {
		t.Errorf("help-text drift vs %s\n--- want\n%s\n--- got\n%s\n--- hint\nUPDATE_SNAPSHOTS=1 go test ./internal/cli/ -run TestHelpSnapshots",
			path, string(want), got)
	}
}

// TestHelpSnapshots locks every subcommand's --help text against a
// per-command .txt fixture. Drives the cobra tree from RootCmd() so new
// subcommands automatically participate.
func TestHelpSnapshots(t *testing.T) {
	root := RootCmd()
	walks := walkTree(root)
	if len(walks) == 0 {
		t.Fatal("walkTree returned 0 commands — registration broken?")
	}
	for _, w := range walks {
		name := snapshotName(w.path)
		t.Run(name, func(t *testing.T) {
			got := renderHelp(t, w.cmd)
			assertHelpSnapshot(t, name, got)
		})
	}
}

// TestRootHelpSnapshot locks the curated root help text (the
// i18n-resolved "root.help" key). Bypasses cobra because `one --help`
// does too (see shouldRenderRootHelp in Execute). Catches drift in
// the manually-maintained COMMANDS block.
//
// Renders in DefaultLocale (en-US) for snapshot determinism — the
// snapshot is the English fixture; per-locale snapshots can be
// layered in later if we localise further.
func TestRootHelpSnapshot(t *testing.T) {
	_ = i18n.Init(i18n.DefaultLocale)
	assertHelpSnapshot(t, "root.txt", RootHelp())
}
