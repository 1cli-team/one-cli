package vercel

// sync.go scaffolds the project-side `vercel.json` file the first time
// a project picks deploy/vercel. Vercel's framework auto-detection
// covers most of what users need; we still write an explicit framework
// hint so first-run `vercel pull --yes` doesn't have to ask.

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// VercelConfigFilename is the canonical config file vercel CLI reads.
const VercelConfigFilename = "vercel.json"

// ShouldSync reports whether a Sync call would do work — only when no
// vercel.json is present (we never overwrite a hand-edited file).
func ShouldSync(projectDir string) bool {
	_, err := os.Stat(filepath.Join(projectDir, VercelConfigFilename))
	return errors.Is(err, fs.ErrNotExist)
}

// Sync writes a minimal vercel.json with a framework hint guessed from
// the template id. Idempotent: existing files are left alone.
func Sync(projectDir, templateID string) error {
	target := filepath.Join(projectDir, VercelConfigFilename)
	if !ShouldSync(projectDir) {
		return nil
	}
	cfg := defaultConfig(templateID)
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(target, raw, 0o644)
}

// defaultConfig maps One CLI template ids to vercel.json framework
// hints. The list is deliberately short — users on unrecognised
// templates get an empty `{}` file and rely on Vercel's framework
// auto-detection.
func defaultConfig(templateID string) map[string]any {
	switch templateID {
	case "nextjs-app":
		return map[string]any{"framework": "nextjs"}
	case "react-spa":
		return map[string]any{"framework": "vite"}
	case "astro-site", "starlight-docs":
		return map[string]any{"framework": "astro"}
	}
	return map[string]any{}
}
