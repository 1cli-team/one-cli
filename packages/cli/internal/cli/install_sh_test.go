package cli_test

// Static checks for the curl-based installer at apps/docs/public/install.sh.
// The script is shipped to users via a
// public OSS endpoint, so a regression here means broken installs. We
// can't run the script end-to-end without hitting OSS, but we can
// guard against the cheap-but-real failure modes:
//
//  1. Bash syntax error introduced by an edit (bash -n parse-only)
//  2. A required sentinel removed accidentally (strict-mode flags,
//     SHA256 verification, OS/ARCH matrix, configurable env vars)
//
// Skipped when /bin/bash is not present (e.g. minimal Windows runners).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func installScriptPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(repoRoot(t), "..", "..", "apps", "docs", "public", "install.sh")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("install.sh missing (%v) — skip if running outside the main repo", err)
	}
	return path
}

func TestInstallSh_BashSyntaxValid(t *testing.T) {
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("bash not on PATH: %v", err)
	}
	script := installScriptPath(t)

	cmd := exec.Command(bashPath, "-n", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash -n failed (regression — install.sh has syntax error):\n  output: %s\n  err: %v",
			string(out), err)
	}
}

func TestInstallSh_RequiredSentinels(t *testing.T) {
	script := installScriptPath(t)
	body, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)

	// Categories of required substrings. Each entry is a comment +
	// one-or-more strings any of which must appear (helps when the
	// script is refactored without changing the contract).
	type sentinel struct {
		why  string
		want []string // any-of
	}
	required := []sentinel{
		{
			why:  "strict-mode opt-in protects against silent failures during install",
			want: []string{"set -euo pipefail"},
		},
		{
			why:  "SHA256 verification is the security-critical step (no signed releases yet)",
			want: []string{"sha256", "shasum", "SHA256"},
		},
		{
			why:  "macOS support — drop-out here means Apple users get a misleading error",
			want: []string{"Darwin", "darwin"},
		},
		{
			why:  "Linux support",
			want: []string{"Linux", "linux"},
		},
		{
			why:  "ARM64 support — Apple Silicon and Linux ARM",
			want: []string{"arm64", "aarch64"},
		},
		{
			why:  "AMD64 support",
			want: []string{"amd64", "x86_64"},
		},
		{
			why:  "ONE_VERSION env var lets users pin a specific version (used in CI)",
			want: []string{"ONE_VERSION"},
		},
		{
			why:  "ONE_INSTALL_DIR env var matches `task install-local` convention",
			want: []string{"ONE_INSTALL_DIR"},
		},
		{
			why:  "wrap entire flow in a function so a truncated curl can't run a half-script",
			want: []string{"main()", "main ", "function main"},
		},
		{
			why:  "ONE_FORCE env var still wired for downgrade / force-reinstall paths",
			want: []string{"ONE_FORCE"},
		},
		{
			why:  "version_compare helper drives the upgrade/skip/downgrade decision",
			want: []string{"version_compare"},
		},
		{
			why:  "current_version helper reads the installed binary's --version",
			want: []string{"current_version"},
		},
		{
			why:  "downgrade path must surface a clear, actionable error",
			want: []string{"downgrade blocked"},
		},
	}

	for _, s := range required {
		matched := false
		for _, candidate := range s.want {
			if strings.Contains(content, candidate) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("install.sh is missing %q sentinel — none of %v found", s.why, s.want)
		}
	}
}

func TestInstallSh_NoTrailingMainCallBeforeWrapping(t *testing.T) {
	// Belt-and-braces: the wrap-in-main pattern only protects against
	// truncated curl downloads if `main "$@"` is the very last
	// non-comment, non-blank line. If somebody added something after
	// it (e.g. a stray cleanup line), the protection breaks silently.
	script := installScriptPath(t)
	body, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(body), "\n")

	lastCode := ""
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		lastCode = trim
	}
	if !strings.HasPrefix(lastCode, "main ") && lastCode != "main" {
		t.Errorf("last code line should invoke main() so a truncated curl is safe; got %q", lastCode)
	}
}
