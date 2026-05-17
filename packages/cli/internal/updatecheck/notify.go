package updatecheck

// Synchronous read-and-print path. Called from cli.Execute()'s defer so
// it runs on every command exit, including error paths. Skip rules
// short-circuit before doing any work — see shouldSkip for the full list.

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
)

// installCommand is what the warning tells the user to run. Matches the
// command in [README.md](README.md) so the notification's recommendation
// is the same path users have already seen.
const installCommand = "curl -fsSL https://1cli.dev/install.sh | bash"

// Notify reads the cached check result and prints a yellow warning to
// stderr if a strictly newer version is available. Idempotent and
// silent on every skip path — must never print anything that could
// pollute structured output (we already gate on output.IsTTY()).
//
// currentVersion is the binary's compiled-in `main.version`; passing it
// in (rather than reading a global) keeps this package importable from
// anywhere without circular deps.
func Notify(currentVersion string) {
	if shouldSkip(currentVersion) {
		return
	}
	// Block briefly if MaybeRefreshAsync started a fetch that hasn't
	// finished yet — necessary for short-lived commands (--help /
	// --version) that would otherwise outrun the goroutine and never
	// see a populated cache.
	waitForRefresh()
	c, err := loadCache()
	if err != nil || c == nil || c.LatestVersion == "" {
		return
	}
	if !isNewer(c.LatestVersion, currentVersion) {
		return
	}
	printWarning(os.Stderr, c.LatestVersion, currentVersion)
}

// printWarning is the formatting layer, separated for tests. Lipgloss's
// auto-detect handles non-TTY destinations (writes plain text without
// ANSI). For our use we know stderr is a TTY (shouldSkip enforced output
// mode == TTY), so colors render reliably.
func printWarning(w *os.File, latest, current string) {
	yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	headline := yellow.Render(fmt.Sprintf("⚠  one cli 有新版本可用：%s（当前 %s）", latest, current))
	cmd := dim.Render("   " + installCommand)
	fmt.Fprintln(w, headline)
	fmt.Fprintln(w, cmd)
}

// shouldSkip is the single source of truth for "should this command
// participate in the update-check pipeline?" Used by both Notify (read
// path) and MaybeRefreshAsync (network path) so the skip decision is
// honored uniformly.
//
// The order is: cheapest checks first, so the common case (CI, JSON
// output) never touches the filesystem.
func shouldSkip(currentVersion string) bool {
	if isCI() {
		return true
	}
	if !output.IsTTY() {
		return true
	}
	if currentVersion == "" || currentVersion == "0.0.0-dev" {
		return true
	}
	return false
}

// isCI reports whether common CI env vars are set. Not exhaustive; covers
// the 5 systems users of this CLI realistically run in. The generic `CI`
// var is the standard "I'm in CI" signal that GitHub Actions / GitLab CI
// / CircleCI / Buildkite / Drone all set.
func isCI() bool {
	for _, k := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "BUILDKITE"} {
		if os.Getenv(k) != "" {
			return true
		}
	}
	return false
}
