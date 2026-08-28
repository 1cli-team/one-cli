// verify-help refuses structural drift in help text that the snapshot
// tests cannot catch.
//
// Run via Taskfile: `task verify-help`. Exits non-zero with one
// human-readable line per problem.
//
// Two checks:
//
//   - **help catalogue completeness.** The concise root help deliberately
//     lists only the six everyday commands. `one help --all` is generated
//     from the Cobra tree and must contain every registered top-level
//     command.
//
//   - **Example-block flag existence.** For every command in the
//     cobra tree, any line in cmd.Long or cmd.Example that looks like
//     a real command invocation (`one <subcmd> [args] [--flags]`) gets
//     parsed: we walk the cobra tree to find the command that line is
//     demonstrating, then check every --flag token against that
//     command's local + inherited flag set. Catches the post-rename
//     case where a flag still appears in an example but no longer
//     exists. Lines that don't resolve to a real command path are
//     skipped — prose like `set the --foo flag` outside an example
//     block isn't validated.
//
// Snapshots in testdata/reference/help/ catch text-level edits;
// verify-help catches the structural-only drifts (rootHelp missing /
// extra command, example using a flag that no longer exists). The two
// complement each other.
package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/cli"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/helpui"
)

func main() {
	problems := run()
	if len(problems) > 0 {
		fmt.Fprintln(os.Stderr, "verify-help: drift found")
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "  %s\n", p)
		}
		fmt.Fprintln(os.Stderr, "\nFix one of:")
		fmt.Fprintln(os.Stderr, "  - Keep root help limited to the six everyday commands and keep `one help --all` complete.")
		fmt.Fprintln(os.Stderr, "  - Update the Example / Long text in the offending cmd.go to use a flag that actually exists.")
		fmt.Fprintln(os.Stderr, "  - Re-run with UPDATE_SNAPSHOTS=1 if you have also intentionally changed help text:")
		fmt.Fprintln(os.Stderr, "      UPDATE_SNAPSHOTS=1 go test ./internal/cli/ -run TestHelpSnapshots")
		os.Exit(1)
	}
	root := cli.RootCmd()
	names := topLevelNames(root)
	fmt.Printf("verify-help: ok (%d top-level commands: %s)\n",
		len(names), strings.Join(names, ", "))
}

func run() []string {
	root := cli.RootCmd()
	var problems []string
	problems = append(problems, checkRootHelp(root)...)
	problems = append(problems, checkAllHelp(root)...)
	problems = append(problems, checkExampleFlags(root)...)
	return problems
}

// topLevelNames returns the sorted list of registered top-level
// command names (excluding cobra's auto-injected `help`).
func topLevelNames(root *cobra.Command) []string {
	names := []string{}
	for _, c := range root.Commands() {
		if c.Hidden || c.Deprecated != "" {
			continue
		}
		if c.Name() == "help" {
			continue
		}
		names = append(names, c.Name())
	}
	sort.Strings(names)
	return names
}

// rootHelpCommandRE matches a COMMANDS-block line: 2-or-more spaces,
// then a lowercase command identifier, then more whitespace + prose.
// We restrict to lines that begin with exactly two spaces (the block's
// canonical indent) so we don't match every indented thing in the file.
var rootHelpCommandRE = regexp.MustCompile(`^  ([a-z][a-z0-9-]*)\s{2,}`)

