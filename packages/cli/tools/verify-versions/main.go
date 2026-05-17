// verify-versions refuses drift between the canonical VERSION file and
// every place else in the repo that hard-codes a version number.
//
// Run via Taskfile: `task verify-versions`. Exits non-zero with a
// human-readable file:line + expected/got message on any mismatch.
//
// Sources of truth:
//   - VERSION (repo root) — canonical semver, written by goreleaser at
//     release time. Treated as gospel.
//
// Things checked:
//   - packages/skills/one-cli/SKILL.md frontmatter metadata.version must
//     equal VERSION exactly. Mismatch silently changes which version of
//     the skill agents see; bundled mirror would also drift after sync.
//   - apps/docs/content/docs/{zh,en}/installation.md must not contain any
//     `vX.Y.Z` token strictly greater than VERSION.
//     Older `vX.Y.Z` examples (downgrade demos, ONE_VERSION snippets)
//     are allowed; only forward drift fails. Catches the common pattern
//     of bumping VERSION but forgetting to refresh doc samples on the
//     way back down.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "verify-versions:", err)
		os.Exit(1)
	}
}

func run() error {
	canonical, err := readVersion()
	if err != nil {
		return err
	}
	if err := checkSkillFrontmatter(canonical); err != nil {
		return err
	}
	if err := checkInstallationDoc(canonical); err != nil {
		return err
	}
	fmt.Printf("verify-versions: ok (VERSION = %s)\n", canonical)
	return nil
}

// repoRel resolves a path relative to the repo root. The Taskfile invokes
// this tool with `dir: packages/cli`, so we walk two levels up. Mirrors
// the pattern in tools/gen-error-codes/main.go.
func repoRel(parts ...string) string {
	return filepath.Join(append([]string{"..", ".."}, parts...)...)
}

var semverRE = regexp.MustCompile(`^\d+\.\d+\.\d+`)

func readVersion() (string, error) {
	p := repoRel("VERSION")
	b, err := os.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", p, err)
	}
	v := strings.TrimSpace(string(b))
	if !semverRE.MatchString(v) {
		return "", fmt.Errorf("%s contents (%q) are not a valid semver", p, v)
	}
	return v, nil
}

func checkSkillFrontmatter(canonical string) error {
	p := repoRel("packages", "skills", "one-cli", "SKILL.md")
	b, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("read %s: %w", p, err)
	}
	fm, err := extractFrontmatter(b)
	if err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	var parsed struct {
		Metadata struct {
			Version string `yaml:"version"`
		} `yaml:"metadata"`
	}
	if err := yaml.Unmarshal(fm, &parsed); err != nil {
		return fmt.Errorf("%s frontmatter parse: %w", p, err)
	}
	if parsed.Metadata.Version != canonical {
		return fmt.Errorf(
			"%s: metadata.version=%q, want %q (matches VERSION).\n  Fix: bump frontmatter and run 'task sync-bundled'.",
			p, parsed.Metadata.Version, canonical,
		)
	}
	return nil
}

// extractFrontmatter returns the bytes between the first leading `---`
// fence and the next `---` line. Tolerates both LF and CRLF line endings.
func extractFrontmatter(content []byte) ([]byte, error) {
	s := strings.ReplaceAll(string(content), "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return nil, fmt.Errorf("missing leading --- frontmatter delimiter")
	}
	rest := s[4:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, fmt.Errorf("missing closing --- frontmatter delimiter")
	}
	return []byte(rest[:end]), nil
}

// versionTokenRE matches v-prefixed semver tokens like `v1.0.0`.
// Bare semver (e.g., `1.0.0` in a comment) is intentionally NOT matched
// to avoid false positives on Node / Go / library version mentions.
var versionTokenRE = regexp.MustCompile(`\bv(\d+)\.(\d+)\.(\d+)\b`)

func checkInstallationDoc(canonical string) error {
	cMaj, cMin, cPat, err := parseSemver(canonical)
	if err != nil {
		return err
	}
	var problems []string
	for _, p := range []string{
		repoRel("apps", "docs", "content", "docs", "zh", "installation.md"),
		repoRel("apps", "docs", "content", "docs", "en", "installation.md"),
	} {
		b, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			for _, m := range versionTokenRE.FindAllStringSubmatch(line, -1) {
				maj, _ := strconv.Atoi(m[1])
				min, _ := strconv.Atoi(m[2])
				pat, _ := strconv.Atoi(m[3])
				if greaterSemver(maj, min, pat, cMaj, cMin, cPat) {
					problems = append(problems, fmt.Sprintf(
						"%s:%d: %q is newer than VERSION %s",
						p, i+1, m[0], canonical,
					))
				}
			}
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf(
			"installation doc has version tokens newer than VERSION:\n  %s\n  Fix: bump VERSION first, or update the doc samples to match.",
			strings.Join(problems, "\n  "),
		)
	}
	return nil
}

func parseSemver(v string) (int, int, int, error) {
	m := regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`).FindStringSubmatch(v)
	if m == nil {
		return 0, 0, 0, fmt.Errorf("not a semver: %q", v)
	}
	maj, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	pat, _ := strconv.Atoi(m[3])
	return maj, min, pat, nil
}

func greaterSemver(maj1, min1, pat1, maj2, min2, pat2 int) bool {
	if maj1 != maj2 {
		return maj1 > maj2
	}
	if min1 != min2 {
		return min1 > min2
	}
	return pat1 > pat2
}
