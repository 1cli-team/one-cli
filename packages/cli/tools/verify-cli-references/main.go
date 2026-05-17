// verify-cli-references walks every prose / template doc in the repo and
// asserts that any `one <subcommand>` reference inside a code-span or
// fenced code block names a real, currently-registered subcommand.
//
// Run via Taskfile: `task verify-cli-references`. Exits non-zero with a
// list of file:line offenders on any drift.
//
// Source of truth: cobra command tree, accessed via cli.RootCmd(). All
// side-effect command registrations have run by the time RootCmd()
// returns, so this stays in sync automatically when commands are added,
// removed, or renamed.
//
// What we scan:
//   - Top-level: README.md, CLAUDE.md, CONTRIBUTING.md
//   - apps/docs/content/docs/**/*.{md,mdx}
//   - packages/skills/one-cli/SKILL.md and references/**/*.md
//   - packages/templates/<id>/README.md and README.md.hbs
//
// Where we look (intentional):
//   - Inside `inline code spans` and ```fenced code blocks```.
//   - NOT in plain prose. The English word "one" appears constantly
//     ("any one of these", "one third", etc.) and false-positives there
//     would be loud and unhelpful. Any real command reference users
//     copy-paste is in backticks anyway.
//
// Escape hatch: lines between `<!-- verify-cli:ignore-start -->` and
// `<!-- verify-cli:ignore-end -->` are skipped. Use this for migration
// guides, "removed commands" tables, and changelogs that intentionally
// reference now-deleted command names.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/cli"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "verify-cli-references:", err)
		os.Exit(1)
	}
}

func run() error {
	valid := collectCommands(cli.RootCmd())
	files, err := docFiles()
	if err != nil {
		return err
	}
	var problems []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		problems = append(problems, scanFile(f, b, valid)...)
	}
	if len(problems) > 0 {
		return fmt.Errorf(
			"found unknown `one <subcommand>` references in docs:\n  %s\n\nFix one of:\n  - Rename to a registered command (see `one --help`).\n  - If the reference is intentional (migration guide / changelog), wrap the section in:\n      <!-- verify-cli:ignore-start -->\n      ... section with deprecated command names ...\n      <!-- verify-cli:ignore-end -->",
			strings.Join(problems, "\n  "),
		)
	}
	keys := make([]string, 0, len(valid))
	for k := range valid {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("verify-cli-references: ok (%d files, %d top-level commands: %s)\n",
		len(files), len(keys), strings.Join(keys, ", "))
	return nil
}

// collectCommands walks rootCmd and gathers the set of valid top-level
// subcommand names plus their aliases.
func collectCommands(root *cobra.Command) map[string]struct{} {
	set := make(map[string]struct{})
	for _, c := range root.Commands() {
		if c.Name() == "" {
			continue
		}
		set[c.Name()] = struct{}{}
		for _, alias := range c.Aliases {
			set[alias] = struct{}{}
		}
	}
	// Brand-name exemption: `one cli` is the project name (the binary
	// is `one`, but the project everywhere is "one cli"). It's not a
	// command and never will be — adding it here lets prose like
	// `one cli` is a Go binary` pass without per-occurrence ignore
	// wrappers in every doc that names the project.
	set["cli"] = struct{}{}
	return set
}

// repoRel resolves a path relative to the repo root. Taskfile invokes
// this tool with `dir: packages/cli`, so we walk two levels up.
func repoRel(parts ...string) string {
	return filepath.Join(append([]string{"..", ".."}, parts...)...)
}

func docFiles() ([]string, error) {
	var files []string

	for _, top := range []string{"README.md", "CLAUDE.md", "CONTRIBUTING.md"} {
		p := repoRel(top)
		if _, err := os.Stat(p); err == nil {
			files = append(files, p)
		}
	}

	type walkSpec struct {
		root      string
		extension []string
	}
	roots := []walkSpec{
		{repoRel("apps", "docs", "content", "docs"), []string{".md", ".mdx"}},
		{repoRel("packages", "skills"), []string{".md"}},
	}
	for _, w := range roots {
		if err := filepath.WalkDir(w.root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			for _, ext := range w.extension {
				if strings.HasSuffix(path, ext) {
					files = append(files, path)
					return nil
				}
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	for _, glob := range []string{
		filepath.Join(repoRel("packages", "templates"), "*", "README.md"),
		filepath.Join(repoRel("packages", "templates"), "*", "README.md.hbs"),
	} {
		matches, err := filepath.Glob(glob)
		if err != nil {
			return nil, err
		}
		files = append(files, matches...)
	}

	sort.Strings(files)
	return files, nil
}

var (
	// oneCmdRE matches a `one ` invocation followed by a lowercase token
	// that could plausibly be a subcommand. The token must start with
	// a-z to filter out flags (`-h`), placeholders (`<dir>`), and meta
	// (`--version`).
	oneCmdRE = regexp.MustCompile(`\bone\s+([a-z][a-z0-9-]*)`)

	// codeSpanRE matches inline `code spans` (single backticks). Avoids
	// crossing newlines.
	codeSpanRE = regexp.MustCompile("`([^`\n]+)`")
)

func scanFile(path string, content []byte, valid map[string]struct{}) []string {
	var problems []string
	ignore := false
	inFence := false

	for i, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)

		// Fence tracking runs first, unconditionally. If we deferred it
		// behind the ignore check, a closing ``` inside an ignore block
		// would never toggle inFence and every subsequent prose line
		// would be misread as still-inside-fence.
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}

		if strings.Contains(line, "verify-cli:ignore-start") {
			ignore = true
			continue
		}
		if strings.Contains(line, "verify-cli:ignore-end") {
			ignore = false
			continue
		}
		if ignore {
			continue
		}

		var sources []string
		if inFence {
			sources = []string{line}
		} else {
			for _, m := range codeSpanRE.FindAllStringSubmatch(line, -1) {
				sources = append(sources, m[1])
			}
		}

		for _, s := range sources {
			for _, m := range oneCmdRE.FindAllStringSubmatch(s, -1) {
				cmd := m[1]
				if _, ok := valid[cmd]; !ok {
					problems = append(problems, fmt.Sprintf(
						"%s:%d: `one %s …` — `%s` is not a registered subcommand",
						path, i+1, cmd, cmd,
					))
				}
			}
		}
	}
	return problems
}
