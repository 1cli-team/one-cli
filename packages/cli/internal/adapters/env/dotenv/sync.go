package dotenv

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Sync ensures the secrets-relevant gitignore lines are in place.
// Patterns:
//
//	.env            base secrets file
//	.env.*          per-environment overrides (dev/staging/prod/...)
//	!.env.example   committed template still allowed through
//
// `.env.<env>` lives alongside `.env` and is treated as
// machine-local — never commit values, that's what `.env.example`
// is for. Idempotent: re-running on an already-configured repo is a
// no-op.
func Sync(workspaceRoot string) error {
	gitignorePath := filepath.Join(workspaceRoot, ".gitignore")
	existing := ""
	raw, err := os.ReadFile(gitignorePath)
	switch {
	case err == nil:
		existing = string(raw)
	case errors.Is(err, fs.ErrNotExist):
		existing = ""
	default:
		return err
	}

	wanted := []string{".env", ".env.*", "!.env.example"}
	missing := []string{}
	for _, w := range wanted {
		if !hasGitignoreLine(existing, w) {
			missing = append(missing, w)
		}
	}
	// Strip the now-redundant legacy lines that the older Sync wrote.
	// `.env.*` already covers `.env.local` and `.env.*.local`; leaving
	// the older lines around is harmless but noisy.
	cleaned := stripGitignoreLines(existing,
		".env.local", ".env.*.local")
	if len(missing) == 0 && cleaned == existing {
		return nil
	}

	out := strings.TrimRight(cleaned, "\n")
	if out != "" {
		out += "\n"
	}
	if cleaned != "" && !strings.Contains(cleaned, "# secrets") {
		out += "\n# secrets\n"
	}
	out += strings.Join(missing, "\n")
	if len(missing) > 0 {
		out += "\n"
	}
	return os.WriteFile(gitignorePath, []byte(out), 0o644)
}

// stripGitignoreLines removes any line that exactly matches one of the
// supplied patterns (after trim). Used to clear retired Sync output
// when the canonical pattern set tightens.
func stripGitignoreLines(content string, drop ...string) string {
	if content == "" {
		return content
	}
	want := make(map[string]struct{}, len(drop))
	for _, d := range drop {
		want[d] = struct{}{}
	}
	lines := strings.Split(content, "\n")
	kept := make([]string, 0, len(lines))
	for _, l := range lines {
		if _, ok := want[strings.TrimSpace(l)]; ok {
			continue
		}
		kept = append(kept, l)
	}
	return strings.Join(kept, "\n")
}

func hasGitignoreLine(content, line string) bool {
	for _, l := range strings.Split(content, "\n") {
		if strings.TrimSpace(l) == line {
			return true
		}
	}
	return false
}