// checkRootHelp parses the EVERYDAY COMMANDS block out of `one --help` and
// requires exactly the intentionally small daily command set.
func checkRootHelp(root *cobra.Command) []string {
	// Re-render the root help via the same path Execute() takes for
	// `one --help`. RootCmd does not own the help text directly
	// (it's intercepted in Execute), so we re-fetch it through the
	// public renderer hook below.
	helpText := rootHelpText()
	listed := map[string]bool{}
	inBlock := false
	for _, line := range strings.Split(helpText, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "EVERYDAY COMMANDS" || trimmed == "日常命令" {
			inBlock = true
			continue
		}
		if inBlock {
			// Block ends at the next ALL-CAPS section header.
			if trimmed != "" && trimmed == strings.ToUpper(trimmed) &&
				!strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "\t") {
				inBlock = false
				continue
			}
			if m := rootHelpCommandRE.FindStringSubmatch(line); m != nil {
				listed[m[1]] = true
			}
		}
	}

	want := map[string]bool{
		"create": true, "add": true, "dev": true,
		"deploy": true, "env": true, "configure": true,
	}

	var problems []string
	for name := range listed {
		if !want[name] {
			problems = append(problems,
				fmt.Sprintf("root help advertises non-everyday command %q", name))
		}
	}
	for name := range want {
		if !listed[name] {
			problems = append(problems,
				fmt.Sprintf("root help is missing everyday command %q", name))
		}
		if root.CommandPath() == "" {
			continue
		}
		if command, _, err := root.Find([]string{name}); err != nil || command == root {
			problems = append(problems,
				fmt.Sprintf("root help lists everyday command %q but it is not registered", name))
		}
	}
	sort.Strings(problems)
	return problems
}

func checkAllHelp(root *cobra.Command) []string {
	var buf strings.Builder
	helpui.RenderAll(root, &buf)
	text := buf.String()
	var problems []string
	for _, name := range topLevelNames(root) {
		if !regexp.MustCompile(`(?m)^  ` + regexp.QuoteMeta(name) + `\s`).MatchString(text) {
			problems = append(problems,
				fmt.Sprintf("one help --all is missing top-level command %q", name))
		}
	}
	sort.Strings(problems)
	return problems
}

// rootHelpText returns the curated rootHelp constant string. We re-
// render it through the same `one --help` path Execute() uses
// (shouldRenderRootHelp → os.Stdout.WriteString(rootHelp)) by calling
// the package-level test helper. Since rootHelp is an unexported
// constant, we expose it only via cli.RootHelp(); see internal/cli.
func rootHelpText() string {
	return cli.RootHelp()
}

// flagInExampleRE matches `--flag` tokens. The trailing word boundary
// keeps us from accidentally chopping into = or other punctuation;
// downstream code strips trailing punctuation explicitly.
var flagInExampleRE = regexp.MustCompile(`--([a-z][a-z0-9-]*)`)

// invocationRE matches a line that looks like a real CLI invocation:
//
//	(start | whitespace) one <subcmd>...
//
// We anchor on `one ` so prose mentioning the bare `--flag` outside
// a command line is ignored. Continuation lines ending with `\` are
// joined back together before scanning.
var invocationRE = regexp.MustCompile(`(^|\s)one\s+([a-z][a-z0-9/-]*(?:\s+[a-z][a-z0-9/-]*)*)`)

// checkExampleFlags walks every command, scans its Example + Long
// text for command invocations, resolves each invocation to a real
// command, and validates every --flag token in that invocation
// against the resolved command's flag set.
func checkExampleFlags(root *cobra.Command) []string {
	var problems []string
	var visit func(c *cobra.Command)
	visit = func(c *cobra.Command) {
		for _, source := range []struct {
			field, text string
		}{
			{"Example", c.Example},
			{"Long", c.Long},
		} {
			problems = append(problems, scanInvocations(root, c, source.field, source.text)...)
		}
		for _, child := range c.Commands() {
			if child.Hidden || child.Deprecated != "" {
				continue
			}
			if child.Name() == "help" {
				continue
			}
			visit(child)
		}
	}
	visit(root)
	sort.Strings(problems)
	return problems
}

// scanInvocations parses one piece of help text (Example or Long) for
// lines that look like real `one ...` invocations, resolves each to a
// command, and validates every --flag token.
func scanInvocations(root, owner *cobra.Command, field, text string) []string {
	if text == "" {
		return nil
	}
	// Join continuation lines ending with `\` so multi-line examples
	// (common in configure add invocations) are scanned as a single
	// command. Strip trailing backslash + newline.
	joined := strings.ReplaceAll(text, "\\\n", " ")

	var problems []string
	for _, rawLine := range strings.Split(joined, "\n") {
		line := strings.TrimSpace(rawLine)
		// Comments inside examples (after `#`) are prose; ignore.
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		// Strip leading shell prompt or pipe noise so the matcher
		// still anchors on `one`.
		line = strings.TrimPrefix(line, "$ ")

		m := invocationRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		// The first capture group can include the leading whitespace;
		// strip and split into command-path tokens. invocationRE
		// already limited the capture to [a-z0-9/-] runs separated by
		// whitespace, so URL fragments / placeholders / flags can't
		// appear here. We DO keep slashes — configurecmd's leaf
		// commands have names like "env/infisical".
		pathStr := strings.TrimSpace(m[2])
		pathTokens := strings.Fields(pathStr)
		resolved := resolveCommand(root, pathTokens)
		if resolved == nil {
			// Path didn't resolve to a real command — could be a
			// historical example, a placeholder like `one <command>`,
			// or just prose. Don't flag, just skip.
			continue
		}

		// Extract --flag tokens from the FULL line so we catch flags
		// even after the command path.
		flagSet := collectFlagSet(resolved)
		for _, fm := range flagInExampleRE.FindAllStringSubmatch(line, -1) {
			flag := fm[1]
			if flag == "help" || flag == "version" {
				// Cobra always provides these on every command.
				continue
			}
			if !flagSet[flag] {
				problems = append(problems, fmt.Sprintf(
					"%s.%s references --%s but command %q has no such flag",
					owner.CommandPath(), field, flag, resolved.CommandPath(),
				))
			}
		}
	}
	return problems
}

// resolveCommand walks the cobra tree following pathTokens, stopping
// at the deepest token that resolves to a real subcommand. The first
// token may be the binary name itself (`one`) which we drop.
// Trailing positional args (e.g. `one container build user-api`) are
// not commands, so we stop and return the deepest valid prefix
// (`one container build`) rather than returning nil. Flags on
// `user-api` should be validated against `container build`.
//
// Returns nil only when the very first token doesn't match any child.
func resolveCommand(root *cobra.Command, pathTokens []string) *cobra.Command {
	if len(pathTokens) == 0 {
		return nil
	}
	if pathTokens[0] == "one" || pathTokens[0] == root.Name() {
		pathTokens = pathTokens[1:]
	}
	cur := root
	matched := false
	for _, t := range pathTokens {
		var next *cobra.Command
		for _, c := range cur.Commands() {
			if c.Name() == t {
				next = c
				break
			}
			for _, alias := range c.Aliases {
				if alias == t {
					next = c
					break
				}
			}
			if next != nil {
				break
			}
		}
		if next == nil {
			// Positional arg — stop walking, return deepest match.
			break
		}
		cur = next
		matched = true
	}
	if !matched {
		return nil
	}
	return cur
}

// collectFlagSet returns the set of --flag names that may legitimately
// appear in cmd's example text:
//
//   - cmd's local + inherited persistent flags
//   - if cmd has subcommands (i.e. it's a verb-group parent like
//     `configure add`), also the union of every descendant's local
//     flags. Parent Long blocks typically demonstrate the shape of
//     subcommand invocations: `one configure add <pair> [--profile
//     <name>]` is a real example; --profile lives on every leaf even
//     though it isn't a flag of `configure add` itself.
func collectFlagSet(cmd *cobra.Command) map[string]bool {
	set := map[string]bool{}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		set[f.Name] = true
	})
	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		set[f.Name] = true
	})
	if cmd.HasSubCommands() {
		var union func(c *cobra.Command)
		union = func(c *cobra.Command) {
			for _, child := range c.Commands() {
				child.Flags().VisitAll(func(f *pflag.Flag) {
					set[f.Name] = true
				})
				union(child)
			}
		}
		union(cmd)
	}
	return set
}
